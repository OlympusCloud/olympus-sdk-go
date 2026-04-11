package olympus

import (
	"context"
	"fmt"
	"net/url"
)

// StorageService handles file storage operations backed by Cloudflare R2.
//
// Routes: /storage/*.
type StorageService struct {
	http *httpClient
}

// Upload uploads binary data (base64-encoded) to a path and returns the public URL.
func (s *StorageService) Upload(ctx context.Context, contentBase64, path string) (string, error) {
	resp, err := s.http.post(ctx, "/storage/upload", map[string]interface{}{
		"path":    path,
		"content": contentBase64,
	})
	if err != nil {
		return "", err
	}
	return getString(resp, "url"), nil
}

// GetURL returns the public or signed URL for a stored object.
func (s *StorageService) GetURL(ctx context.Context, path string) (string, error) {
	q := url.Values{}
	q.Set("path", path)

	resp, err := s.http.get(ctx, "/storage/url", q)
	if err != nil {
		return "", err
	}
	return getString(resp, "url"), nil
}

// PresignUpload generates a pre-signed upload URL for direct client uploads.
// expiresInSeconds is the validity duration (default: 3600).
func (s *StorageService) PresignUpload(ctx context.Context, path string, expiresInSeconds int) (string, error) {
	body := map[string]interface{}{
		"path": path,
	}
	if expiresInSeconds > 0 {
		body["expires_in"] = expiresInSeconds
	}

	resp, err := s.http.post(ctx, "/storage/presign", body)
	if err != nil {
		return "", err
	}

	if v := getString(resp, "url"); v != "" {
		return v, nil
	}
	return getString(resp, "presigned_url"), nil
}

// Delete removes a stored object.
func (s *StorageService) Delete(ctx context.Context, path string) error {
	return s.http.del(ctx, fmt.Sprintf("/storage/objects/%s", path))
}
