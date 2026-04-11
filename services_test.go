package olympus

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// --------------------------------------------------------------------------
// Auth service tests
// --------------------------------------------------------------------------

func TestAuthLogin(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/auth/login": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["email"] != "user@test.com" {
				t.Errorf("expected email user@test.com, got %v", body["email"])
			}
			jsonResponse(w, 200, map[string]interface{}{
				"access_token":  "tok_abc",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "ref_xyz",
				"user_id":       "usr-1",
				"tenant_id":     "ten-1",
				"roles":         []string{"manager"},
			})
		},
	})

	ctx := context.Background()
	session, err := client.Auth().Login(ctx, "user@test.com", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.AccessToken != "tok_abc" {
		t.Errorf("expected access_token tok_abc, got %s", session.AccessToken)
	}
	if session.RefreshToken != "ref_xyz" {
		t.Errorf("expected refresh_token ref_xyz, got %s", session.RefreshToken)
	}
	if session.UserID != "usr-1" {
		t.Errorf("expected user_id usr-1, got %s", session.UserID)
	}
	if len(session.Roles) != 1 || session.Roles[0] != "manager" {
		t.Errorf("unexpected roles: %v", session.Roles)
	}
	// Verify token was set on client
	if client.HTTPClient().accessToken != "tok_abc" {
		t.Error("access token not set on HTTP client after login")
	}
}

func TestAuthLoginPin(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/auth/login/pin": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["pin"] != "123456" {
				t.Errorf("expected pin 123456, got %v", body["pin"])
			}
			if body["location_id"] != "loc-1" {
				t.Errorf("expected location_id loc-1, got %v", body["location_id"])
			}
			jsonResponse(w, 200, map[string]interface{}{
				"access_token": "pin_tok",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		},
	})

	ctx := context.Background()
	session, err := client.Auth().LoginPin(ctx, "123456", "loc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.AccessToken != "pin_tok" {
		t.Errorf("expected access_token pin_tok, got %s", session.AccessToken)
	}
}

func TestAuthMe(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/auth/me": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id":        "usr-1",
				"email":     "test@example.com",
				"name":      "Test User",
				"roles":     []string{"admin"},
				"tenant_id": "ten-1",
				"status":    "active",
			})
		},
	})

	ctx := context.Background()
	user, err := client.Auth().Me(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "usr-1" {
		t.Errorf("expected id usr-1, got %s", user.ID)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}
	if user.Name != "Test User" {
		t.Errorf("expected name Test User, got %s", user.Name)
	}
}

func TestAuthLogout(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/auth/logout": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("{}"))
		},
	})

	client.HTTPClient().SetAccessToken("some-token")
	ctx := context.Background()
	err := client.Auth().Logout(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.HTTPClient().accessToken != "" {
		t.Error("access token not cleared after logout")
	}
}

func TestAuthCreateUser(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/users": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "New User" {
				t.Errorf("expected name 'New User', got %v", body["name"])
			}
			jsonResponse(w, 201, map[string]interface{}{
				"id":    "usr-new",
				"email": "new@test.com",
				"name":  "New User",
			})
		},
	})

	ctx := context.Background()
	user, err := client.Auth().CreateUser(ctx, CreateUserRequest{
		Name:  "New User",
		Email: "new@test.com",
		Role:  "staff",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "usr-new" {
		t.Errorf("expected id usr-new, got %s", user.ID)
	}
}

func TestAuthCheckPermission(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/users/usr-1/permissions/check": func(w http.ResponseWriter, r *http.Request) {
			perm := r.URL.Query().Get("permission")
			if perm != "orders.create" {
				t.Errorf("expected permission orders.create, got %s", perm)
			}
			jsonResponse(w, 200, map[string]interface{}{"allowed": true})
		},
	})

	ctx := context.Background()
	allowed, err := client.Auth().CheckPermission(ctx, "usr-1", "orders.create")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected allowed=true")
	}
}

func TestAuthCreateAPIKey(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/tenants/me/api-keys": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 201, map[string]interface{}{
				"id":     "key-1",
				"name":   "My Key",
				"key":    "oc_live_secret",
				"scopes": []string{"orders.read"},
			})
		},
	})

	ctx := context.Background()
	key, err := client.Auth().CreateAPIKey(ctx, "My Key", []string{"orders.read"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.Key != "oc_live_secret" {
		t.Errorf("expected key oc_live_secret, got %s", key.Key)
	}
}

// --------------------------------------------------------------------------
// Commerce service tests
// --------------------------------------------------------------------------

func TestCommerceCreateOrder(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/commerce/orders": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["source"] != "pos" {
				t.Errorf("expected source pos, got %v", body["source"])
			}
			jsonResponse(w, 201, map[string]interface{}{
				"id":     "ord-1",
				"status": "pending",
				"source": "pos",
				"total":  2598,
				"items": []interface{}{
					map[string]interface{}{
						"catalog_id": "burger-01",
						"qty":        2,
						"price":      1299,
					},
				},
			})
		},
	})

	ctx := context.Background()
	order, err := client.Commerce().CreateOrder(ctx, CreateOrderRequest{
		Items:  []OrderItem{{CatalogID: "burger-01", Qty: 2, Price: 1299}},
		Source: "pos",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.ID != "ord-1" {
		t.Errorf("expected id ord-1, got %s", order.ID)
	}
	if order.Total != 2598 {
		t.Errorf("expected total 2598, got %d", order.Total)
	}
	if len(order.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(order.Items))
	}
	if order.Items[0].CatalogID != "burger-01" {
		t.Errorf("expected catalog_id burger-01, got %s", order.Items[0].CatalogID)
	}
}

func TestCommerceGetOrder(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/commerce/orders/ord-1": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id":     "ord-1",
				"status": "completed",
			})
		},
	})

	ctx := context.Background()
	order, err := client.Commerce().GetOrder(ctx, "ord-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != "completed" {
		t.Errorf("expected status completed, got %s", order.Status)
	}
}

func TestCommerceUpdateOrderStatus(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/commerce/orders/ord-1/status": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id":     "ord-1",
				"status": "preparing",
			})
		},
	})

	ctx := context.Background()
	order, err := client.Commerce().UpdateOrderStatus(ctx, "ord-1", "preparing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != "preparing" {
		t.Errorf("expected status preparing, got %s", order.Status)
	}
}

func TestCommerceCancelOrder(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/commerce/orders/ord-1/cancel": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("{}"))
		},
	})

	ctx := context.Background()
	err := client.Commerce().CancelOrder(ctx, "ord-1", "customer request")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommerceGetCatalog(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/central-menu/items": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"id": "item-1", "name": "Burger", "price": 1299},
					map[string]interface{}{"id": "item-2", "name": "Fries", "price": 499},
				},
			})
		},
	})

	ctx := context.Background()
	items, err := client.Commerce().GetCatalog(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "Burger" {
		t.Errorf("expected name Burger, got %s", items[0].Name)
	}
	if items[1].Price != 499 {
		t.Errorf("expected price 499, got %d", items[1].Price)
	}
}

// --------------------------------------------------------------------------
// AI service tests
// --------------------------------------------------------------------------

func TestAIQuery(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/ai/chat": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"content":       "Burgers sold best this week",
				"model":         "gemini-flash",
				"tier":          "T2",
				"tokens_used":   150,
				"finish_reason": "stop",
				"request_id":    "req-ai-1",
			})
		},
	})

	ctx := context.Background()
	resp, err := client.AI().Query(ctx, "What sold best?", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Burgers sold best this week" {
		t.Errorf("unexpected content: %s", resp.Content)
	}
	if resp.Model != "gemini-flash" {
		t.Errorf("expected model gemini-flash, got %s", resp.Model)
	}
	if resp.TokensUsed != 150 {
		t.Errorf("expected tokens_used 150, got %d", resp.TokensUsed)
	}
}

func TestAIQueryWithOptions(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/ai/chat": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["tier"] != "T3" {
				t.Errorf("expected tier T3, got %v", body["tier"])
			}
			jsonResponse(w, 200, map[string]interface{}{"content": "answer"})
		},
	})

	ctx := context.Background()
	_, err := client.AI().Query(ctx, "test", &QueryOptions{Tier: "T3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAIInvokeAgent(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/agent/invoke": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["agent"] != "rex" {
				t.Errorf("expected agent rex, got %v", body["agent"])
			}
			jsonResponse(w, 200, map[string]interface{}{
				"output":     "Revenue is up 15%",
				"agent_name": "rex",
				"steps": []interface{}{
					map[string]interface{}{
						"action":      "query_analytics",
						"observation": "Retrieved revenue data",
					},
				},
				"tokens_used": 250,
			})
		},
	})

	ctx := context.Background()
	result, err := client.AI().InvokeAgent(ctx, "rex", "analyze revenue", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "Revenue is up 15%" {
		t.Errorf("unexpected output: %s", result.Output)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(result.Steps))
	}
	if result.Steps[0].Action != "query_analytics" {
		t.Errorf("unexpected step action: %s", result.Steps[0].Action)
	}
}

func TestAIEmbed(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/ai/embeddings": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"embedding": []interface{}{0.1, 0.2, 0.3, 0.4},
					},
				},
			})
		},
	})

	ctx := context.Background()
	vec, err := client.AI().Embed(ctx, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != 4 {
		t.Fatalf("expected 4 dimensions, got %d", len(vec))
	}
	if vec[0] != 0.1 {
		t.Errorf("expected first dim 0.1, got %f", vec[0])
	}
}

func TestAIClassify(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/ai/classify": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"label":      "complaint",
				"confidence": 0.92,
				"scores": map[string]interface{}{
					"complaint":  0.92,
					"praise":     0.05,
					"suggestion": 0.03,
				},
			})
		},
	})

	ctx := context.Background()
	result, err := client.AI().Classify(ctx, "the food was cold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Label != "complaint" {
		t.Errorf("expected label complaint, got %s", result.Label)
	}
	if result.Confidence != 0.92 {
		t.Errorf("expected confidence 0.92, got %f", result.Confidence)
	}
}

func TestAITranslate(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/translation/translate": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"translated_text": "Hola mundo",
			})
		},
	})

	ctx := context.Background()
	result, err := client.AI().Translate(ctx, "Hello world", "es")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hola mundo" {
		t.Errorf("expected 'Hola mundo', got '%s'", result)
	}
}

// --------------------------------------------------------------------------
// Pay service tests
// --------------------------------------------------------------------------

func TestPayCharge(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/payments/intents": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["order_id"] != "ord-1" {
				t.Errorf("expected order_id ord-1, got %v", body["order_id"])
			}
			jsonResponse(w, 200, map[string]interface{}{
				"id":       "pay-1",
				"status":   "succeeded",
				"order_id": "ord-1",
				"amount":   2499,
				"currency": "USD",
			})
		},
	})

	ctx := context.Background()
	payment, err := client.Pay().Charge(ctx, "ord-1", 2499, "pm_card_visa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payment.ID != "pay-1" {
		t.Errorf("expected id pay-1, got %s", payment.ID)
	}
	if payment.Amount != 2499 {
		t.Errorf("expected amount 2499, got %d", payment.Amount)
	}
}

func TestPayRefund(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/payments/pay-1/refund": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id":         "ref-1",
				"payment_id": "pay-1",
				"status":     "succeeded",
				"amount":     500,
				"reason":     "wrong item",
			})
		},
	})

	ctx := context.Background()
	refund, err := client.Pay().Refund(ctx, "pay-1", &RefundOptions{Amount: 500, Reason: "wrong item"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refund.Amount != 500 {
		t.Errorf("expected amount 500, got %d", refund.Amount)
	}
}

func TestPayGetBalance(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/finance/balance": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"available": 150000,
				"pending":   25000,
				"currency":  "USD",
			})
		},
	})

	ctx := context.Background()
	balance, err := client.Pay().GetBalance(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balance.Available != 150000 {
		t.Errorf("expected available 150000, got %d", balance.Available)
	}
	if balance.Total() != 175000 {
		t.Errorf("expected total 175000, got %d", balance.Total())
	}
}

// --------------------------------------------------------------------------
// Billing service tests
// --------------------------------------------------------------------------

func TestBillingGetCurrentPlan(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/billing/subscription": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id":            "plan-blaze",
				"name":          "Blaze",
				"tier":          "blaze",
				"monthly_price": 9900,
				"max_locations": 5,
				"ai_credits":    15000,
			})
		},
	})

	ctx := context.Background()
	plan, err := client.Billing().GetCurrentPlan(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Name != "Blaze" {
		t.Errorf("expected plan name Blaze, got %s", plan.Name)
	}
	if plan.MonthlyPrice != 9900 {
		t.Errorf("expected monthly_price 9900, got %d", plan.MonthlyPrice)
	}
}

func TestBillingGetUsage(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/billing/stats": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"period":           "2026-04",
				"ai_credits_used":  5000,
				"ai_credits_limit": 15000,
				"api_calls_count":  42000,
			})
		},
	})

	ctx := context.Background()
	usage, err := client.Billing().GetUsage(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.AICreditsUsed != 5000 {
		t.Errorf("expected ai_credits_used 5000, got %d", usage.AICreditsUsed)
	}
}

// --------------------------------------------------------------------------
// Gating service tests
// --------------------------------------------------------------------------

func TestGatingIsEnabled(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/policies/evaluate": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"allowed": true,
			})
		},
	})

	ctx := context.Background()
	enabled, err := client.Gating().IsEnabled(ctx, "feature.online_ordering")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Error("expected feature to be enabled")
	}
}

func TestGatingEvaluate(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/policies/evaluate": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"allowed": true,
				"value":   "premium",
				"reason":  "plan allows feature",
			})
		},
	})

	ctx := context.Background()
	result, err := client.Gating().Evaluate(ctx, "ai.tier_access", map[string]interface{}{
		"location_id": "loc-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected allowed=true")
	}
	if result.Reason != "plan allows feature" {
		t.Errorf("unexpected reason: %s", result.Reason)
	}
}

// --------------------------------------------------------------------------
// Events service tests
// --------------------------------------------------------------------------

func TestEventsPublish(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/events/publish": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["event_type"] != "order.created" {
				t.Errorf("expected event_type order.created, got %v", body["event_type"])
			}
			w.WriteHeader(200)
			w.Write([]byte("{}"))
		},
	})

	ctx := context.Background()
	err := client.Events().Publish(ctx, "order.created", map[string]interface{}{
		"order_id": "ord-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEventsWebhookRegister(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/platform/tenants/me/webhooks": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				jsonResponse(w, 201, map[string]interface{}{
					"id":     "wh-1",
					"url":    "https://example.com/webhook",
					"events": []interface{}{"order.created"},
					"secret": "whsec_abc",
				})
			}
		},
	})

	ctx := context.Background()
	wh, err := client.Events().WebhookRegister(ctx, "https://example.com/webhook", []string{"order.created"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wh.ID != "wh-1" {
		t.Errorf("expected id wh-1, got %s", wh.ID)
	}
	if wh.Secret != "whsec_abc" {
		t.Errorf("expected secret whsec_abc, got %s", wh.Secret)
	}
}

// --------------------------------------------------------------------------
// Storage service tests
// --------------------------------------------------------------------------

func TestStorageUpload(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/storage/upload": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"url": "https://cdn.olympuscloud.ai/images/burger.webp",
			})
		},
	})

	ctx := context.Background()
	url, err := client.Storage().Upload(ctx, "base64data", "images/burger.webp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://cdn.olympuscloud.ai/images/burger.webp" {
		t.Errorf("unexpected URL: %s", url)
	}
}

func TestStoragePresignUpload(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/storage/presign": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"url": "https://storage.example.com/presigned?token=abc",
			})
		},
	})

	ctx := context.Background()
	url, err := client.Storage().PresignUpload(ctx, "images/upload.png", 3600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty presign URL")
	}
}

// --------------------------------------------------------------------------
// Marketplace service tests
// --------------------------------------------------------------------------

func TestMarketplaceListApps(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/marketplace/apps": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"apps": []interface{}{
					map[string]interface{}{
						"id":            "app-1",
						"name":          "Inventory Pro",
						"category":      "operations",
						"rating":        4.5,
						"install_count": 1200,
					},
				},
			})
		},
	})

	ctx := context.Background()
	apps, err := client.Marketplace().ListApps(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Name != "Inventory Pro" {
		t.Errorf("expected name Inventory Pro, got %s", apps[0].Name)
	}
	if apps[0].Rating != 4.5 {
		t.Errorf("expected rating 4.5, got %f", apps[0].Rating)
	}
}

func TestMarketplaceInstall(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/marketplace/apps/app-1/install": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id":       "inst-1",
				"app_id":   "app-1",
				"app_name": "Inventory Pro",
				"status":   "installed",
			})
		},
	})

	ctx := context.Background()
	inst, err := client.Marketplace().Install(ctx, "app-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Status != "installed" {
		t.Errorf("expected status installed, got %s", inst.Status)
	}
}

// --------------------------------------------------------------------------
// Devices service tests
// --------------------------------------------------------------------------

func TestDevicesEnroll(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/auth/devices/register": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id":       "dev-1",
				"name":     "Kiosk #3",
				"profile":  "kiosk",
				"status":   "enrolled",
				"platform": "android",
			})
		},
	})

	ctx := context.Background()
	device, err := client.Devices().Enroll(ctx, "hw-abc", "kiosk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if device.Profile != "kiosk" {
		t.Errorf("expected profile kiosk, got %s", device.Profile)
	}
}

func TestDevicesListDevices(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/diagnostics/devices": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"devices": []interface{}{
					map[string]interface{}{
						"id":       "dev-1",
						"name":     "POS #1",
						"platform": "android",
						"status":   "online",
					},
					map[string]interface{}{
						"id":       "dev-2",
						"name":     "KDS #1",
						"platform": "android",
						"status":   "offline",
					},
				},
			})
		},
	})

	ctx := context.Background()
	devices, err := client.Devices().ListDevices(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
}

// --------------------------------------------------------------------------
// Notify service tests
// --------------------------------------------------------------------------

func TestNotifyPush(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/notifications/push": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["user_id"] != "usr-1" {
				t.Errorf("expected user_id usr-1, got %v", body["user_id"])
			}
			if body["title"] != "Order Ready" {
				t.Errorf("expected title 'Order Ready', got %v", body["title"])
			}
			w.WriteHeader(200)
			w.Write([]byte("{}"))
		},
	})

	ctx := context.Background()
	err := client.Notify().Push(ctx, "usr-1", "Order Ready", "Your order #42 is ready for pickup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotifyEmail(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/messaging/email": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("{}"))
		},
	})

	ctx := context.Background()
	err := client.Notify().Email(ctx, "user@test.com", "Welcome", "<h1>Welcome</h1>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --------------------------------------------------------------------------
// Data service tests
// --------------------------------------------------------------------------

func TestDataQuery(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/data/query": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"rows": []interface{}{
					map[string]interface{}{"id": "1", "name": "Burger", "count": 42},
					map[string]interface{}{"id": "2", "name": "Fries", "count": 38},
				},
			})
		},
	})

	ctx := context.Background()
	rows, err := client.Data().Query(ctx, "SELECT * FROM orders", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestDataInsert(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/data/customers": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, 201, map[string]interface{}{
				"id":    "cust-1",
				"name":  "Jane Doe",
				"email": "jane@test.com",
			})
		},
	})

	ctx := context.Background()
	result, err := client.Data().Insert(ctx, "customers", map[string]interface{}{
		"name":  "Jane Doe",
		"email": "jane@test.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "cust-1" {
		t.Errorf("expected id cust-1, got %v", result["id"])
	}
}

// --------------------------------------------------------------------------
// Observe service tests
// --------------------------------------------------------------------------

func TestObserveLogEvent(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/monitoring/client/events": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["event"] != "page_view" {
				t.Errorf("expected event page_view, got %v", body["event"])
			}
			w.WriteHeader(200)
			w.Write([]byte("{}"))
		},
	})

	ctx := context.Background()
	err := client.Observe().LogEvent(ctx, "page_view", map[string]interface{}{
		"page": "/menu",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestObserveStartTrace(t *testing.T) {
	_, client := testServer(t, map[string]http.HandlerFunc{
		"/monitoring/client/traces": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "checkout" {
				t.Errorf("expected name checkout, got %v", body["name"])
			}
			if body["duration_ms"] == nil {
				t.Error("expected duration_ms to be set")
			}
			w.WriteHeader(200)
			w.Write([]byte("{}"))
		},
	})

	ctx := context.Background()
	span := client.Observe().StartTrace(ctx, "checkout")
	if span.TraceID == "" {
		t.Error("expected non-empty trace ID")
	}
	if span.Name != "checkout" {
		t.Errorf("expected name checkout, got %s", span.Name)
	}

	// Verify elapsed works before end
	elapsed := span.Elapsed()
	if elapsed < 0 {
		t.Error("expected non-negative elapsed duration")
	}

	err := span.End(ctx)
	if err != nil {
		t.Fatalf("unexpected error ending trace: %v", err)
	}
}

// --------------------------------------------------------------------------
// Model helpers tests
// --------------------------------------------------------------------------

func TestAgentTaskStatusHelpers(t *testing.T) {
	pending := AgentTask{Status: "pending"}
	running := AgentTask{Status: "running"}
	completed := AgentTask{Status: "completed"}
	failed := AgentTask{Status: "failed"}

	if !pending.IsPending() {
		t.Error("pending should be IsPending()")
	}
	if !running.IsPending() {
		t.Error("running should be IsPending()")
	}
	if !completed.IsCompleted() {
		t.Error("completed should be IsCompleted()")
	}
	if !failed.IsFailed() {
		t.Error("failed should be IsFailed()")
	}
	if completed.IsFailed() {
		t.Error("completed should not be IsFailed()")
	}
}

func TestPaginationHelpers(t *testing.T) {
	p := Pagination{Page: 2, PerPage: 20, Total: 100, TotalPages: 5}
	if !p.HasNextPage() {
		t.Error("expected HasNextPage() true for page 2/5")
	}
	if !p.HasPreviousPage() {
		t.Error("expected HasPreviousPage() true for page 2")
	}

	first := Pagination{Page: 1, TotalPages: 5}
	if first.HasPreviousPage() {
		t.Error("expected HasPreviousPage() false for page 1")
	}

	last := Pagination{Page: 5, TotalPages: 5}
	if last.HasNextPage() {
		t.Error("expected HasNextPage() false for page 5/5")
	}
}
