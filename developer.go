package olympus

import "context"

// DeveloperService wraps developer platform endpoints: API key management,
// DevBox sandboxes, and canary deployments.
//
// v0.3.0 — Issues #2834, #2835, #2828, #2829
type DeveloperService struct {
	http *httpClient
}

// CreateAPIKey generates a new API key for a developer.
func (s *DeveloperService) CreateAPIKey(ctx context.Context, developerID string, body map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/developers/"+developerID+"/keys", body)
}

// ListAPIKeys returns API keys for a developer (masked).
func (s *DeveloperService) ListAPIKeys(ctx context.Context, developerID string) (map[string]interface{}, error) {
	return s.http.get(ctx, "/developers/"+developerID+"/keys", nil)
}

// RevokeAPIKey revokes an API key.
func (s *DeveloperService) RevokeAPIKey(ctx context.Context, developerID, keyID string) error {
	return s.http.del(ctx, "/developers/"+developerID+"/keys/"+keyID)
}

// RotateAPIKey revokes the old key and creates a new one.
func (s *DeveloperService) RotateAPIKey(ctx context.Context, developerID, keyID string) (map[string]interface{}, error) {
	return s.http.post(ctx, "/developers/"+developerID+"/keys/"+keyID+"/rotate", nil)
}

// ProvisionDevBox creates a new DevBox sandbox.
func (s *DeveloperService) ProvisionDevBox(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/devbox/provision", body)
}

// GetDevBoxSession returns DevBox session info.
func (s *DeveloperService) GetDevBoxSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return s.http.get(ctx, "/devbox/"+sessionID, nil)
}

// TerminateDevBox terminates a DevBox session.
func (s *DeveloperService) TerminateDevBox(ctx context.Context, sessionID string) error {
	return s.http.del(ctx, "/devbox/"+sessionID)
}

// ListCollaborators returns DevBox session collaborators.
func (s *DeveloperService) ListCollaborators(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return s.http.get(ctx, "/devbox/"+sessionID+"/collaborators", nil)
}

// InviteCollaborator invites a user to a DevBox session.
func (s *DeveloperService) InviteCollaborator(ctx context.Context, sessionID string, body map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/devbox/"+sessionID+"/invite", body)
}

// DeployApp deploys an app version as a canary.
func (s *DeveloperService) DeployApp(ctx context.Context, developerID, appID string, body map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/developers/"+developerID+"/apps/"+appID+"/deploy", body)
}

// PromoteDeployment promotes a canary to 100% traffic.
func (s *DeveloperService) PromoteDeployment(ctx context.Context, developerID, appID, deployID string) (map[string]interface{}, error) {
	return s.http.post(ctx, "/developers/"+developerID+"/apps/"+appID+"/deployments/"+deployID+"/promote", nil)
}

// RollbackDeployment rolls back to the previous version.
func (s *DeveloperService) RollbackDeployment(ctx context.Context, developerID, appID, deployID string) (map[string]interface{}, error) {
	return s.http.post(ctx, "/developers/"+developerID+"/apps/"+appID+"/deployments/"+deployID+"/rollback", nil)
}
