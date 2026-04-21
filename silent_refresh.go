package olympus

import (
	"context"
	"sync"
	"time"
)

// ----------------------------------------------------------------------------
// Silent token refresh + session event stream
// (olympus-cloud-gcp#3403 §1.4 / olympus-cloud-gcp#3412). 5-language parity
// with dart / typescript / python / rust SDKs.
//
// Design:
//
//   - Goroutine using time.NewTimer (exp-based, not periodic).
//   - Timer fires at `exp - refreshMargin` — on fire, call /auth/refresh and
//     emit SessionRefreshed, or emit SessionExpired on failure.
//   - Exponential-decay event fan-out — subscribers get a buffered channel
//     (capacity 8). If a subscriber's buffer is full, the emitter drops the
//     event for that subscriber rather than blocking.
//   - All mutable state (active session, subscribers, goroutine cancel
//     handle) is protected by a single sync.Mutex on AuthService.
//   - Idempotent StartSilentRefresh: calling it while a goroutine is active
//     cancels the prior goroutine, waits for it to exit, and starts a fresh
//     one. No goroutine leak.
//
// ----------------------------------------------------------------------------

// DefaultRefreshMargin is the default window before token expiry in which the
// silent-refresh goroutine will proactively refresh. Matches the 60s value
// used by the dart / typescript / python / rust SDK implementations.
const DefaultRefreshMargin = 60 * time.Second

// refreshEventBufferSize is the per-subscriber channel capacity. Deliberately
// small — subscribers are expected to consume promptly. A full buffer drops
// the new event rather than blocking the emitter (see broadcastSessionEvent).
const refreshEventBufferSize = 8

// SessionEvent is a discriminated union over session transitions observed by
// SessionEvents subscribers. Use a type switch:
//
//	for evt := range ch {
//	    switch e := evt.(type) {
//	    case *SessionLoggedIn:   // ...
//	    case *SessionRefreshed:  // ...
//	    case *SessionExpired:    // e.Reason
//	    case *SessionLoggedOut:  // ...
//	    }
//	}
type SessionEvent interface {
	isSessionEvent()
}

// SessionLoggedIn fires when a new session is acquired (Login / LoginSSO /
// LoginPin / LoginMFA succeeds).
type SessionLoggedIn struct{ Session *AuthSession }

func (*SessionLoggedIn) isSessionEvent() {}

// SessionRefreshed fires after a successful silent (or explicit) refresh.
type SessionRefreshed struct{ Session *AuthSession }

func (*SessionRefreshed) isSessionEvent() {}

// SessionExpired fires when the silent-refresh goroutine's attempt to refresh
// fails terminally. After this event the AuthService access token is
// cleared and the goroutine exits.
type SessionExpired struct{ Reason string }

func (*SessionExpired) isSessionEvent() {}

// SessionLoggedOut fires when Logout succeeds (before the token is cleared
// from HTTP, but after the refresh goroutine is stopped).
type SessionLoggedOut struct{}

func (*SessionLoggedOut) isSessionEvent() {}

// sessionState is the mutable bookkeeping for silent refresh + event fan-out.
// Held via a pointer on AuthService (sessionState *sessionState) so zero-value
// AuthService (created ad-hoc by OlympusClient.Auth()) is safe — state is
// lazy-initialised on first use.
type sessionState struct {
	mu sync.Mutex

	// current session — used by the refresh goroutine to read the
	// refresh_token + exp at each tick.
	session *AuthSession

	// subscribers registered via SessionEvents.
	subscribers []chan SessionEvent

	// goroutine lifecycle: stopCh signals the goroutine to exit, doneCh is
	// closed by the goroutine on exit. kickCh is a buffered (cap 1) nudge to
	// restart the timer from the current session.
	stopCh chan struct{}
	doneCh chan struct{}
	kickCh chan struct{}
}

// ensureState returns the lazily-created sessionState for the AuthService.
// We use a pointer field + a sync.Once style guard so concurrent first-use
// from multiple goroutines cannot race.
func (s *AuthService) ensureState() *sessionState {
	s.stateOnce.Do(func() {
		s.state = &sessionState{}
	})
	return s.state
}

// ----------------------------------------------------------------------------
// Public API
// ----------------------------------------------------------------------------

// SessionEvents returns a buffered channel (capacity 8) of session events
// plus a cancel func that unregisters the subscriber and closes the channel.
//
// Callers should drain the channel regularly. If the buffer fills, newer
// events for that subscriber are dropped (the emitter never blocks).
//
// Safe to call from any goroutine.
func (s *AuthService) SessionEvents() (<-chan SessionEvent, func()) {
	st := s.ensureState()
	ch := make(chan SessionEvent, refreshEventBufferSize)
	st.mu.Lock()
	st.subscribers = append(st.subscribers, ch)
	st.mu.Unlock()

	var cancelOnce sync.Once
	cancel := func() {
		cancelOnce.Do(func() {
			st.mu.Lock()
			for i, c := range st.subscribers {
				if c == ch {
					// Remove from slice preserving order.
					st.subscribers = append(st.subscribers[:i], st.subscribers[i+1:]...)
					break
				}
			}
			st.mu.Unlock()
			close(ch)
		})
	}
	return ch, cancel
}

// StartSilentRefresh spawns a goroutine that refreshes the access token
// `refreshMargin` before its `exp` claim. If refreshMargin is <= 0,
// DefaultRefreshMargin (60s) is used.
//
// Idempotent: calling twice cancels the prior goroutine (it returns cleanly
// before the new one starts — no leak).
//
// Returns a cancel func that, when called, stops the goroutine. Equivalent
// to StopSilentRefresh.
//
// Requires a session with a refresh_token — typically called immediately
// after Login / LoginSSO / LoginPin / LoginMFA. If there is no current
// session the goroutine still starts but idles until a session is acquired.
func (s *AuthService) StartSilentRefresh(refreshMargin time.Duration) func() {
	if refreshMargin <= 0 {
		refreshMargin = DefaultRefreshMargin
	}
	st := s.ensureState()

	st.mu.Lock()
	// Cancel any prior goroutine in-place. We hold the mutex but release
	// it before waiting on doneCh so the goroutine can take it if needed.
	prevStop := st.stopCh
	prevDone := st.doneCh
	st.stopCh = make(chan struct{})
	st.doneCh = make(chan struct{})
	st.kickCh = make(chan struct{}, 1)
	newStop := st.stopCh
	newDone := st.doneCh
	newKick := st.kickCh
	st.mu.Unlock()

	if prevStop != nil {
		close(prevStop)
		<-prevDone
	}

	go s.refreshLoop(newStop, newDone, newKick, refreshMargin)

	return s.StopSilentRefresh
}

// StopSilentRefresh cancels the silent-refresh goroutine if one is running.
// Idempotent — safe to call multiple times and safe to call before Start.
func (s *AuthService) StopSilentRefresh() {
	st := s.ensureState()
	st.mu.Lock()
	stop := st.stopCh
	done := st.doneCh
	st.stopCh = nil
	st.doneCh = nil
	st.kickCh = nil
	st.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if done != nil {
		<-done
	}
}

// ----------------------------------------------------------------------------
// Internal — called from Login / Logout / Refresh etc.
// ----------------------------------------------------------------------------

// onSessionAcquired stores the session, broadcasts the appropriate event,
// and nudges the refresh goroutine (if one is running) to reschedule from
// the new exp. isRefresh distinguishes SessionLoggedIn (false) from
// SessionRefreshed (true).
func (s *AuthService) onSessionAcquired(session *AuthSession, isRefresh bool) {
	if session == nil {
		return
	}
	st := s.ensureState()
	st.mu.Lock()
	st.session = session
	kick := st.kickCh
	st.mu.Unlock()

	var evt SessionEvent
	if isRefresh {
		evt = &SessionRefreshed{Session: session}
	} else {
		evt = &SessionLoggedIn{Session: session}
	}
	s.broadcastSessionEvent(evt)

	// Nudge the goroutine (non-blocking — kickCh is cap 1).
	if kick != nil {
		select {
		case kick <- struct{}{}:
		default:
		}
	}
}

// onSessionLoggedOut clears stored session, stops the goroutine (if any),
// and broadcasts SessionLoggedOut.
func (s *AuthService) onSessionLoggedOut() {
	st := s.ensureState()

	// Capture + clear goroutine handles under the lock, then signal + wait
	// outside the lock so the goroutine's loop can proceed.
	st.mu.Lock()
	stop := st.stopCh
	done := st.doneCh
	st.stopCh = nil
	st.doneCh = nil
	st.kickCh = nil
	st.session = nil
	st.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if done != nil {
		<-done
	}

	s.broadcastSessionEvent(&SessionLoggedOut{})
}

// broadcastSessionEvent delivers evt to every current subscriber. Non-blocking
// per-subscriber — a full buffer drops the event for that subscriber (the
// emitter never blocks).
func (s *AuthService) broadcastSessionEvent(evt SessionEvent) {
	st := s.ensureState()
	st.mu.Lock()
	// Copy the slice under the lock so we release it before sending (which
	// is non-blocking but we want to minimise lock hold time).
	subs := make([]chan SessionEvent, len(st.subscribers))
	copy(subs, st.subscribers)
	st.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
			// Buffer full — drop rather than block the emitter.
		}
	}
}

// refreshLoop is the goroutine body. Reads the stored session's exp, sleeps
// until `exp - margin`, calls /auth/refresh, and either reschedules on
// success or emits SessionExpired + exits on failure.
//
// The goroutine OWNS `done` — it's the only code that closes it. Callers
// (StopSilentRefresh / StartSilentRefresh / onSessionLoggedOut) close `stop`
// to signal exit, then block on `<-done` for confirmation.
func (s *AuthService) refreshLoop(stop <-chan struct{}, done chan<- struct{}, kick <-chan struct{}, margin time.Duration) {
	defer close(done)

	for {
		// Compute next fire time from the stored session's access-token exp.
		fireIn, ok := s.nextRefreshDelay(margin)
		if !ok {
			// No session / no exp yet — wait for a kick or stop. Don't emit
			// SessionExpired here: "no session" is a legitimate pre-login
			// state, not an expiry.
			select {
			case <-stop:
				return
			case <-kick:
				continue
			}
		}

		if fireIn <= 0 {
			// Already past exp - margin — refresh immediately.
			fireIn = 0
		}

		timer := time.NewTimer(fireIn)
		select {
		case <-stop:
			timer.Stop()
			return
		case <-kick:
			// Session changed (Login / Refresh) — recompute exp on next loop.
			timer.Stop()
			continue
		case <-timer.C:
			// Fire the refresh.
			if !s.performRefresh() {
				// performRefresh already emitted SessionExpired and cleared
				// the session. Exit.
				return
			}
			// performRefresh emitted SessionRefreshed; loop to reschedule.
		}
	}
}

// nextRefreshDelay returns the duration until the next refresh fire relative
// to now, based on the stored session's access-token exp claim. Returns
// (0, false) if there is no session or the exp claim is missing.
func (s *AuthService) nextRefreshDelay(margin time.Duration) (time.Duration, bool) {
	st := s.ensureState()
	st.mu.Lock()
	session := st.session
	st.mu.Unlock()
	if session == nil || session.AccessToken == "" {
		return 0, false
	}
	claims := parseJWTPayload(session.AccessToken)
	if claims == nil {
		return 0, false
	}
	expRaw, ok := claims["exp"]
	if !ok {
		return 0, false
	}
	var expUnix int64
	switch v := expRaw.(type) {
	case float64:
		expUnix = int64(v)
	case int:
		expUnix = int64(v)
	case int64:
		expUnix = v
	default:
		return 0, false
	}
	fireAt := time.Unix(expUnix, 0).Add(-margin)
	return time.Until(fireAt), true
}

// performRefresh calls /auth/refresh using the stored session's
// refresh_token and emits the appropriate event. Returns true on success
// (SessionRefreshed emitted, state updated) and false on failure
// (SessionExpired emitted, state cleared).
func (s *AuthService) performRefresh() bool {
	st := s.ensureState()
	st.mu.Lock()
	session := st.session
	st.mu.Unlock()

	if session == nil || session.RefreshToken == "" {
		s.clearSessionAndEmitExpired("no refresh token")
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := s.http.post(ctx, "/auth/refresh", map[string]interface{}{
		"refresh_token": session.RefreshToken,
	})
	if err != nil {
		s.clearSessionAndEmitExpired(err.Error())
		return false
	}

	newSession := parseAuthSession(resp)
	if newSession.AccessToken == "" {
		s.clearSessionAndEmitExpired("refresh returned empty access_token")
		return false
	}

	s.http.SetAccessToken(newSession.AccessToken)

	st.mu.Lock()
	st.session = newSession
	st.mu.Unlock()

	s.broadcastSessionEvent(&SessionRefreshed{Session: newSession})
	return true
}

// clearSessionAndEmitExpired wipes the stored session + HTTP access token
// and broadcasts SessionExpired.
func (s *AuthService) clearSessionAndEmitExpired(reason string) {
	st := s.ensureState()
	st.mu.Lock()
	st.session = nil
	st.mu.Unlock()
	s.http.ClearAccessToken()
	s.broadcastSessionEvent(&SessionExpired{Reason: reason})
}
