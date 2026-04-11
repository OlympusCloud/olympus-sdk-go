package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// CommerceService handles orders, catalog, and commerce operations.
//
// Wraps the Olympus Commerce service (Rust) via the Go API Gateway.
// Routes: /commerce/*, /central-menu/*.
type CommerceService struct {
	http *httpClient
}

// CreateOrderRequest holds the parameters for creating an order.
type CreateOrderRequest struct {
	Items      []OrderItem `json:"items"`
	Source     string      `json:"source"`
	TableID    string      `json:"table_id,omitempty"`
	CustomerID string      `json:"customer_id,omitempty"`
}

// CreateOrder creates a new order.
func (s *CommerceService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
	body := map[string]interface{}{
		"items":  req.Items,
		"source": req.Source,
	}
	if req.TableID != "" {
		body["table_id"] = req.TableID
	}
	if req.CustomerID != "" {
		body["customer_id"] = req.CustomerID
	}

	resp, err := s.http.post(ctx, "/commerce/orders", body)
	if err != nil {
		return nil, err
	}
	return parseOrder(resp), nil
}

// GetOrder retrieves a single order by ID.
func (s *CommerceService) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	resp, err := s.http.get(ctx, fmt.Sprintf("/commerce/orders/%s", orderID), nil)
	if err != nil {
		return nil, err
	}
	return parseOrder(resp), nil
}

// ListOrdersOptions holds optional filters for listing orders.
type ListOrdersOptions struct {
	Page   int
	Limit  int
	Status string
}

// ListOrders lists orders with optional filters and pagination.
func (s *CommerceService) ListOrders(ctx context.Context, opts *ListOrdersOptions) (*PaginatedResponse[Order], error) {
	q := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			q.Set("page", fmt.Sprintf("%d", opts.Page))
		}
		if opts.Limit > 0 {
			q.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
		if opts.Status != "" {
			q.Set("status", opts.Status)
		}
	}

	resp, err := s.http.get(ctx, "/commerce/orders", q)
	if err != nil {
		return nil, err
	}

	orders := parseSlice(resp, "data", parseOrder)
	return &PaginatedResponse[Order]{
		Data:       orders,
		Pagination: parsePagination(resp),
	}, nil
}

// UpdateOrderStatus transitions an order to a new status.
func (s *CommerceService) UpdateOrderStatus(ctx context.Context, orderID, status string) (*Order, error) {
	resp, err := s.http.patch(ctx, fmt.Sprintf("/commerce/orders/%s/status", orderID), map[string]interface{}{
		"status": status,
	})
	if err != nil {
		return nil, err
	}
	return parseOrder(resp), nil
}

// CancelOrder cancels an order with a reason.
func (s *CommerceService) CancelOrder(ctx context.Context, orderID, reason string) error {
	_, err := s.http.post(ctx, fmt.Sprintf("/commerce/orders/%s/cancel", orderID), map[string]interface{}{
		"reason": reason,
	})
	return err
}

// AddOrderItems adds items to an existing order.
func (s *CommerceService) AddOrderItems(ctx context.Context, orderID string, items []OrderItem) (*Order, error) {
	resp, err := s.http.post(ctx, fmt.Sprintf("/commerce/orders/%s/items", orderID), map[string]interface{}{
		"items": items,
	})
	if err != nil {
		return nil, err
	}
	return parseOrder(resp), nil
}

// CreateCatalogItemRequest holds the parameters for creating a catalog item.
type CreateCatalogItemRequest struct {
	Name        string            `json:"name"`
	Price       int               `json:"price"`
	Category    string            `json:"category,omitempty"`
	Description string            `json:"description,omitempty"`
	ImageURL    string            `json:"image_url,omitempty"`
	Modifiers   []CatalogModifier `json:"modifiers,omitempty"`
}

// CreateCatalogItem creates a new catalog item (menu item, product, etc.).
func (s *CommerceService) CreateCatalogItem(ctx context.Context, req CreateCatalogItemRequest) (*CatalogItem, error) {
	body := map[string]interface{}{
		"name":  req.Name,
		"price": req.Price,
	}
	if req.Category != "" {
		body["category"] = req.Category
	}
	if req.Description != "" {
		body["description"] = req.Description
	}
	if req.ImageURL != "" {
		body["image_url"] = req.ImageURL
	}
	if req.Modifiers != nil {
		body["modifiers"] = req.Modifiers
	}

	resp, err := s.http.post(ctx, "/central-menu/items", body)
	if err != nil {
		return nil, err
	}
	return parseCatalogItem(resp), nil
}

// GetCatalog retrieves the catalog, optionally filtered by category.
func (s *CommerceService) GetCatalog(ctx context.Context, categoryID string) ([]CatalogItem, error) {
	q := url.Values{}
	if categoryID != "" {
		q.Set("category_id", categoryID)
	}

	resp, err := s.http.get(ctx, "/central-menu/items", q)
	if err != nil {
		return nil, err
	}

	items := parseSlice(resp, "items", parseCatalogItem)
	if len(items) == 0 {
		items = parseSlice(resp, "data", parseCatalogItem)
	}
	return items, nil
}

// GetCatalogItem retrieves a single catalog item by ID.
func (s *CommerceService) GetCatalogItem(ctx context.Context, itemID string) (*CatalogItem, error) {
	resp, err := s.http.get(ctx, fmt.Sprintf("/central-menu/items/%s", itemID), nil)
	if err != nil {
		return nil, err
	}
	return parseCatalogItem(resp), nil
}

// UpdateCatalogItemRequest holds the parameters for updating a catalog item.
type UpdateCatalogItemRequest struct {
	Name        *string `json:"name,omitempty"`
	Price       *int    `json:"price,omitempty"`
	Category    *string `json:"category,omitempty"`
	Description *string `json:"description,omitempty"`
	Available   *bool   `json:"available,omitempty"`
}

// UpdateCatalogItem updates an existing catalog item.
func (s *CommerceService) UpdateCatalogItem(ctx context.Context, itemID string, req UpdateCatalogItemRequest) (*CatalogItem, error) {
	body := map[string]interface{}{}
	if req.Name != nil {
		body["name"] = *req.Name
	}
	if req.Price != nil {
		body["price"] = *req.Price
	}
	if req.Category != nil {
		body["category"] = *req.Category
	}
	if req.Description != nil {
		body["description"] = *req.Description
	}
	if req.Available != nil {
		body["available"] = *req.Available
	}

	resp, err := s.http.patch(ctx, fmt.Sprintf("/central-menu/items/%s", itemID), body)
	if err != nil {
		return nil, err
	}
	return parseCatalogItem(resp), nil
}

// DeleteCatalogItem deletes a catalog item.
func (s *CommerceService) DeleteCatalogItem(ctx context.Context, itemID string) error {
	return s.http.del(ctx, fmt.Sprintf("/central-menu/items/%s", itemID))
}
