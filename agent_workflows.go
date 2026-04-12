// AI Agent Workflow Orchestration client — wraps /agent-workflows/* routes (#2915).
//
// Distinct from WorkflowsService which handles marketplace templates.
// See olympus-cloud-gcp issue #2915 for full architecture.

package olympus

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// AgentWorkflowsService provides tenant-scoped multi-agent DAG workflows (#2915).
//
// A workflow is a directed acyclic graph of agent nodes. Each node invokes
// an agent from the tenant's registry. Triggers can be manual, cron-based
// (via Cloud Scheduler), or event-driven (order.created, inventory.low, etc.).
//
// Free tier: 100 executions, 1000 agent messages, 10k D1 queries per month.
type AgentWorkflowsService struct {
	http *httpClient
}

// ListWorkflowsOptions filters workflow listings.
type ListWorkflowsOptions struct {
	// Status filters by "draft", "active", "paused", "archived".
	Status string
	// Limit caps the number of returned workflows.
	Limit int
}

// List returns workflows for the current tenant.
func (s *AgentWorkflowsService) List(ctx context.Context, opts *ListWorkflowsOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Status != "" {
			q.Set("status", opts.Status)
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	resp, err := s.http.get(ctx, "/agent-workflows", q)
	if err != nil {
		return nil, err
	}
	return extractMapSlice(resp, "workflows", "data"), nil
}

// Get returns a single workflow by ID with its full DAG schema.
func (s *AgentWorkflowsService) Get(ctx context.Context, workflowID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/agent-workflows/%s", workflowID), nil)
}

// CreateWorkflowRequest defines a new workflow.
type CreateWorkflowRequest struct {
	// Name is the human-readable workflow name.
	Name string `json:"name"`
	// Description is an optional workflow description.
	Description string `json:"description,omitempty"`
	// Schema is the DAG definition with nodes (agent steps) and edges (data flow).
	Schema map[string]interface{} `json:"schema"`
	// Triggers is a list of trigger configs: cron/event/manual.
	Triggers []map[string]interface{} `json:"triggers,omitempty"`
}

// Create provisions a new workflow.
func (s *AgentWorkflowsService) Create(ctx context.Context, req CreateWorkflowRequest) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"name":   req.Name,
		"schema": req.Schema,
	}
	if req.Description != "" {
		body["description"] = req.Description
	}
	if req.Triggers != nil {
		body["triggers"] = req.Triggers
	}
	return s.http.post(ctx, "/agent-workflows", body)
}

// Update modifies an existing workflow. Pass only fields to change.
func (s *AgentWorkflowsService) Update(ctx context.Context, workflowID string, updates map[string]interface{}) (map[string]interface{}, error) {
	return s.http.put(ctx, fmt.Sprintf("/agent-workflows/%s", workflowID), updates)
}

// Delete soft-deletes (archives) a workflow.
func (s *AgentWorkflowsService) Delete(ctx context.Context, workflowID string) error {
	return s.http.del(ctx, fmt.Sprintf("/agent-workflows/%s", workflowID))
}

// Execute manually triggers a workflow execution with optional input payload.
// Returns the execution ID — poll GetExecution for results.
func (s *AgentWorkflowsService) Execute(ctx context.Context, workflowID string, input map[string]interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	if input != nil {
		body["input"] = input
	}
	return s.http.post(ctx, fmt.Sprintf("/agent-workflows/%s/execute", workflowID), body)
}

// ListExecutionsOptions filters execution history.
type ListExecutionsOptions struct {
	Status string
	Limit  int
}

// ListExecutions returns execution history for a workflow.
func (s *AgentWorkflowsService) ListExecutions(ctx context.Context, workflowID string, opts *ListExecutionsOptions) ([]map[string]interface{}, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Status != "" {
			q.Set("status", opts.Status)
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	resp, err := s.http.get(ctx, fmt.Sprintf("/agent-workflows/%s/executions", workflowID), q)
	if err != nil {
		return nil, err
	}
	return extractMapSlice(resp, "executions", "data"), nil
}

// GetExecution returns full execution detail including per-step results.
func (s *AgentWorkflowsService) GetExecution(ctx context.Context, executionID string) (map[string]interface{}, error) {
	return s.http.get(ctx, fmt.Sprintf("/agent-workflow-executions/%s", executionID), nil)
}

// SetSchedule sets or updates the cron schedule for a workflow.
// cronExpression follows standard cron: "minute hour day month weekday".
func (s *AgentWorkflowsService) SetSchedule(ctx context.Context, workflowID, cronExpression string) (map[string]interface{}, error) {
	return s.http.post(ctx, fmt.Sprintf("/agent-workflows/%s/schedule", workflowID), map[string]interface{}{
		"cron_expression": cronExpression,
	})
}

// RemoveSchedule clears the cron schedule from a workflow.
func (s *AgentWorkflowsService) RemoveSchedule(ctx context.Context, workflowID string) error {
	return s.http.del(ctx, fmt.Sprintf("/agent-workflows/%s/schedule", workflowID))
}

// Usage returns current month usage vs tenant tier limits.
func (s *AgentWorkflowsService) Usage(ctx context.Context) (map[string]interface{}, error) {
	return s.http.get(ctx, "/agent-workflows/usage", nil)
}

// extractMapSlice returns the first non-nil slice under any of the given keys,
// cast to []map[string]interface{}.
func extractMapSlice(resp map[string]interface{}, keys ...string) []map[string]interface{} {
	for _, k := range keys {
		raw, ok := resp[k].([]interface{})
		if !ok || raw == nil {
			continue
		}
		out := make([]map[string]interface{}, 0, len(raw))
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return []map[string]interface{}{}
}
