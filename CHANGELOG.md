# Changelog

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
