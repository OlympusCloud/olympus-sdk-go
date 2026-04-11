package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// DataService handles data queries, CRUD, and search operations.
//
// Routes: /data/*, /ai/search.
type DataService struct {
	http *httpClient
}

// Query executes a read-only SQL query against the platform data layer.
// Returns rows as a list of column-name-keyed maps.
func (s *DataService) Query(ctx context.Context, sql string, params map[string]interface{}) ([]map[string]interface{}, error) {
	body := map[string]interface{}{
		"sql": sql,
	}
	if params != nil {
		body["params"] = params
	}

	resp, err := s.http.post(ctx, "/data/query", body)
	if err != nil {
		return nil, err
	}

	return parseRawSlice(resp, "rows", "data"), nil
}

// Insert inserts a record into a table.
func (s *DataService) Insert(ctx context.Context, table string, record map[string]interface{}) (map[string]interface{}, error) {
	resp, err := s.http.post(ctx, fmt.Sprintf("/data/%s", table), record)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Update updates a record by ID.
func (s *DataService) Update(ctx context.Context, table, id string, fields map[string]interface{}) (map[string]interface{}, error) {
	resp, err := s.http.patch(ctx, fmt.Sprintf("/data/%s/%s", table, id), fields)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Delete deletes a record by ID.
func (s *DataService) Delete(ctx context.Context, table, id string) error {
	return s.http.del(ctx, fmt.Sprintf("/data/%s/%s", table, id))
}

// SearchOptions holds optional parameters for search.
type SearchOptions struct {
	Scope string
	Limit int
}

// Search performs full-text or semantic search across indexed data.
func (s *DataService) Search(ctx context.Context, query string, opts *SearchOptions) ([]SearchResult, error) {
	body := map[string]interface{}{
		"query": query,
	}
	if opts != nil {
		if opts.Scope != "" {
			body["scope"] = opts.Scope
		}
		if opts.Limit > 0 {
			body["limit"] = opts.Limit
		}
	}

	resp, err := s.http.post(ctx, "/ai/search", body)
	if err != nil {
		return nil, err
	}
	return parseSlice(resp, "results", parseSearchResult), nil
}

// parseRawSlice extracts a raw JSON array from a response, trying multiple keys.
func parseRawSlice(data map[string]interface{}, keys ...string) []map[string]interface{} {
	for _, key := range keys {
		if items, ok := data[key].([]interface{}); ok {
			result := make([]map[string]interface{}, 0, len(items))
			for _, item := range items {
				if m, ok := item.(map[string]interface{}); ok {
					result = append(result, m)
				}
			}
			return result
		}
	}
	return nil
}

// mapToValues converts a string-string map to url.Values.
func mapToValues(m map[string]string) url.Values {
	v := url.Values{}
	for key, val := range m {
		v.Set(key, val)
	}
	return v
}
