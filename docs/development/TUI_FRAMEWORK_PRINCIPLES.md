# TUI Framework Principles

This document establishes mandatory principles for TUI development in this codebase. All code in `cmd/` must adhere to these principles.

## The Five Principles

### 1. No Direct lipgloss.NewStyle() in Application Code

**Rule**: Application code (`cmd/`) must not use `lipgloss.NewStyle()` except for justified layout requirements.

**Why**: Custom styles fragment the design system, make themes inconsistent, and duplicate code.

**Allowed exceptions**:
- Layout-specific properties (`Width()`, `Padding()`) that aren't semantic
- Dynamic color parameters where the caller provides the color

**Enforcement**: Any `lipgloss.NewStyle()` in `cmd/` requires a comment explaining why it can't use a theme style.

### 2. Use Theme's Semantic Styles

**Rule**: All text styling must use the theme's pre-built semantic styles.

**Available styles**:
```go
th.Title        // Primary + Bold - for headers/titles
th.ErrorText    // Error + Bold - for error messages
th.SuccessText  // Success + Bold - for success messages
th.WarningText  // Warning + Bold - for warnings
th.MutedText    // Muted - for secondary/help text
th.PrimaryText  // Primary (no bold) - for emphasis
th.SecondaryText // Secondary - for less important text
th.LabelText    // Primary + Bold - for form labels
th.ValueText    // Accent - for form values
th.URLText      // Accent + Underline - for links
```

**Container styles**:
```go
th.ViewportBorder // Rounded border with Primary color
th.ViewportPlain  // No styling (for raw viewports)
```

**Why**: Semantic styles ensure consistency, support theming, and make the intent clear.

### 3. Use Framework Components

**Rule**: Use shared components instead of custom implementations.

**Available components**:

| Component | Package | Use Case |
|-----------|---------|----------|
| Header | `pkg/tui/components/header` | Rainbow logo with border |
| Spinner | `pkg/tui/components/spinner` | Animated loading indicator |
| Progress | `pkg/tui/components/progress` | Multi-step progress tracking |
| Steplist | `pkg/tui/components/steplist` | Step/check list rendering |

**Why**: Components encapsulate complex rendering logic, ensure visual consistency, and reduce duplication.

**Layout package**: Use `pkg/tui/layout` for standardized page layouts:

```go
// Option 1: Full Page model (for new TUIs)
page := layout.NewPage(
    layout.NewViewport(viewport.New(0, 0)),
    layout.WithHeader(func(w int) string { return header.Render(th) }),
    layout.WithFooter(func(w int) string { return help.View(keys) }),
)

// Option 2: ContentHeight helper (for existing TUIs)
case tea.WindowSizeMsg:
    headerStr := header.RenderWithContent(th, status)
    footerStr := help.View(keys)
    vpHeight := layout.ContentHeight(msg.Height, headerStr, footerStr)
    viewport.Height = vpHeight
```

**Never use magic numbers for heights** - always measure by rendering:
```go
// Bad: Magic numbers that don't match reality
headerHeight := 5 // WRONG - header is actually 7+ lines

// Good: Measure actual rendered height
vpHeight := layout.ContentHeight(windowHeight, headerStr, footerStr)
```

### 4. Use pkg/cli/format for Formatting

**Rule**: All time, duration, and ID formatting must use the format package.

**Available formatters**:
```go
// Time/Duration
format.FormatAge(t)           // "5m ago", "2h ago"
format.FormatAgeCompact(t)    // "5m", "2h"
format.FormatLastBeat(t)      // "now", "30s", "5m"
format.FormatDuration(d)      // "5s ago"
format.FormatDurationShort(d) // "5s"
format.FormatTimestamp(t)     // "15:04:05"

// IDs/Paths
format.FormatContainerID(id)  // Truncate to 12 chars
format.FormatSessionID(id)    // Truncate to 16 chars
format.FormatPath(path)       // Truncate from beginning

// JSON
format.HighlightJSON(s, colors) // Syntax highlighting
format.IsJSON(s)                // Validation
```

**Why**: Consistent formatting across all commands improves UX and reduces bugs.

### 5. lipgloss in App Code = Red Flag

**Rule**: Any `lipgloss.` usage in application code should trigger investigation.

**Investigation questions**:
1. Is this a missing theme style? → Add to theme
2. Is this a missing component? → Extract to components
3. Is this a missing formatter? → Add to format package
4. Is this justified layout code? → Add explanatory comment

**Why**: The framework should handle all styling. If app code needs lipgloss directly, the framework is incomplete.

## Compliance Checklist

Before merging code that touches TUI:

- [ ] No `lipgloss.NewStyle()` without justification comment
- [ ] All text uses semantic theme styles
- [ ] Reusable rendering uses components
- [ ] Time/ID formatting uses format package
- [ ] Any remaining lipgloss usage is documented

## Adding to the Framework

When you identify a pattern that should be extracted:

1. **New style**: Add to `pkg/tui/theme/theme.go` with semantic name
2. **New component**: Create in `pkg/tui/components/` with tests
3. **New formatter**: Add to `pkg/cli/format/format.go` with tests

## Current Exceptions

These uses of `lipgloss.NewStyle()` are justified:

| Location | Reason |
|----------|--------|
| `relay/model.go:268` | Dynamic tag colors from palette |
| `relay/model.go:331` | Width(12) layout requirement |
| `repl/repl.go:204,210` | Dynamic color parameter |
| `execute.go:285` | Padding layout requirement |

## Examples

### Bad: Custom Style in Command
```go
// DON'T DO THIS
errorStyle := lipgloss.NewStyle().Foreground(th.Error)
fmt.Println(errorStyle.Render("Error!"))
```

### Good: Theme Style
```go
// DO THIS
fmt.Println(th.ErrorText.Render("Error!"))
```

### Bad: Custom Duration Formatting
```go
// DON'T DO THIS
if d < time.Minute {
    return fmt.Sprintf("%ds", int(d.Seconds()))
}
```

### Good: Format Package
```go
// DO THIS
format.FormatDurationShort(d)
```

### Bad: Custom Step Rendering
```go
// DON'T DO THIS
icon := "✓"
style := th.SuccessText
fmt.Println(style.Render(icon + " " + step.Name))
```

### Good: Steplist Component
```go
// DO THIS
steplist.RenderItem(item, cfg, spinnerView)
```

### Bad: Magic Numbers for Layout
```go
// DON'T DO THIS - header renders off screen!
headerHeight := 5 // guessing wrong - actual height varies!
vpHeight := windowHeight - headerHeight - footerHeight
```

### Good: Measure by Rendering
```go
// DO THIS - measure actual rendered heights
headerStr := header.RenderWithContent(th, status)
footerStr := help.View(keys)
vpHeight := layout.ContentHeight(windowHeight, headerStr, footerStr)
```
