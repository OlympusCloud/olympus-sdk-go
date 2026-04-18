package olympus

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// TuningService handles AI tuning jobs, synthetic persona generation,
// and chaos audio simulation for voice pipeline testing.
//
// Routes: /v1/tuning/*, /v1/personas/*, /v1/chaos/audio/*.
type TuningService struct {
	http *httpClient
}

// ---------------------------------------------------------------------------
// Tuning Jobs
// ---------------------------------------------------------------------------

// CreateTuningJob creates a new tuning job.
//
// jobType identifies the tuning strategy (e.g. "lora", "full", "distillation",
// "rlhf"). parameters carries model-specific config such as base_model,
// dataset_id, epochs, learning_rate, etc.
func (s *TuningService) CreateTuningJob(ctx context.Context, jobType string, parameters map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/v1/tuning/jobs", map[string]interface{}{
		"job_type":   jobType,
		"parameters": parameters,
	})
}

// ListTuningJobs lists tuning jobs with optional status filter and limit.
func (s *TuningService) ListTuningJobs(ctx context.Context, status string, limit int) (map[string]interface{}, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return s.http.get(ctx, "/v1/tuning/jobs", q)
}

// GetTuningJob returns details for a single tuning job.
func (s *TuningService) GetTuningJob(ctx context.Context, jobID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/v1/tuning/jobs/%s", jobID), nil)
}

// CancelTuningJob cancels a running or queued tuning job.
func (s *TuningService) CancelTuningJob(ctx context.Context, jobID string) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/v1/tuning/jobs/%s/cancel", jobID), nil)
}

// GetTuningResults returns the results of a completed tuning job, including
// metrics, evaluation scores, and the output model artifact reference.
func (s *TuningService) GetTuningResults(ctx context.Context, jobID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/v1/tuning/jobs/%s/results", jobID), nil)
}

// ---------------------------------------------------------------------------
// Synthetic Persona Generation
// ---------------------------------------------------------------------------

// GeneratePersona generates a single synthetic persona for load/QA testing.
//
// config specifies persona attributes such as locale, accent, speaking_style,
// vocabulary_level, noise_profile, and intent_distribution.
func (s *TuningService) GeneratePersona(ctx context.Context, config map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/v1/personas/generate", config)
}

// GeneratePersonaBatch generates a batch of synthetic personas.
//
// count is the number of personas (1-1000). distribution defines the
// statistical distribution of persona characteristics.
func (s *TuningService) GeneratePersonaBatch(ctx context.Context, count int, distribution map[string]interface{}) (map[string]interface{}, error) {
	return s.http.post(ctx, "/v1/personas/batch", map[string]interface{}{
		"count":        count,
		"distribution": distribution,
	})
}

// ---------------------------------------------------------------------------
// Chaos Audio Simulation
// ---------------------------------------------------------------------------

// SimulateNoise applies environmental noise to an audio sample for chaos testing.
//
// audioBase64 is base64-encoded audio (WAV or MP3). noiseType selects the
// profile: background_chatter, drive_thru_wind, kitchen_noise, traffic, rain,
// static, crowd, or machinery. intensity is 0.0-1.0 controlling noise level.
func (s *TuningService) SimulateNoise(ctx context.Context, audioBase64, noiseType string, intensity float64) (map[string]interface{}, error) {
	return s.http.post(ctx, "/v1/chaos/audio/simulate", map[string]interface{}{
		"audio_base64": audioBase64,
		"noise_type":   noiseType,
		"intensity":    intensity,
	})
}
