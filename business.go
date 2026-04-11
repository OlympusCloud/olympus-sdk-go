package olympus

import (
	"context"
	"net/url"
)

// BusinessService wraps business data access endpoints used by consumer apps
// like Maximus to show business owner dashboards.
//
// v0.3.0 — Issue #2570
type BusinessService struct {
	http *httpClient
}

// GetRevenueSummary returns revenue across today/week/month/year periods.
func (s *BusinessService) GetRevenueSummary(ctx context.Context) (map[string]interface{}, error) {
	return s.http.get(ctx, "/business/revenue/summary", nil)
}

// GetRevenueTrends returns trend data points for charting.
func (s *BusinessService) GetRevenueTrends(ctx context.Context, period string) (map[string]interface{}, error) {
	q := url.Values{}
	if period == "" {
		period = "30d"
	}
	q.Set("period", period)
	return s.http.get(ctx, "/business/revenue/trends", q)
}

// GetTopSellers returns top-selling items by revenue.
func (s *BusinessService) GetTopSellers(ctx context.Context, limit int, period string) (map[string]interface{}, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", string(rune(limit)))
	}
	if period != "" {
		q.Set("period", period)
	}
	return s.http.get(ctx, "/business/top-sellers", q)
}

// GetOnDutyStaff returns currently on-duty staff members.
func (s *BusinessService) GetOnDutyStaff(ctx context.Context, locationID string) (map[string]interface{}, error) {
	q := url.Values{}
	if locationID != "" {
		q.Set("location_id", locationID)
	}
	return s.http.get(ctx, "/business/staff/on-duty", q)
}

// GetInsights returns AI-generated business insights.
func (s *BusinessService) GetInsights(ctx context.Context, category string) (map[string]interface{}, error) {
	q := url.Values{}
	if category != "" {
		q.Set("category", category)
	}
	return s.http.get(ctx, "/business/insights", q)
}

// GetComparisons returns period-over-period metric comparisons.
func (s *BusinessService) GetComparisons(ctx context.Context, currentPeriod, compareTo string) (map[string]interface{}, error) {
	q := url.Values{}
	if currentPeriod == "" {
		currentPeriod = "this_month"
	}
	if compareTo == "" {
		compareTo = "last_month"
	}
	q.Set("current_period", currentPeriod)
	q.Set("compare_to", compareTo)
	return s.http.get(ctx, "/business/comparisons", q)
}
