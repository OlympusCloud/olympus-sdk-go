# Changelog

## Unreleased

### Silent token refresh + session events (OlympusCloud/olympus-cloud-gcp#3403 §1.4 / olympus-cloud-gcp#3412)

In-SDK 401-at-TTL auto-resolution. A goroutine decodes the access-token
`exp` claim, sleeps until `exp - refreshMargin`, and calls `/auth/refresh`
transparently. Session transitions are broadcast to subscribers via a
buffered event channel.

**New APIs on `AuthService`:**

- `StartSilentRefresh(refreshMargin time.Duration) func()` — spawns the
  refresh goroutine. Returns a cancel func equivalent to `StopSilentRefresh`.
  Idempotent — calling twice cancels the prior goroutine cleanly (no leak).
  Pass `<=0` to use `DefaultRefreshMargin` (60s), matching the dart /
  typescript / python / rust SDKs.
- `StopSilentRefresh()` — stops the goroutine. Safe to call before Start.
- `SessionEvents() (<-chan SessionEvent, func())` — buffered channel (cap 8)
  of session transitions. Fan-out is non-blocking — a full buffer drops
  rather than blocks the emitter. The returned cancel unregisters and
  closes the channel.

**New types:** `SessionEvent` discriminated union with variants
`*SessionLoggedIn`, `*SessionRefreshed`, `*SessionExpired{Reason}`,
`*SessionLoggedOut`. `DefaultRefreshMargin = 60 * time.Second`.

**Behaviour wired through existing methods:**

- `Login` / `LoginSSO` / `LoginPin` / `LoginMFA` — emit `SessionLoggedIn`
  and nudge the refresh goroutine to reschedule from the new exp.
- `Refresh` — emits `SessionRefreshed`, reschedules.
- `Logout` — cancels the goroutine, clears HTTP token, emits
  `SessionLoggedOut`.
- On refresh failure (non-2xx or transport error) — goroutine emits
  `SessionExpired{Reason}`, clears the HTTP token, and exits.

Race-safe: mutex-protected goroutine handles + stored session; non-blocking
fan-out; full `-race` test coverage including concurrent Start/Stop and
concurrent subscribers.

### Scope helper + generated constants (OlympusCloud/olympus-cloud-gcp#3403 §1.2)

Wires client-side app-scope introspection off the JWT `app_scopes` claim and
ships typed constants for every scope + role in the platform catalog.

**New APIs on `AuthService`:**

- `HasScope(scope string) bool` — true iff the current session carries the
  given canonical scope string (e.g. `auth.session.read@user`).
- `RequireScope(scope string) error` — returns `*ScopeRequiredError` when the
  scope is not present. Useful as a client-side pre-flight before routing the
  user through the consent flow.
- `GrantedScopes() []string` — defensive copy of the scope list decoded from
  the access token's `app_scopes` claim.

**New types:** `ScopeRequiredError` in `errors.go`; `AppScopes []string` field
on `AuthSession`.

**Generated constants** (`constants_scopes.go`, `constants_roles.go`): every
scope + role from `docs/platform/scopes.yaml` + `docs/platform/roles.yaml` as
a typed Go constant, e.g. `olympus.ScopeAuthSessionReadAtUser`,
`olympus.RoleTenantAdmin`. Regenerate via
`scripts/generate_sdk_scope_constants.py` in the monorepo. 5-language parity
with dart / typescript / python / rust SDKs.

## 0.5.0 (2026-04-19)

### Wave 2 of the SDK 1.0 Campaign (OlympusCloud/olympus-cloud-gcp#3216)

Mirrors the canonical dart SDK surface for voice + identity + smart-home +
SMS + voice-orders. Wire-mirrors `voice_service.dart`,
`identity_service.dart`, `smart_home_service.dart`, `sms_service.dart`, and
`voice_order_service.dart`.

**New services:**

- `client.Identity()` — Olympus ID (global cross-tenant identity) + Document
  AI age-verification. Wraps `/platform/identities*` and `/identity/*`.
- `client.SmartHome()` — smart-home platforms, devices, rooms, scenes,
  automations. Wraps `/smart-home/*`.
- `client.SMS()` — voice-platform SMS (`/voice/sms/*`) and unified CPaaS
  messaging (`/cpaas/messages/*`).

**New methods on existing services:**

- `client.Voice()` — full dart parity. Adds agent CRUD (Create/Get/Update/
  Delete/Clone), persona library, ambiance/voice tuning, workflow
  templates, voicemails, conversations, analytics, campaigns, phone
  numbers, marketplace voices/packs, calls, speaker enrollment, voice
  profiles, edge pipeline (ProcessAudio + GetVoiceWebSocketURL +
  PipelineHealth), caller profiles, escalation/business hours, agent
  testing. ~50 new methods.

**New types:** `OlympusIdentity`, `IdentityLink`,
`GetOrCreateFromFirebaseRequest`, `ListDevicesOptions`, `SendRequest`,
`GetConversationsOptions`, `SendViaCpaasRequest`, `ListConfigsOptions`,
`CreateAgentRequest`, `UpdateAgentRequest`, `CloneAgentRequest`,
`PreviewAgentVoiceRequest`, `ProvisionAgentRequest`, `ListPersonasOptions`,
`InstantiateAgentTemplateRequest`, `PublishAgentAsTemplateRequest`,
`UploadAmbianceBedRequest`, `UpdateAgentAmbianceRequest`,
`UpdateAgentVoiceOverridesRequest`, `ListWorkflowTemplatesOptions`,
`ListVoicemailsOptions`, `ListConversationsOptions`,
`ListVoiceMessagesOptions`, `GetAnalyticsOptions`, `ListCampaignsOptions`,
`ListNumbersOptions`, `SearchNumbersOptions`, `ListVoicesOptions`,
`ListPacksOptions`, `ListProfilesOptions`, `ProcessAudioRequest`,
`ListCallerProfilesOptions`, `TestAgentRequest`.

**Client surface completion (PR #2 follow-up):**

- `*OlympusClient` accessors `SetAccessToken`, `ClearAccessToken`,
  `SetAppToken`, `ClearAppToken`, `OnCatalogStale`, `IsAppScoped`,
  `HasScopeBit` — these were referenced by the v0.4.x test suite added in
  PR #2 but never wired through the public client. Wave 2 adds them so
  `go test ./...` is green from a fresh checkout.

**Versioning:** `Version = "0.5.0"` exposed via the new `version.go`. The
`X-SDK-Version` header now derives from this constant (was hardcoded to
`go/0.1.0`).

**Tests:** new `voice_test.go`, `identity_test.go`, `smart_home_test.go`,
`sms_test.go`, `voice_orders_test.go`. Test suite now: `go test ./...`
green, race-clean.

## 0.4.0 (2026-04-18)

### Wave 1 of the SDK 1.0 Campaign (OlympusCloud/olympus-cloud-gcp#3216, Wave #3217)

**New services:**

- `client.Voice()` — Voice Agent V2 cascade resolver (#3162 V2-005) and
  adjacent voice-agent operations.
- `client.Connect()` — marketing-funnel + pre-conversion lead capture
  (#3108).

**New methods:**

- `client.Voice().GetEffectiveConfig(ctx, agentID)` → `*VoiceEffectiveConfig`.
  Backing endpoint `GET /api/v1/voice-agents/configs/{id}/effective-config`.
- `client.Voice().GetPipeline(ctx, agentID)` → `*VoicePipeline`. Canonical
  subset for runtimes / provisioners.
- `client.Connect().CreateLead(ctx, CreateLeadRequest)` → `*CreateLeadResponse`.
  Unauthenticated; idempotent on email over 1h.

**New types:** `VoiceEffectiveConfig`, `VoicePipeline`, `VoiceDefaultsCascade`,
`VoiceDefaultsRung`, `CreateLeadRequest`, `CreateLeadResponse`, `UTM`.

**Versioning (first tagged release):** this repository was previously
un-tagged on the module proxy. v0.4.0 is the first cut as part of the SDK
1.0 campaign. Subsequent waves bump to v0.5.0 → v1.0.0.

**Deferred from Wave 1:**

- `client.Auth().CreateServiceToken(...)` — endpoint #2848 exists in Rust
  auth but is not routed through the Go gateway. Tracked in platform issue
  OlympusCloud/olympus-cloud-gcp#3220. Wave 1.5.
- Identity / training coverage — Wave 2 per campaign doc §2.

**Tests:** `voice_v2_test.go` — 4/4 passing. Fixtures are real captures
from dev.api.olympuscloud.ai — same as olympus-sdk-dart#8 +
olympus-sdk-typescript#1 Wave 1 PRs.

## 0.3.0 (2026-04-10)

Major release adding 6 new services. 19 services total.

### New Services

- **Creator** — Posts, media, profile, analytics, AI content generation, team
- **Platform** — Tenant signup/cleanup workflows, lifecycle management
- **Developer** — API keys, DevBox sandboxes, canary deployments
- **Business** — Revenue summary/trends, top sellers, on-duty staff, AI insights
- **Maximus** — Consumer AI assistant: voice, calendar, email, billing
- **POS** — Voice order routing to Square/Toast/Clover

### Migration

```go
package main

import (
    "context"
    "github.com/OlympusCloud/olympus-sdk-go"
)

func main() {
    client := olympus.NewClient(olympus.Config{
        AppID:  "com.my-app",
        APIKey: "oc_live_...",
    })

    ctx := context.Background()

    // New v0.3.0 services
    posts, _ := client.Creator().ListPosts(ctx, olympus.ListPostsOptions{Status: "published"})
    tenant, _ := client.Platform().Signup(ctx, olympus.SignupRequest{
        CompanyName: "Acme",
        AdminEmail:  "owner@acme.com",
        AdminName:   "Bob",
        Industry:    "restaurant",
    })
    revenue, _ := client.Business().GetRevenueSummary(ctx)
    response, _ := client.Maximus().VoiceQuery(ctx, "What sold best today?")
}
```

## 0.1.0

Initial release with 13 services: Auth, Commerce, AI, Pay, Notify, Events,
Data, Storage, Marketplace, Billing, Gating, Devices, Observe.
