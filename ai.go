package olympus

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// AIService handles AI inference, agent orchestration, embeddings, and NLP.
//
// Wraps the Olympus AI Gateway (Python) via the Go API Gateway.
// Routes: /ai/*, /agent/*, /translation/*.
type AIService struct {
	http *httpClient
}

// QueryOptions holds optional parameters for a single-turn AI query.
//
// RequiredCapabilities activates capability-based routing (#2919). Non-text
// values bypass the text tier selector and route to the cheapest model in
// the Ether catalog matching the capabilities. Values: text, vision,
// audio_in, audio_out, audio_live, video_in, video_generation, video_live,
// image_generation, embedding, reasoning, agentic_coding, world_model,
// robotics_control, medical_specialist, legal_specialist,
// financial_specialist, scientific_specialist, function_calling,
// structured_output, long_context.
type QueryOptions struct {
	Tier                 string                 `json:"tier,omitempty"`
	Context              map[string]interface{} `json:"context,omitempty"`
	RequiredCapabilities []string               `json:"required_capabilities,omitempty"`
}

// Query sends a single-turn prompt to the AI gateway.
func (s *AIService) Query(ctx context.Context, prompt string, opts *QueryOptions) (*AIResponse, error) {
	body := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	if opts != nil {
		if opts.Tier != "" {
			body["tier"] = opts.Tier
		}
		if opts.Context != nil {
			body["context"] = opts.Context
		}
		if len(opts.RequiredCapabilities) > 0 {
			body["required_capabilities"] = opts.RequiredCapabilities
		}
	}

	resp, err := s.http.post(ctx, "/ai/chat", body)
	if err != nil {
		return nil, err
	}
	return parseAIResponse(resp), nil
}

// GenerateImageOptions holds optional parameters for image generation.
type GenerateImageOptions struct {
	PreferredProvider string `json:"preferred_provider,omitempty"`
}

// GenerateImage creates an image from a text prompt via the cheapest matching
// provider in the Ether catalog (Flux Schnell free, DALL-E 3, Imagen 4).
// Returns a raw map with image_url or image_b64 depending on provider.
func (s *AIService) GenerateImage(ctx context.Context, prompt string, opts *GenerateImageOptions) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"required_capabilities": []string{"image_generation"},
	}
	if opts != nil && opts.PreferredProvider != "" {
		body["preferred_provider"] = opts.PreferredProvider
	}
	return s.http.post(ctx, "/ai/chat", body)
}

// GenerateVideoOptions holds optional parameters for video generation.
type GenerateVideoOptions struct {
	DurationSeconds   int    `json:"duration_seconds,omitempty"`
	PreferredProvider string `json:"preferred_provider,omitempty"`
}

// GenerateVideo creates a video from a text prompt (Veo / Kling / Pika / Luma / Hailuo).
// Returns async job reference — poll /ai/video-jobs/{id} for completion.
func (s *AIService) GenerateVideo(ctx context.Context, prompt string, opts *GenerateVideoOptions) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"required_capabilities": []string{"video_generation"},
	}
	if opts != nil {
		if opts.DurationSeconds > 0 {
			body["duration_seconds"] = opts.DurationSeconds
		}
		if opts.PreferredProvider != "" {
			body["preferred_provider"] = opts.PreferredProvider
		}
	}
	return s.http.post(ctx, "/ai/chat", body)
}

// SpecialistQuery calls a vertical specialist model (medical/legal/financial/scientific).
// Routes to Med-Gemini, Harvey, BloombergGPT, ESM-3, etc. based on the
// specialty flag. Valid values: "medical", "legal", "financial", "scientific".
// systemContext is an optional domain-specific system prompt.
func (s *AIService) SpecialistQuery(ctx context.Context, prompt, specialty, systemContext string) (*AIResponse, error) {
	capMap := map[string]string{
		"medical":    "medical_specialist",
		"legal":      "legal_specialist",
		"financial":  "financial_specialist",
		"scientific": "scientific_specialist",
	}
	capName, ok := capMap[specialty]
	if !ok {
		return nil, fmt.Errorf("olympus-sdk: unknown specialty %q; must be one of medical/legal/financial/scientific", specialty)
	}
	messages := []map[string]string{}
	if systemContext != "" {
		messages = append(messages, map[string]string{"role": "system", "content": systemContext})
	}
	messages = append(messages, map[string]string{"role": "user", "content": prompt})
	body := map[string]interface{}{
		"messages":              messages,
		"required_capabilities": []string{"reasoning", capName},
	}
	resp, err := s.http.post(ctx, "/ai/chat", body)
	if err != nil {
		return nil, err
	}
	return parseAIResponse(resp), nil
}

// Chat sends a multi-turn chat completion request.
func (s *AIService) Chat(ctx context.Context, messages []ChatMessage, model string) (*AIResponse, error) {
	body := map[string]interface{}{
		"messages": messages,
	}
	if model != "" {
		body["model"] = model
	}

	resp, err := s.http.post(ctx, "/ai/chat", body)
	if err != nil {
		return nil, err
	}
	return parseAIResponse(resp), nil
}

// StreamCallback is called for each chunk of a streaming AI response.
type StreamCallback func(chunk string)

// Stream sends a prompt and streams the response chunk-by-chunk via SSE.
// The callback is invoked for each content delta. Returns the accumulated
// full response text.
func (s *AIService) Stream(ctx context.Context, prompt string, callback StreamCallback) (string, error) {
	fullURL := s.http.baseURL + "/ai/chat"

	bodyData, err := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": true,
	})
	if err != nil {
		return "", fmt.Errorf("olympus-sdk: failed to marshal stream request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(string(bodyData)))
	if err != nil {
		return "", fmt.Errorf("olympus-sdk: failed to create stream request: %w", err)
	}

	s.http.applyHeaders(req)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := s.http.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("olympus-sdk: stream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		return "", s.http.parseError(resp.StatusCode, body[:n])
	}

	var accumulated strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}

		payload := line[6:]
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
			continue
		}

		content := extractStreamContent(parsed)
		if content != "" {
			if callback != nil {
				callback(content)
			}
			accumulated.WriteString(content)
		}
	}

	return accumulated.String(), scanner.Err()
}

// extractStreamContent pulls the content delta from an SSE chunk payload.
func extractStreamContent(parsed map[string]interface{}) string {
	// OpenAI-compatible format: choices[0].delta.content
	if choices, ok := parsed["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if content, ok := delta["content"].(string); ok {
					return content
				}
			}
		}
	}
	// Flat format: content
	if content, ok := parsed["content"].(string); ok {
		return content
	}
	return ""
}

// InvokeAgent invokes a LangGraph agent synchronously.
func (s *AIService) InvokeAgent(ctx context.Context, agentName, task string, params map[string]interface{}) (*AgentResult, error) {
	body := map[string]interface{}{
		"agent": agentName,
		"task":  task,
	}
	if params != nil {
		body["params"] = params
	}

	resp, err := s.http.post(ctx, "/agent/invoke", body)
	if err != nil {
		return nil, err
	}
	return parseAgentResult(resp), nil
}

// CreateTask creates an asynchronous agent task.
func (s *AIService) CreateTask(ctx context.Context, agent, task string, requiresApproval, notifyOnComplete bool) (*AgentTask, error) {
	body := map[string]interface{}{
		"agent":              agent,
		"task":               task,
		"requires_approval":  requiresApproval,
		"notify_on_complete": notifyOnComplete,
	}

	resp, err := s.http.post(ctx, "/agent/tasks", body)
	if err != nil {
		return nil, err
	}
	return parseAgentTask(resp), nil
}

// GetTaskStatus polls the status of an async agent task.
func (s *AIService) GetTaskStatus(ctx context.Context, taskID string) (*AgentTask, error) {
	resp, err := s.http.get(ctx, fmt.Sprintf("/agent/tasks/%s", taskID), nil)
	if err != nil {
		return nil, err
	}
	return parseAgentTask(resp), nil
}

// Embed generates an embedding vector for the given text.
func (s *AIService) Embed(ctx context.Context, text string) ([]float64, error) {
	resp, err := s.http.post(ctx, "/ai/embeddings", map[string]interface{}{
		"input": text,
	})
	if err != nil {
		return nil, err
	}

	// OpenAI-compatible format: data[0].embedding
	if data, ok := resp["data"].([]interface{}); ok && len(data) > 0 {
		if entry, ok := data[0].(map[string]interface{}); ok {
			if emb, ok := entry["embedding"].([]interface{}); ok {
				return toFloat64Slice(emb), nil
			}
		}
	}
	// Flat format: embedding
	if emb, ok := resp["embedding"].([]interface{}); ok {
		return toFloat64Slice(emb), nil
	}

	return nil, nil
}

// Search performs semantic search over indexed content.
func (s *AIService) Search(ctx context.Context, query, index string, limit int) ([]SearchResult, error) {
	body := map[string]interface{}{
		"query": query,
	}
	if index != "" {
		body["index"] = index
	}
	if limit > 0 {
		body["limit"] = limit
	}

	resp, err := s.http.post(ctx, "/ai/search", body)
	if err != nil {
		return nil, err
	}

	return parseSlice(resp, "results", parseSearchResult), nil
}

// Classify classifies text into categories.
func (s *AIService) Classify(ctx context.Context, text string) (*Classification, error) {
	resp, err := s.http.post(ctx, "/ai/classify", map[string]interface{}{
		"text": text,
	})
	if err != nil {
		return nil, err
	}
	return parseClassification(resp), nil
}

// Translate translates text to the given target language (ISO 639-1 code).
func (s *AIService) Translate(ctx context.Context, text, targetLang string) (string, error) {
	resp, err := s.http.post(ctx, "/translation/translate", map[string]interface{}{
		"text":            text,
		"target_language": targetLang,
	})
	if err != nil {
		return "", err
	}

	if v, ok := resp["translated_text"].(string); ok {
		return v, nil
	}
	if v, ok := resp["translation"].(string); ok {
		return v, nil
	}
	return "", nil
}

// Sentiment analyzes the sentiment of text.
func (s *AIService) Sentiment(ctx context.Context, text string) (*SentimentResult, error) {
	resp, err := s.http.post(ctx, "/ai/sentiment", map[string]interface{}{
		"text": text,
	})
	if err != nil {
		return nil, err
	}
	return parseSentimentResult(resp), nil
}

// STT performs speech-to-text transcription of base64-encoded audio.
func (s *AIService) STT(ctx context.Context, audioBase64 string) (string, error) {
	resp, err := s.http.post(ctx, "/ai/stt", map[string]interface{}{
		"audio": audioBase64,
	})
	if err != nil {
		return "", err
	}

	if v, ok := resp["text"].(string); ok {
		return v, nil
	}
	if v, ok := resp["transcript"].(string); ok {
		return v, nil
	}
	return "", nil
}

// TTS performs text-to-speech synthesis. Returns base64-encoded audio.
func (s *AIService) TTS(ctx context.Context, text, voiceID string) (string, error) {
	body := map[string]interface{}{
		"text": text,
	}
	if voiceID != "" {
		body["voice_id"] = voiceID
	}

	resp, err := s.http.post(ctx, "/ai/tts", body)
	if err != nil {
		return "", err
	}

	if v, ok := resp["audio"].(string); ok {
		return v, nil
	}
	return "", nil
}
