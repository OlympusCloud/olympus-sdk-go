package olympus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// makeJWTWithExp builds a test JWT whose payload carries only the `exp` claim
// at `expAt` seconds since epoch (integer, matching real tokens).
func makeJWTWithExp(expAt int64) string {
	return makeJWT(map[string]interface{}{
		"sub": "u",
		"exp": float64(expAt),
	})
}

// newRefreshServer returns a test server that handles POST /auth/refresh.
// The handler increments hitCount atomically and returns the next token with
// an exp `newExpIn` from now, or a 401 when the `fail` flag is set.
type refreshServer struct {
	srv      *httptest.Server
	hitCount int32
	mu       sync.Mutex
	fail     bool
	newExpIn time.Duration
}

func newRefreshServer(t *testing.T, newExpIn time.Duration) *refreshServer {
	t.Helper()
	rs := &refreshServer{newExpIn: newExpIn}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&rs.hitCount, 1)
		rs.mu.Lock()
		fail := rs.fail
		expIn := rs.newExpIn
		rs.mu.Unlock()

		if fail {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "UNAUTHORIZED",
					"message": "refresh rejected",
				},
			})
			return
		}

		// Issue a new token.
		exp := time.Now().Add(expIn).Unix()
		newAccess := makeJWTWithExp(exp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  newAccess,
			"token_type":    "Bearer",
			"expires_in":    int(expIn.Seconds()),
			"refresh_token": "refresh-next",
		})
	})
	rs.srv = httptest.NewServer(mux)
	t.Cleanup(rs.srv.Close)
	return rs
}

func (rs *refreshServer) setFail(b bool) {
	rs.mu.Lock()
	rs.fail = b
	rs.mu.Unlock()
}

func (rs *refreshServer) hits() int32 { return atomic.LoadInt32(&rs.hitCount) }

// newClientTo creates a client pointed at the given server URL.
func newClientTo(url string) *OlympusClient {
	return NewClient(Config{
		AppID:      "test",
		APIKey:     "k",
		BaseURL:    url,
		MaxRetries: 0,
	})
}

// seedSession primes AuthService with a session so StartSilentRefresh has
// something to refresh.
func seedSession(auth *AuthService, accessToken, refreshToken string) {
	// Emits SessionLoggedIn; nudges kick if goroutine is running.
	auth.http.SetAccessToken(accessToken)
	auth.onSessionAcquired(&AuthSession{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, false)
}

// waitForEvent reads from ch until an event matches predicate OR timeout.
// Returns the matched event or nil on timeout.
func waitForEvent(t *testing.T, ch <-chan SessionEvent, timeout time.Duration, predicate func(SessionEvent) bool) SessionEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return nil
			}
			if predicate(evt) {
				return evt
			}
		case <-deadline:
			return nil
		}
	}
}

// ----------------------------------------------------------------------------
// SessionEvents subscribe / unsubscribe
// ----------------------------------------------------------------------------

func TestSessionEvents_SubscribeAndEmitLoggedIn(t *testing.T) {
	rs := newRefreshServer(t, 10*time.Second)
	c := newClientTo(rs.srv.URL)
	ch, cancel := c.Auth().SessionEvents()
	defer cancel()

	seedSession(c.Auth(), makeJWTWithExp(time.Now().Add(1*time.Hour).Unix()), "rt")

	evt := waitForEvent(t, ch, 100*time.Millisecond, func(e SessionEvent) bool {
		_, ok := e.(*SessionLoggedIn)
		return ok
	})
	if evt == nil {
		t.Fatal("expected SessionLoggedIn event")
	}
	logged, ok := evt.(*SessionLoggedIn)
	if !ok || logged.Session == nil {
		t.Fatalf("expected SessionLoggedIn with session, got %T", evt)
	}
	if logged.Session.RefreshToken != "rt" {
		t.Errorf("expected refresh token 'rt', got %q", logged.Session.RefreshToken)
	}
}

func TestSessionEvents_CancelClosesChannel(t *testing.T) {
	rs := newRefreshServer(t, 10*time.Second)
	c := newClientTo(rs.srv.URL)
	ch, cancel := c.Auth().SessionEvents()
	cancel()

	// After cancel the channel must be closed — draining returns !ok.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel closed after cancel")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("cancel did not close channel")
	}

	// Double-cancel must not panic.
	cancel()
}

func TestSessionEvents_DropsOnFullBuffer(t *testing.T) {
	rs := newRefreshServer(t, 10*time.Second)
	c := newClientTo(rs.srv.URL)
	ch, cancel := c.Auth().SessionEvents()
	defer cancel()

	// Emit more events than the buffer (cap 8) can hold. We DO NOT drain ch.
	// Each emit should return immediately; no goroutine blocks.
	emitDone := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			c.Auth().broadcastSessionEvent(&SessionLoggedOut{})
		}
		close(emitDone)
	}()
	select {
	case <-emitDone:
		// Good — emitter never blocked.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("broadcastSessionEvent blocked when subscriber buffer was full")
	}

	// Drain what we can — should be at most refreshEventBufferSize events.
	count := 0
	draining := true
	for draining {
		select {
		case <-ch:
			count++
		default:
			draining = false
		}
	}
	if count > refreshEventBufferSize {
		t.Errorf("drained %d events, buffer cap is %d", count, refreshEventBufferSize)
	}
	if count == 0 {
		t.Error("expected at least one buffered event")
	}
}

// ----------------------------------------------------------------------------
// StartSilentRefresh — timer + reschedule + expiry
// ----------------------------------------------------------------------------

func TestSilentRefresh_FiresBeforeExp(t *testing.T) {
	// Token expires in 200ms; margin 120ms → refresh fires ~80ms.
	rs := newRefreshServer(t, 10*time.Second)
	c := newClientTo(rs.srv.URL)

	ch, cancelSub := c.Auth().SessionEvents()
	defer cancelSub()

	exp := time.Now().Add(200 * time.Millisecond).Unix()
	seedSession(c.Auth(), makeJWTWithExp(exp), "rt-1")

	stop := c.Auth().StartSilentRefresh(120 * time.Millisecond)
	defer stop()

	// Expect SessionRefreshed within ~300ms.
	evt := waitForEvent(t, ch, 500*time.Millisecond, func(e SessionEvent) bool {
		_, ok := e.(*SessionRefreshed)
		return ok
	})
	if evt == nil {
		t.Fatalf("expected SessionRefreshed, got %d hits", rs.hits())
	}
	if rs.hits() < 1 {
		t.Errorf("expected at least 1 refresh hit, got %d", rs.hits())
	}
}

func TestSilentRefresh_Reschedules(t *testing.T) {
	// Each refresh returns a token expiring in 250ms; margin 120ms → fire at
	// ~130ms. Expect 2+ fires within 600ms.
	rs := newRefreshServer(t, 250*time.Millisecond)
	c := newClientTo(rs.srv.URL)

	ch, cancelSub := c.Auth().SessionEvents()
	defer cancelSub()

	exp := time.Now().Add(250 * time.Millisecond).Unix()
	seedSession(c.Auth(), makeJWTWithExp(exp), "rt-1")

	stop := c.Auth().StartSilentRefresh(120 * time.Millisecond)
	defer stop()

	refreshCount := 0
	deadline := time.After(800 * time.Millisecond)
drain:
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				break drain
			}
			if _, isR := evt.(*SessionRefreshed); isR {
				refreshCount++
				if refreshCount >= 2 {
					break drain
				}
			}
		case <-deadline:
			break drain
		}
	}
	if refreshCount < 2 {
		t.Errorf("expected >=2 refreshes, got %d (server hits=%d)", refreshCount, rs.hits())
	}
}

func TestSilentRefresh_FailureEmitsExpiredAndStops(t *testing.T) {
	rs := newRefreshServer(t, 10*time.Second)
	rs.setFail(true)
	c := newClientTo(rs.srv.URL)

	ch, cancelSub := c.Auth().SessionEvents()
	defer cancelSub()

	exp := time.Now().Add(200 * time.Millisecond).Unix()
	seedSession(c.Auth(), makeJWTWithExp(exp), "rt-1")

	_ = c.Auth().StartSilentRefresh(120 * time.Millisecond)

	evt := waitForEvent(t, ch, 500*time.Millisecond, func(e SessionEvent) bool {
		_, ok := e.(*SessionExpired)
		return ok
	})
	if evt == nil {
		t.Fatal("expected SessionExpired after refresh failure")
	}
	exp2, _ := evt.(*SessionExpired)
	if exp2.Reason == "" {
		t.Error("expected non-empty Reason on SessionExpired")
	}
	// Access token must be cleared on expiry.
	if c.Auth().http.GetAccessToken() != "" {
		t.Error("access token should be cleared after SessionExpired")
	}
}

func TestSilentRefresh_LogoutCancelsAndEmits(t *testing.T) {
	// Stand up a dedicated server with both /auth/refresh and /auth/logout
	// so we don't rewrite handlers after serving has started.
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/refresh", func(w http.ResponseWriter, _ *http.Request) {
		exp := time.Now().Add(10 * time.Second).Unix()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  makeJWTWithExp(exp),
			"token_type":    "Bearer",
			"refresh_token": "next",
		})
	})
	mux.HandleFunc("/auth/logout", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newClientTo(srv.URL)
	ch, cancelSub := c.Auth().SessionEvents()
	defer cancelSub()

	exp := time.Now().Add(10 * time.Second).Unix()
	seedSession(c.Auth(), makeJWTWithExp(exp), "rt-1")

	_ = c.Auth().StartSilentRefresh(120 * time.Millisecond)

	if err := c.Auth().Logout(context.Background()); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	evt := waitForEvent(t, ch, 500*time.Millisecond, func(e SessionEvent) bool {
		_, ok := e.(*SessionLoggedOut)
		return ok
	})
	if evt == nil {
		t.Fatal("expected SessionLoggedOut event")
	}

	// Goroutine should be stopped — stopCh is nil after logout.
	st := c.Auth().ensureState()
	st.mu.Lock()
	active := st.stopCh != nil
	st.mu.Unlock()
	if active {
		t.Error("expected goroutine stopped after Logout")
	}
}

func TestSilentRefresh_DoubleStartCancelsFirst(t *testing.T) {
	rs := newRefreshServer(t, 10*time.Second)
	c := newClientTo(rs.srv.URL)

	exp := time.Now().Add(10 * time.Second).Unix()
	seedSession(c.Auth(), makeJWTWithExp(exp), "rt-1")

	stop1 := c.Auth().StartSilentRefresh(1 * time.Hour) // long-idle
	// Capture the first doneCh so we can assert it closed after the second
	// Start (rotation semantics).
	st := c.Auth().ensureState()
	st.mu.Lock()
	firstDone := st.doneCh
	st.mu.Unlock()
	if firstDone == nil {
		t.Fatal("doneCh should be set after first Start")
	}

	_ = c.Auth().StartSilentRefresh(1 * time.Hour)

	// The first goroutine must have exited — firstDone must be closed now.
	select {
	case <-firstDone:
		// Good.
	case <-time.After(200 * time.Millisecond):
		t.Fatal("first refresh goroutine did not exit after second Start")
	}

	// Clean up the second goroutine.
	stop1() // equivalent to StopSilentRefresh — safe to call twice.
	c.Auth().StopSilentRefresh()
}

func TestSilentRefresh_StopIsIdempotent(t *testing.T) {
	rs := newRefreshServer(t, 10*time.Second)
	c := newClientTo(rs.srv.URL)

	// Stop before start — must be a no-op.
	c.Auth().StopSilentRefresh()

	exp := time.Now().Add(10 * time.Second).Unix()
	seedSession(c.Auth(), makeJWTWithExp(exp), "rt-1")

	stop := c.Auth().StartSilentRefresh(1 * time.Hour)
	stop()
	stop()                            // second cancel — safe.
	c.Auth().StopSilentRefresh()      // redundant — safe.
}

func TestSilentRefresh_NoSessionIdles(t *testing.T) {
	// Goroutine with no session should idle (not emit SessionExpired).
	rs := newRefreshServer(t, 10*time.Second)
	c := newClientTo(rs.srv.URL)

	ch, cancelSub := c.Auth().SessionEvents()
	defer cancelSub()

	stop := c.Auth().StartSilentRefresh(100 * time.Millisecond)
	defer stop()

	// Wait briefly — no event should fire.
	select {
	case evt := <-ch:
		t.Fatalf("unexpected event from idle goroutine: %T", evt)
	case <-time.After(150 * time.Millisecond):
		// Good — idled.
	}

	// Hitting the refresh endpoint should have been 0 times.
	if rs.hits() != 0 {
		t.Errorf("expected 0 refresh hits while idling, got %d", rs.hits())
	}
}

// ----------------------------------------------------------------------------
// Concurrency — concurrent Start/Stop, race safety
// ----------------------------------------------------------------------------

func TestSilentRefresh_ConcurrentStartStop(t *testing.T) {
	rs := newRefreshServer(t, 10*time.Second)
	c := newClientTo(rs.srv.URL)

	exp := time.Now().Add(10 * time.Second).Unix()
	seedSession(c.Auth(), makeJWTWithExp(exp), "rt-1")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cancel := c.Auth().StartSilentRefresh(1 * time.Hour)
			time.Sleep(5 * time.Millisecond)
			cancel()
		}()
	}
	wg.Wait()

	// Final cleanup.
	c.Auth().StopSilentRefresh()

	// Goroutine state must be clean.
	st := c.Auth().ensureState()
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.stopCh != nil || st.doneCh != nil {
		t.Error("expected clean state after concurrent Start/Stop")
	}
}

func TestSilentRefresh_ConcurrentSubscribers(t *testing.T) {
	rs := newRefreshServer(t, 10*time.Second)
	c := newClientTo(rs.srv.URL)

	const subs = 20
	chans := make([]<-chan SessionEvent, subs)
	cancels := make([]func(), subs)
	for i := 0; i < subs; i++ {
		chans[i], cancels[i] = c.Auth().SessionEvents()
	}

	// Emit a login event.
	exp := time.Now().Add(1 * time.Hour).Unix()
	seedSession(c.Auth(), makeJWTWithExp(exp), "rt-1")

	// All subscribers should see SessionLoggedIn.
	for i := 0; i < subs; i++ {
		evt := waitForEvent(t, chans[i], 200*time.Millisecond, func(e SessionEvent) bool {
			_, ok := e.(*SessionLoggedIn)
			return ok
		})
		if evt == nil {
			t.Errorf("subscriber %d did not receive SessionLoggedIn", i)
		}
	}

	// Cancel all.
	for i := 0; i < subs; i++ {
		cancels[i]()
	}

	// Subscriber list should be empty.
	st := c.Auth().ensureState()
	st.mu.Lock()
	n := len(st.subscribers)
	st.mu.Unlock()
	if n != 0 {
		t.Errorf("expected 0 subscribers after all cancels, got %d", n)
	}
}

// ----------------------------------------------------------------------------
// Login / Logout event emission (AuthService integration)
// ----------------------------------------------------------------------------

func TestLogin_EmitsSessionLoggedIn(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		exp := time.Now().Add(1 * time.Hour).Unix()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  makeJWTWithExp(exp),
			"token_type":    "Bearer",
			"refresh_token": "rt-login",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newClientTo(srv.URL)
	ch, cancel := c.Auth().SessionEvents()
	defer cancel()

	_, err := c.Auth().Login(context.Background(), "u@x", "p")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	evt := waitForEvent(t, ch, 200*time.Millisecond, func(e SessionEvent) bool {
		_, ok := e.(*SessionLoggedIn)
		return ok
	})
	if evt == nil {
		t.Fatal("expected SessionLoggedIn from Login")
	}
}

func TestRefresh_EmitsSessionRefreshed(t *testing.T) {
	rs := newRefreshServer(t, 1*time.Hour)
	c := newClientTo(rs.srv.URL)

	ch, cancel := c.Auth().SessionEvents()
	defer cancel()

	_, err := c.Auth().Refresh(context.Background(), "rt")
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	evt := waitForEvent(t, ch, 200*time.Millisecond, func(e SessionEvent) bool {
		_, ok := e.(*SessionRefreshed)
		return ok
	})
	if evt == nil {
		t.Fatal("expected SessionRefreshed from explicit Refresh")
	}
}
