package render

import (
	"bytes"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/cli"
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
	err := RenderAgentList(&buf, agents, cli.ModePlain, nil)

	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "test-agent")
	assert.Contains(t, out, "running")
	assert.Contains(t, out, "cli")
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
	err := RenderAgentList(&buf, agents, cli.ModeJSON, nil)

	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, `"AgentID"`)
	assert.Contains(t, out, `"test-agent"`)
	assert.Contains(t, out, `"Status"`)
	assert.Contains(t, out, `"running"`)
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

	th := theme.NewDark()
	var buf bytes.Buffer
	err := RenderAgentList(&buf, agents, cli.ModeRich, th)

	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "test-agent")
	assert.Contains(t, out, "running")
	// Rich mode should have styled output (not plain text)
	assert.NotEmpty(t, out)
}

func TestRenderAgentList_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := RenderAgentList(&buf, []AgentInfo{}, cli.ModePlain, nil)

	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "No agents running")
}

func TestRenderAgentList_MultipleAgents(t *testing.T) {
	agents := []AgentInfo{
		{AgentID: "agent-1", Status: "running", SpawnSource: "cli", CreatedAt: time.Now()},
		{AgentID: "agent-2", Status: "paused", SpawnSource: "relay", CreatedAt: time.Now()},
		{AgentID: "agent-3", Status: "exited", SpawnSource: "cli", CreatedAt: time.Now()},
	}

	var buf bytes.Buffer
	err := RenderAgentList(&buf, agents, cli.ModePlain, nil)

	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "agent-1")
	assert.Contains(t, out, "agent-2")
	assert.Contains(t, out, "agent-3")
}
