package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// CreatorService wraps the Rust Creator backend (port 8004).
//
// Provides access to posts, media, episodes, profile, analytics, team,
// branding, calendar, AI content generation, social posts, and shows.
//
// v0.3.0 — Issue #2839
type CreatorService struct {
	http *httpClient
}

// ListPosts retrieves paginated posts with optional filters.
func (s *CreatorService) ListPosts(ctx context.Context, opts ListPostsOptions) (map[string]interface{}, error) {
	q := url.Values{}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.ContentType != "" {
		q.Set("content_type", opts.ContentType)
	}
	if opts.Search != "" {
		q.Set("search", opts.Search)
	}
	if opts.Page > 0 {
		q.Set("page", fmt.Sprintf("%d", opts.Page))
	}
	if opts.PageSize > 0 {
		q.Set("page_size", fmt.Sprintf("%d", opts.PageSize))
	}
	return s.http.get(ctx, "/api/v1/posts", q)
}

// ListPostsOptions configures the ListPosts query.
type ListPostsOptions struct {
	Status      string
	ContentType string
	Search      string
	Page        int
	PageSize    int
}

// CreatePost creates a new post.
func (s *CreatorService) CreatePost(ctx context.Context, post map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/api/v1/posts", post)
}

// GetPost retrieves a post by ID.
func (s *CreatorService) GetPost(ctx context.Context, postID string) (map[string]interface{}, error) {
	return s.http.get(ctx, "/api/v1/posts/"+postID, nil)
}

// UpdatePost updates a post.
func (s *CreatorService) UpdatePost(ctx context.Context, postID string, updates map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, "/api/v1/posts/"+postID, updates)
}

// DeletePost deletes a post.
func (s *CreatorService) DeletePost(ctx context.Context, postID string) error {
	return s.http.del(ctx, "/api/v1/posts/"+postID)
}

// PublishPost publishes a post (optionally scheduled).
func (s *CreatorService) PublishPost(ctx context.Context, postID string, body map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/api/v1/posts/"+postID+"/publish", body)
}

// SchedulePost schedules a post for future publishing.
func (s *CreatorService) SchedulePost(ctx context.Context, postID string, body map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/api/v1/posts/"+postID+"/schedule", body)
}

// ListMedia returns paginated media files.
func (s *CreatorService) ListMedia(ctx context.Context, opts map[string]string) (map[string]interface{}, error) {
	q := url.Values{}
	for k, v := range opts {
		q.Set(k, v)
	}
	return s.http.get(ctx, "/creator/media", q)
}

// InitiateUpload starts a media upload (returns presigned URL).
func (s *CreatorService) InitiateUpload(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/creator/media/upload", body)
}

// ConfirmUpload confirms upload completion.
func (s *CreatorService) ConfirmUpload(ctx context.Context, mediaID string, metadata map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/creator/media/"+mediaID+"/confirm", metadata)
}

// DeleteMedia deletes a media file.
func (s *CreatorService) DeleteMedia(ctx context.Context, mediaID string) error {
	return s.http.del(ctx, "/creator/media/"+mediaID)
}

// GetStorageStats returns storage usage stats.
func (s *CreatorService) GetStorageStats(ctx context.Context) (map[string]interface{}, error) {
	return s.http.get(ctx, "/creator/media/storage", nil)
}

// GetProfile returns the creator profile.
func (s *CreatorService) GetProfile(ctx context.Context) (map[string]interface{}, error) {
	return s.http.get(ctx, "/creator/profile", nil)
}

// UpdateProfile updates the creator profile.
func (s *CreatorService) UpdateProfile(ctx context.Context, updates map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, "/creator/profile", updates)
}

// GetAnalyticsSummary returns reach, engagement, and revenue metrics.
func (s *CreatorService) GetAnalyticsSummary(ctx context.Context, period string) (map[string]interface{}, error) {
	q := url.Values{}
	if period == "" {
		period = "30d"
	}
	q.Set("period", period)
	return s.http.get(ctx, "/creator/analytics/summary", q)
}

// GetContentAnalytics returns per-content analytics.
func (s *CreatorService) GetContentAnalytics(ctx context.Context, contentID string) (map[string]interface{}, error) {
	return s.http.get(ctx, "/creator/analytics/content/"+contentID, nil)
}

// GetAudienceInsights returns demographics and locations.
func (s *CreatorService) GetAudienceInsights(ctx context.Context) (map[string]interface{}, error) {
	return s.http.get(ctx, "/creator/analytics/audience", nil)
}

// GenerateContent generates AI content from a prompt.
func (s *CreatorService) GenerateContent(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/creator/ai/generate", body)
}

// ListAITemplates returns AI content templates.
func (s *CreatorService) ListAITemplates(ctx context.Context) (map[string]interface{}, error) {
	return s.http.get(ctx, "/creator/ai/templates", nil)
}

// ListTeam returns team members.
func (s *CreatorService) ListTeam(ctx context.Context) (map[string]interface{}, error) {
	return s.http.get(ctx, "/creator/team", nil)
}

// InviteTeamMember invites a team member.
func (s *CreatorService) InviteTeamMember(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/creator/team/invite", body)
}

// RemoveTeamMember removes a team member.
func (s *CreatorService) RemoveTeamMember(ctx context.Context, memberID string) error {
	return s.http.del(ctx, "/creator/team/"+memberID)
}
