package discover

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/render"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
)

func TestNewModel(t *testing.T) {
	ctx := context.Background()
	th := theme.NewRetroTheme(theme.PaletteCGA)
	fetcher := func(ctx context.Context) ([]render.AgentInfo, error) {
		return []render.AgentInfo{}, nil
	}

	model := NewModel(ctx, th, fetcher)

	assert.NotNil(t, model.theme, "Expected theme to be set")
	assert.NotNil(t, model.spinner, "Expected spinner to be initialized")
	assert.True(t, model.loading, "Expected loading to be true initially")
	assert.False(t, model.quitting, "Expected quitting to be false initially")
	assert.NotNil(t, model.fetchAgents, "Expected fetcher function to be set")
}

func TestModel_Init(t *testing.T) {
	ctx := context.Background()
	th := theme.NewRetroTheme(theme.PaletteCGA)
	fetcher := func(ctx context.Context) ([]render.AgentInfo, error) {
		return []render.AgentInfo{}, nil
	}

	model := NewModel(ctx, th, fetcher)
	cmd := model.Init()

	assert.NotNil(t, cmd, "Expected Init to return a command")
}

func TestModel_QuitOnKeyPress(t *testing.T) {
	ctx := context.Background()
	th := theme.NewRetroTheme(theme.PaletteCGA)
	fetcher := func(ctx context.Context) ([]render.AgentInfo, error) {
		return []render.AgentInfo{}, nil
	}

	model := NewModel(ctx, th, fetcher)

	// Test 'q' key
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m := updatedModel.(Model)

	assert.True(t, m.quitting, "Expected quitting to be true after 'q' key")
	assert.NotNil(t, cmd, "Expected quit command to be returned")
}

func TestModel_RefreshOnKeyPress(t *testing.T) {
	ctx := context.Background()
	th := theme.NewRetroTheme(theme.PaletteCGA)
	fetcher := func(ctx context.Context) ([]render.AgentInfo, error) {
		return []render.AgentInfo{}, nil
	}

	model := NewModel(ctx, th, fetcher)
	model.loading = false // Start not loading

	// Test 'r' key
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m := updatedModel.(Model)

	assert.True(t, m.loading, "Expected loading to be true after 'r' key")
	assert.NotNil(t, cmd, "Expected fetch command to be returned")
}

func TestModel_HandleAgentsMsg(t *testing.T) {
	ctx := context.Background()
	th := theme.NewRetroTheme(theme.PaletteCGA)
	fetcher := func(ctx context.Context) ([]render.AgentInfo, error) {
		return []render.AgentInfo{}, nil
	}

	model := NewModel(ctx, th, fetcher)
	model.loading = true

	// Simulate successful agent fetch
	agents := []render.AgentInfo{
		{AgentID: "test-agent", Status: "running", CreatedAt: time.Now()},
	}
	msg := agentsMsg{agents: agents, err: nil}

	updatedModel, _ := model.Update(msg)
	m := updatedModel.(Model)

	assert.False(t, m.loading, "Expected loading to be false after agents received")
	assert.Len(t, m.agents, 1, "Expected 1 agent to be stored")
	assert.Equal(t, "test-agent", m.agents[0].AgentID, "Expected agent ID to match")
	assert.Nil(t, m.err, "Expected no error")
}

func TestModel_HandleAgentsMsgError(t *testing.T) {
	ctx := context.Background()
	th := theme.NewRetroTheme(theme.PaletteCGA)
	fetcher := func(ctx context.Context) ([]render.AgentInfo, error) {
		return nil, errors.New("fetch failed")
	}

	model := NewModel(ctx, th, fetcher)
	model.loading = true

	// Simulate error during fetch
	msg := agentsMsg{agents: nil, err: errors.New("fetch failed")}

	updatedModel, _ := model.Update(msg)
	m := updatedModel.(Model)

	assert.False(t, m.loading, "Expected loading to be false after error")
	assert.NotNil(t, m.err, "Expected error to be set")
	assert.Equal(t, "fetch failed", m.err.Error(), "Expected error message to match")
}

func TestModel_ViewWhenQuitting(t *testing.T) {
	ctx := context.Background()
	th := theme.NewRetroTheme(theme.PaletteCGA)
	fetcher := func(ctx context.Context) ([]render.AgentInfo, error) {
		return []render.AgentInfo{}, nil
	}

	model := NewModel(ctx, th, fetcher)
	model.quitting = true

	view := model.View()

	assert.Contains(t, view, "Stopped watching", "Expected quit message in view")
}

func TestModel_ViewWithError(t *testing.T) {
	ctx := context.Background()
	th := theme.NewRetroTheme(theme.PaletteCGA)
	fetcher := func(ctx context.Context) ([]render.AgentInfo, error) {
		return nil, errors.New("test error")
	}

	model := NewModel(ctx, th, fetcher)
	model.err = errors.New("test error")
	model.loading = false

	view := model.View()

	assert.Contains(t, view, "Error", "Expected error message in view")
	assert.Contains(t, view, "test error", "Expected specific error message in view")
}

func TestModel_ViewWhileLoading(t *testing.T) {
	ctx := context.Background()
	th := theme.NewRetroTheme(theme.PaletteCGA)
	fetcher := func(ctx context.Context) ([]render.AgentInfo, error) {
		return []render.AgentInfo{}, nil
	}

	model := NewModel(ctx, th, fetcher)
	model.loading = true

	view := model.View()

	assert.Contains(t, view, "Loading", "Expected loading message in view")
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"less than second", 500 * time.Millisecond, "0s"},
		{"1 second", 1 * time.Second, "1s"},
		{"30 seconds", 30 * time.Second, "30s"},
		{"1 minute", 1 * time.Minute, "1m"},
		{"5 minutes", 5 * time.Minute, "5m"},
		{"1 hour", 1 * time.Hour, "1h"},
		{"2 hours", 2 * time.Hour, "2h"},
		{"negative duration", -30 * time.Second, "30s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}
