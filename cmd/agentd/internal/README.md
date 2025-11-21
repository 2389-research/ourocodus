# agentd Internal Packages

Internal packages for the `agentd` CLI, providing foundation for Charm-based TUI features.

## Package Overview

### `detect/` - Terminal Detection

Detects terminal capabilities and determines appropriate output mode.

**Key Functions:**
- `IsTTY()` - Check if stdout is a terminal
- `ShouldUsePlainMode()` - Determine if plain mode should be used
- `GetTerminalSize()` - Get current terminal dimensions
- `IsTerminalTooSmall()` - Check if terminal meets minimum requirements (80x24)

**Environment Variables:**
- `NO_COLOR` - Disable colors
- `AGENTD_PLAIN` - Force plain mode
- `CI=true` - Auto-detect CI environment

### `theme/` - Retro Theme System

Provides retro computing aesthetics with multiple color palettes.

**Available Palettes:**
- **CGA** (default) - Bright cyan/magenta/yellow (#00FFFF, #FF00FF, #FFFF55)
- **Amber** - Monochrome amber terminal aesthetic
- **Green** - Green phosphor CRT vibes
- **C64** - Commodore 64 blue/purple palette

**Key Functions:**
- `NewRetroTheme(palette)` - Create theme with specified palette
- `GetLogo(size)` - Get Ourocodus ASCII art logo
- `GetAgentStatusIcon(status)` - Get emoji for agent status
- `DrawBox(title, content, width)` - Draw box with borders
- `DrawHeader(text)` - Draw header with double-line borders
- `GetVintageMessage(category)` - Get random vintage-style message

**Styles Available:**
- `Logo` - ASCII art banner styling
- `Header` - Command header styling
- `BoxBorder` - Bordered box styling
- `StatusBar` - Bottom status line styling
- `Highlight` - Selected/focused item styling

### `output/` - Output Mode Management

Manages output modes (rich/plain/json) for all commands.

**Modes:**
- **Rich** - Full TUI with Bubble Tea (default for TTY)
- **Plain** - Simple text with basic colors (CI/pipes)
- **JSON** - Machine-readable structured output

**Key Functions:**
- `DetectMode(jsonFlag, plainFlag, shouldUsePlain)` - Determine output mode
- `ParseMode(s)` - Parse mode string
- `Mode.IsRich()`, `Mode.IsPlain()`, `Mode.IsJSON()` - Mode checks

**Priority:**
1. `--json` flag (highest)
2. `--plain` flag
3. Environment detection (NO_COLOR, CI, etc.)
4. Auto-detect TTY
5. Terminal size check
6. Default to rich mode

## Usage Example

```go
import (
	"github.com/2389-research/ourocodus/cmd/agentd/internal/detect"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
)

// In your command's RunE function:
func runCommand(cmd *cobra.Command, args []string) error {
	// Step 1: Detect terminal capabilities
	shouldPlain := detect.ShouldUsePlainMode(jsonFlag, plainFlag, os.Environ)

	// Step 2: Determine output mode
	mode := output.DetectMode(jsonFlag, plainFlag, shouldPlain)

	// Step 3: Create theme (if rich mode)
	var th *theme.RetroTheme
	if mode.IsRich() {
		th = theme.NewRetroTheme(theme.PaletteCGA)
	}

	// Step 4: Render appropriate output
	if mode.IsRich() {
		// Use Bubble Tea TUI
		logo := th.Logo.Render(theme.GetLogo(theme.LogoSmall))
		fmt.Println(logo)
	} else if mode.IsPlain() {
		// Use simple text output
		fmt.Println("📋 Plain output")
	} else if mode.IsJSON() {
		// Use JSON output
		json.NewEncoder(os.Stdout).Encode(data)
	}

	return nil
}
```

## Testing

All packages have comprehensive test coverage:

```bash
# Test individual packages
go test ./cmd/agentd/internal/detect -v
go test ./cmd/agentd/internal/theme -v
go test ./cmd/agentd/internal/output -v

# Test integration
go test ./cmd/agentd/internal -v

# Run all tests
make test
```

## Design Decisions

1. **Terminal Detection First** - Always check capabilities before rendering
2. **Mode Priority** - Explicit flags > environment > auto-detect
3. **Backward Compatible** - Plain mode maintains exact current output format
4. **Theme Separation** - Colors, styles, and ASCII art in dedicated package
5. **TUI-Ready** - Foundation ready for Bubble Tea integration in Phase 2
