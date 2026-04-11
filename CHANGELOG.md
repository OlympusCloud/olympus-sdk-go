# Changelog

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
