# olympus-sdk-go

Official Go SDK for [Olympus Cloud](https://olympuscloud.ai) — the AI Business Operating System.

Typed Go client for 19 platform services: Auth, Commerce, AI, Pay, Notify, Events, Data, Storage, Marketplace, Billing, Gating, Devices, Observe, Creator, Platform, Developer, Business, Maximus, POS.

## Install

```bash
go get github.com/OlympusCloud/olympus-sdk-go
```

## Quick start

```go
package main

import (
    "context"
    "fmt"
    olympus "github.com/OlympusCloud/olympus-sdk-go"
)

func main() {
    client := olympus.NewClient(olympus.Config{
        AppID:  "com.my-app",
        APIKey: "oc_live_...",
    })

    ctx := context.Background()

    // Create an order
    order, err := client.Commerce().CreateOrder(ctx, olympus.CreateOrderRequest{
        Items:  []olympus.OrderItem{{CatalogID: "burger-01", Qty: 2, Price: 1299}},
        Source: "pos",
    })
    if err != nil {
        panic(err)
    }
    fmt.Println("Order:", order)

    // v0.3.0 new services
    posts, _ := client.Creator().ListPosts(ctx, olympus.ListPostsOptions{Status: "published"})
    fmt.Println("Posts:", posts)

    revenue, _ := client.Business().GetRevenueSummary(ctx)
    fmt.Println("Revenue:", revenue)

    // Localize a platform error code in the user's preferred language.
    // Manifest is fetched once and cached for 1 hour.
    msg, _ := client.I18n().Localize(ctx, "ORDER_NOT_FOUND", "es")
    fmt.Println(msg)

    // Or localize a typed *OlympusAPIError directly.
    if _, err := client.Commerce().GetOrder(ctx, "missing"); err != nil {
        if apiErr, ok := err.(*olympus.OlympusAPIError); ok {
            friendly, _ := client.I18n().LocalizeError(ctx, apiErr, "fr")
            fmt.Println(friendly)
        }
    }
}
```

## Services

| Service | Description |
|---------|-------------|
| `client.Auth()` | Authentication, user management, API keys |
| `client.Commerce()` | Orders, catalog, POS sessions |
| `client.AI()` | AI inference, agents, embeddings |
| `client.Pay()` | Payments, refunds, terminal |
| `client.Notify()` | Push, SMS, email |
| `client.Events()` | Real-time events, webhooks |
| `client.I18n()` | Error-code → localized message manifest (#3637) |
| `client.Data()` | Query, CRUD, search |
| `client.Storage()` | File upload, R2-backed |
| `client.Marketplace()` | Apps marketplace |
| `client.Billing()` | Subscriptions, invoices |
| `client.Gating()` | Feature flags |
| `client.Devices()` | MDM |
| `client.Observe()` | Observability |
| `client.Creator()` 🆕 | Creator platform: posts, media, AI content |
| `client.Platform()` 🆕 | Tenant signup/cleanup workflows |
| `client.Developer()` 🆕 | API keys, DevBox, canary deployments |
| `client.Business()` 🆕 | Revenue, staff, AI insights |
| `client.Maximus()` 🆕 | Consumer AI assistant |
| `client.POS()` 🆕 | POS voice order integration |
| `client.Voice()` | Voice agents — configs, conversations, campaigns, phone numbers, marketplace voices, edge pipeline |
| `client.VoiceOrders()` | Voice-collected order placement (with menu price validation) |
| `client.SMS()` 🆕 | Outbound SMS via voice platform + unified CPaaS (Telnyx/Twilio) |
| `client.SmartHome()` 🆕 | Smart-home platforms, devices, scenes, automations |
| `client.Identity()` 🆕 | Olympus ID — global cross-tenant identity + age verification |
| `client.Connect()` | Marketing-funnel + lead capture |
| `client.Consent()` | App-scoped permissions consent surface (v2.0) |
| `client.Governance()` | Policy exception framework (v2.0) |

## License

Apache-2.0

## Links

- [Olympus Cloud Docs](https://docs.olympuscloud.ai)
- [Issue Tracker](https://github.com/OlympusCloud/olympus-cloud-gcp/issues)
- [TypeScript SDK](https://github.com/OlympusCloud/olympus-sdk-typescript)
- [Dart SDK](https://github.com/OlympusCloud/olympus-sdk-dart)
- [Changelog](./CHANGELOG.md)
