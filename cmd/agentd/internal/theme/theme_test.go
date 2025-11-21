package theme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRetroTheme_CGA(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)

	assert.Equal(t, PaletteCGA, theme.Palette)
	assert.NotNil(t, theme.Primary)
	assert.NotNil(t, theme.Secondary)
	assert.NotNil(t, theme.Accent)
	assert.NotNil(t, theme.Success)
	assert.NotNil(t, theme.Warning)
	assert.NotNil(t, theme.Error)
	assert.NotNil(t, theme.Muted)
}

func TestNewRetroTheme_Amber(t *testing.T) {
	theme := NewRetroTheme(PaletteAmber)
	assert.Equal(t, PaletteAmber, theme.Palette)
	assert.NotNil(t, theme.Primary)
}

func TestNewRetroTheme_Green(t *testing.T) {
	theme := NewRetroTheme(PaletteGreen)
	assert.Equal(t, PaletteGreen, theme.Palette)
	assert.NotNil(t, theme.Primary)
}

func TestNewRetroTheme_C64(t *testing.T) {
	theme := NewRetroTheme(PaletteC64)
	assert.Equal(t, PaletteC64, theme.Palette)
	assert.NotNil(t, theme.Primary)
}

func TestRetroTheme_Logo(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)
	logo := theme.Logo.Render("TEST")
	assert.NotEmpty(t, logo)
	assert.Contains(t, logo, "TEST")
}

func TestRetroTheme_Header(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)
	header := theme.Header.Render("HEADER")
	assert.NotEmpty(t, header)
	assert.Contains(t, header, "HEADER")
}

func TestRetroTheme_BoxBorder(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)
	box := theme.BoxBorder.Render("content")
	assert.NotEmpty(t, box)
	assert.Contains(t, box, "content")
}

func TestRetroTheme_StatusBar(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)
	status := theme.StatusBar.Render("status")
	assert.NotEmpty(t, status)
	assert.Contains(t, status, "status")
}

func TestRetroTheme_Highlight(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)
	highlight := theme.Highlight.Render("highlighted")
	assert.NotEmpty(t, highlight)
	assert.Contains(t, highlight, "highlighted")
}

func TestPaletteName_String(t *testing.T) {
	tests := []struct {
		palette  PaletteName
		expected string
	}{
		{PaletteCGA, "cga"},
		{PaletteAmber, "amber"},
		{PaletteGreen, "green"},
		{PaletteC64, "c64"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.palette.String())
		})
	}
}

func TestParsePaletteName(t *testing.T) {
	tests := []struct {
		input    string
		expected PaletteName
		valid    bool
	}{
		{"cga", PaletteCGA, true},
		{"amber", PaletteAmber, true},
		{"green", PaletteGreen, true},
		{"c64", PaletteC64, true},
		{"CGA", PaletteCGA, true}, // case insensitive
		{"AMBER", PaletteAmber, true},
		{"invalid", PaletteCGA, false}, // defaults to CGA
		{"", PaletteCGA, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, valid := ParsePaletteName(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.valid, valid)
		})
	}
}
