package header

import (
	"testing"

	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/stretchr/testify/assert"
)

func TestRender(t *testing.T) {
	th := theme.Default()
	result := Render(th)

	// Should not be empty
	assert.NotEmpty(t, result)

	// Should contain the border box
	assert.Contains(t, result, "╭")
	assert.Contains(t, result, "╯")
}

func TestRenderWithContent(t *testing.T) {
	th := theme.Default()
	result := RenderWithContent(th, "Test Status")

	// Should not be empty
	assert.NotEmpty(t, result)

	// Should contain both the logo box and the content
	assert.Contains(t, result, "╭")
	assert.Contains(t, result, "Test Status")
}

func TestRenderWithNilTheme(t *testing.T) {
	// Should not panic with nil theme
	result := Render(nil)
	assert.NotEmpty(t, result)
	// Should still render the logo box
	assert.Contains(t, result, "╭")
}
