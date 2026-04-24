package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// VoiceMarketplaceService handles voice marketplace reviews and adjacent
// catalog endpoints.
//
// Routes: /voice/marketplace/*.
type VoiceMarketplaceService struct {
	http *httpClient
}

// VoiceReview is a single published review for a marketplace voice.
type VoiceReview struct {
	ID        string `json:"id"`
	VoiceID   string `json:"voice_id"`
	Author    string `json:"author_name"`
	// AuthorTenantID is the HMAC-hashed tenant_id (16 hex chars). Stable
	// per tenant for de-duplication; one-way so the underlying tenant_id
	// is not recoverable.
	AuthorTenantID string `json:"author_tenant_id"`
	Rating         int    `json:"rating"`
	Text           string `json:"text"`
	CreatedAt      string `json:"created_at"`
}

// VoiceReviewsPage is a paginated list with aggregate stats.
type VoiceReviewsPage struct {
	Reviews []VoiceReview `json:"reviews"`
	Total   int           `json:"total"`
	Average float64       `json:"average"`
	Limit   int           `json:"limit"`
	Offset  int           `json:"offset"`
}

// VoiceReviewSubmitResponse is returned by SubmitReview.
type VoiceReviewSubmitResponse struct {
	ID        string `json:"id"`
	VoiceID   string `json:"voice_id"`
	Rating    int    `json:"rating"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// ListReviews returns published reviews for a voice with average rating.
func (s *VoiceMarketplaceService) ListReviews(ctx context.Context, voiceID string, limit, offset int) (*VoiceReviewsPage, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		q.Set("offset", fmt.Sprintf("%d", offset))
	}
	resp, err := s.http.get(ctx, fmt.Sprintf("/voice/marketplace/voices/%s/reviews", voiceID), q)
	if err != nil {
		return nil, err
	}
	return parseVoiceReviewsPage(resp), nil
}

// SubmitReview submits a 1..5 star review for a marketplace voice.
//
// One review per (user, voice) — duplicate submissions return 409.
func (s *VoiceMarketplaceService) SubmitReview(ctx context.Context, voiceID string, rating int, text string) (*VoiceReviewSubmitResponse, error) {
	resp, err := s.http.post(ctx, fmt.Sprintf("/voice/marketplace/voices/%s/reviews", voiceID), map[string]interface{}{
		"rating": rating,
		"text":   text,
	})
	if err != nil {
		return nil, err
	}
	return parseVoiceReviewSubmitResponse(resp), nil
}

// DeleteReview soft-deletes the caller's own review.
func (s *VoiceMarketplaceService) DeleteReview(ctx context.Context, reviewID string) error {
	return s.http.del(ctx, fmt.Sprintf("/voice/marketplace/voices/reviews/%s", reviewID))
}

func parseVoiceReview(m map[string]interface{}) VoiceReview {
	return VoiceReview{
		ID:             getString(m, "id"),
		VoiceID:        getString(m, "voice_id"),
		Author:         getString(m, "author_name"),
		AuthorTenantID: getString(m, "author_tenant_id"),
		Rating:         getInt(m, "rating"),
		Text:           getString(m, "text"),
		CreatedAt:      getString(m, "created_at"),
	}
}

func parseVoiceReviewsPage(m map[string]interface{}) *VoiceReviewsPage {
	page := &VoiceReviewsPage{
		Total:   getInt(m, "total"),
		Average: getFloat64(m, "average"),
		Limit:   getInt(m, "limit"),
		Offset:  getInt(m, "offset"),
	}
	if raw, ok := m["reviews"].([]interface{}); ok {
		page.Reviews = make([]VoiceReview, 0, len(raw))
		for _, r := range raw {
			if rm, ok := r.(map[string]interface{}); ok {
				page.Reviews = append(page.Reviews, parseVoiceReview(rm))
			}
		}
	}
	return page
}

func parseVoiceReviewSubmitResponse(m map[string]interface{}) *VoiceReviewSubmitResponse {
	return &VoiceReviewSubmitResponse{
		ID:        getString(m, "id"),
		VoiceID:   getString(m, "voice_id"),
		Rating:    getInt(m, "rating"),
		Text:      getString(m, "text"),
		CreatedAt: getString(m, "created_at"),
	}
}
