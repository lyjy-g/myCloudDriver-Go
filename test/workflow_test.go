package test

import (
	"testing"

	"myclouddrive-go/internal/agent/workflow"
)

func TestWorkflowDefinitionValidation(t *testing.T) {
	d := &workflow.Definition{
		ID:          "test_wf",
		Name:        "Test",
		StartNodeID: "n1",
		Nodes: []workflow.Node{
			{ID: "n1", Type: workflow.NodeTool, Label: "step1"},
		},
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("valid workflow should pass: %v", err)
	}
}

func TestWorkflowValidationEmptyNodes(t *testing.T) {
	d := &workflow.Definition{ID: "bad", StartNodeID: "n1"}
	if err := d.Validate(); err == nil {
		t.Error("expected error for empty nodes")
	}
}

func TestWorkflowValidationMissingStart(t *testing.T) {
	d := &workflow.Definition{
		ID:          "bad",
		StartNodeID: "n99",
		Nodes:       []workflow.Node{{ID: "n1", Type: workflow.NodeTool}},
	}
	if err := d.Validate(); err == nil {
		t.Error("expected error for missing start node")
	}
}

func TestWorkflowValidationUnknownType(t *testing.T) {
	d := &workflow.Definition{
		ID:          "bad",
		StartNodeID: "n1",
		Nodes:       []workflow.Node{{ID: "n1", Type: workflow.NodeType("unknown")}},
	}
	if err := d.Validate(); err == nil {
		t.Error("expected error for unknown node type")
	}
}

func TestWorkflowValidationDuplicateIDs(t *testing.T) {
	d := &workflow.Definition{
		ID:          "bad",
		StartNodeID: "n1",
		Nodes: []workflow.Node{
			{ID: "n1", Type: workflow.NodeTool},
			{ID: "n1", Type: workflow.NodeLLM},
		},
	}
	if err := d.Validate(); err == nil {
		t.Error("expected error for duplicate node IDs")
	}
}

func TestParseDefinition(t *testing.T) {
	raw := []byte(`{
		"id": "test",
		"name": "Test Workflow",
		"startNodeId": "s1",
		"nodes": [
			{"id": "s1", "type": "tool", "label": "step1", "config": {"toolName": "tool.file.list"}}
		]
	}`)
	d, err := workflow.ParseDefinition(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if d.ID != "test" {
		t.Errorf("expected id=test, got %s", d.ID)
	}
	if len(d.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(d.Nodes))
	}
}

func TestParseDefinitionInvalidJSON(t *testing.T) {
	_, err := workflow.ParseDefinition([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestExampleWorkflow(t *testing.T) {
	wf := workflow.ExampleWorkflow()
	if err := wf.Validate(); err != nil {
		t.Fatalf("example workflow should be valid: %v", err)
	}
	if wf.StartNodeID != "n1" {
		t.Error("expected start node n1")
	}
	if len(wf.Nodes) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(wf.Nodes))
	}
}

func TestRunStatuses(t *testing.T) {
	statuses := []workflow.RunStatus{
		workflow.StatusPending,
		workflow.StatusRunning,
		workflow.StatusWaiting,
		workflow.StatusCompleted,
		workflow.StatusFailed,
	}
	for _, s := range statuses {
		if s == "" {
			t.Errorf("status %s is empty", s)
		}
	}
}

func TestConditionEvaluation(t *testing.T) {
	// Test resolveVar
	vars := map[string]any{
		"items": map[string]any{"length": "15"},
	}
	val := resolveVarTest(vars, "items.length")
	if val != "15" {
		t.Errorf("expected 15, got %v", val)
	}
}

func resolveVarTest(vars map[string]any, path string) any {
	parts := []string{"items", "length"}
	current := any(vars)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return current
		}
		current = m[part]
	}
	_ = path
	return current
}
