package render

import (
	"bytes"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/stretchr/testify/assert"
)

func TestRenderAgentList_Plain(t *testing.T) {
	agents := []AgentInfo{
		{
			AgentID:     "test-agent",
			Status:      "running",
			SpawnSource: "cli",
			Workspace:   "/path/to/workspace",
			CreatedAt:   time.Now().Add(-1 * time.Hour),
		},
	}

	var buf bytes.Buffer
	err := RenderAgentList(&buf, agents, output.ModePlain, nil)

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "test-agent")
	assert.Contains(t, output, "running")
	assert.Contains(t, output, "cli")
}

func TestRenderAgentList_JSON(t *testing.T) {
	agents := []AgentInfo{
		{
			AgentID:     "test-agent",
			Status:      "running",
			SpawnSource: "cli",
			CreatedAt:   time.Now(),
		},
	}

	var buf bytes.Buffer
	err := RenderAgentList(&buf, agents, output.ModeJSON, nil)

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, `"AgentID"`)
	assert.Contains(t, output, `"test-agent"`)
	assert.Contains(t, output, `"Status"`)
	assert.Contains(t, output, `"running"`)
}

func TestRenderAgentList_Rich(t *testing.T) {
	agents := []AgentInfo{
		{
			AgentID:     "test-agent",
			Status:      "running",
			SpawnSource: "cli",
			CreatedAt:   time.Now(),
		},
	}

	th := theme.NewRetroTheme(theme.PaletteCGA)
	var buf bytes.Buffer
	err := RenderAgentList(&buf, agents, output.ModeRich, th)

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "test-agent")
	assert.Contains(t, output, "running")
	// Rich mode should have styled output (not plain text)
	assert.NotEmpty(t, output)
}

func TestRenderAgentList_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := RenderAgentList(&buf, []AgentInfo{}, output.ModePlain, nil)

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "No agents running")
}

func TestRenderAgentList_MultipleAgents(t *testing.T) {
	agents := []AgentInfo{
		{AgentID: "agent-1", Status: "running", SpawnSource: "cli", CreatedAt: time.Now()},
		{AgentID: "agent-2", Status: "paused", SpawnSource: "relay", CreatedAt: time.Now()},
		{AgentID: "agent-3", Status: "exited", SpawnSource: "cli", CreatedAt: time.Now()},
	}

	var buf bytes.Buffer
	err := RenderAgentList(&buf, agents, output.ModePlain, nil)

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "agent-1")
	assert.Contains(t, output, "agent-2")
	assert.Contains(t, output, "agent-3")
}
