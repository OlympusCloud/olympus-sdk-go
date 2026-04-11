package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// MarketplaceService handles app marketplace discovery, installation, and management.
//
// Routes: /marketplace/*.
type MarketplaceService struct {
	http *httpClient
}

// ListAppsOptions holds optional filters for listing marketplace apps.
type ListAppsOptions struct {
	Category string
	Industry string
	Query    string
	Limit    int
}

// ListApps lists available marketplace apps with optional filters.
func (s *MarketplaceService) ListApps(ctx context.Context, opts *ListAppsOptions) ([]MarketplaceApp, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Category != "" {
			q.Set("category", opts.Category)
		}
		if opts.Industry != "" {
			q.Set("industry", opts.Industry)
		}
		if opts.Query != "" {
			q.Set("q", opts.Query)
		}
		if opts.Limit > 0 {
			q.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
	}

	resp, err := s.http.get(ctx, "/marketplace/apps", q)
	if err != nil {
		return nil, err
	}

	apps := parseSlice(resp, "apps", parseMarketplaceApp)
	if len(apps) == 0 {
		apps = parseSlice(resp, "data", parseMarketplaceApp)
	}
	return apps, nil
}

// GetApp returns details for a single marketplace app.
func (s *MarketplaceService) GetApp(ctx context.Context, appID string) (*MarketplaceApp, error) {
	resp, err := s.http.get(ctx, fmt.Sprintf("/marketplace/apps/%s", appID), nil)
	if err != nil {
		return nil, err
	}
	return parseMarketplaceApp(resp), nil
}

// Install installs a marketplace app for the current tenant.
func (s *MarketplaceService) Install(ctx context.Context, appID string) (*Installation, error) {
	resp, err := s.http.post(ctx, fmt.Sprintf("/marketplace/apps/%s/install", appID), nil)
	if err != nil {
		return nil, err
	}
	return parseInstallation(resp), nil
}

// Uninstall removes a marketplace app.
func (s *MarketplaceService) Uninstall(ctx context.Context, appID string) error {
	_, err := s.http.post(ctx, fmt.Sprintf("/marketplace/apps/%s/uninstall", appID), nil)
	return err
}

// GetInstalled lists apps currently installed for the tenant.
func (s *MarketplaceService) GetInstalled(ctx context.Context) ([]Installation, error) {
	resp, err := s.http.get(ctx, "/marketplace/installed", nil)
	if err != nil {
		return nil, err
	}

	installs := parseSlice(resp, "installations", parseInstallation)
	if len(installs) == 0 {
		installs = parseSlice(resp, "data", parseInstallation)
	}
	return installs, nil
}

// Review submits a review for a marketplace app.
func (s *MarketplaceService) Review(ctx context.Context, appID string, rating int, text string) error {
	_, err := s.http.post(ctx, fmt.Sprintf("/marketplace/apps/%s/reviews", appID), map[string]interface{}{
		"rating": rating,
		"text":   text,
	})
	return err
}
