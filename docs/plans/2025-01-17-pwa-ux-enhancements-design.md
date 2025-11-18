# PWA UX Enhancements Design

**Date:** 2025-01-17
**Issues:** #66 (Theme toggle), #62 (Inspector search/export), #61 (Retry button), #60 (Agent spawn progress)
**Milestone:** 9 (PWA Polish & Features)

## Overview

This design implements four PWA user experience enhancements that collectively improve the polish and usability of the web interface. All features are additive and non-breaking, shipped together as a cohesive UX improvement milestone.

## Problem Statement

### Theme Toggle (#66)
The PWA currently only supports a dark theme. Users need light theme support for:
- Better visibility in bright environments
- Personal preference and accessibility needs
- Matching system preferences automatically

### Inspector Search/Export (#62)
The Protocol Inspector shows all messages chronologically with no way to:
- Search for specific messages or patterns
- Export message history for debugging
- Clear message history

### Retry Button (#61)
When recoverable errors occur, users can only dismiss notifications and must manually retry operations, creating friction during transient failures.

### Agent Spawn Progress (#60)
Agent spawning provides no visual feedback during the operation, leaving users uncertain about progress or whether the system is working.

## Architecture

### Modular Services Pattern

Create four self-contained services under `internal/webapp/src/services/`:

1. **theme-service.ts** - Theme management and persistence
2. **inspector-service.ts** - Search, export, and clear functionality
3. **notification-service.ts** - Enhanced notifications with retry
4. **loading-service.ts** - Progress indicators for long operations

**Design principles:**
- Each service is self-contained with its own state and DOM handling
- No dependencies between services (except theme propagation)
- Services expose simple public APIs
- App class in `state.ts` composes and coordinates services

## Detailed Design

### 1. Theme Service

**File:** `internal/webapp/src/services/theme-service.ts`

**Responsibilities:**
- Manage theme state (`'dark' | 'light'`)
- Persist preference to localStorage
- Detect system preference on first visit
- Apply theme via data attributes
- Propagate theme to inspector iframe

**API:**
```typescript
class ThemeService {
  constructor()
  toggle(): void
  getCurrentTheme(): 'dark' | 'light'
}
```

**Theme Loading Priority:**
1. Check `localStorage.getItem('ourocodus.theme')`
2. Fall back to `window.matchMedia('(prefers-color-scheme: dark)')`
3. Default to 'dark' if neither available

**CSS Implementation:**
Use `[data-theme="dark"]` and `[data-theme="light"]` attribute selectors:

```css
[data-theme="dark"] {
  --bg-primary: #0a0a0f;
  --bg-secondary: #13131a;
  --text-primary: #f5f5f7;
  --accent-primary: #7c3aed;
  --accent-secondary: #a78bfa;
  --border-color: #27272f;
  --error-color: #ef4444;
  --success-color: #10b981;
  --warning-color: #f59e0b;
}

[data-theme="light"] {
  --bg-primary: #ffffff;
  --bg-secondary: #f5f5f7;
  --text-primary: #1a1a1a;
  --accent-primary: #6d28d9;
  --accent-secondary: #8b5cf6;
  --border-color: #e5e5e5;
  --error-color: #dc2626;
  --success-color: #059669;
  --warning-color: #d97706;
}
```

Replace all hardcoded colors in `styles.css` with `var(--color-name)` references.

**Smooth Transitions:**
Add to major elements:
```css
body, .card, .modal-content {
  transition: background-color 0.3s ease, color 0.3s ease;
}
```

**UI Component:**
Theme toggle button in header:
```html
<button id="themeToggle" class="btn btn-small" title="Toggle theme">
  <span id="themeIcon">🌙</span>
</button>
```

Icons: ☀️ for light theme (shows "switch to dark"), 🌙 for dark theme (shows "switch to light")

**Inspector Sync:**
When theme changes, send postMessage:
```typescript
const inspectorFrame = document.querySelector('iframe');
if (inspectorFrame?.contentWindow) {
  inspectorFrame.contentWindow.postMessage({
    type: 'theme:change',
    theme: this.currentTheme
  }, '*');
}
```

Inspector listens and applies theme without persisting its own preference.

### 2. Inspector Service

**File:** `internal/webapp/src/services/inspector-service.ts`

**Responsibilities:**
- Filter messages by search query
- Export message history as JSON
- Clear message history with confirmation
- Listen for theme changes

**API:**
```typescript
class InspectorService {
  constructor()
  search(query: string): void
  exportJSON(): void
  clear(): void
  onThemeChange(theme: 'dark' | 'light'): void
}
```

**UI Components (added to inspector panel):**
```html
<div class="inspector-controls">
  <input type="text" id="inspectorSearch" placeholder="Search messages...">
  <button id="inspectorExport" class="btn btn-small">Export JSON</button>
  <button id="inspectorClear" class="btn btn-small btn-danger">Clear</button>
</div>
```

**Search Implementation:**
- Case-insensitive string matching: `message.toLowerCase().includes(query.toLowerCase())`
- Filter on every keystroke (debounced 150ms)
- Show/hide message DOM elements based on match
- Search in message type and payload content

**Export Implementation:**
```typescript
exportJSON(): void {
  const data = this.messages.map(msg => ({
    timestamp: msg.timestamp,
    direction: msg.direction, // 'recv' or 'send'
    type: msg.type,
    payload: msg.payload
  }));

  const json = JSON.stringify(data, null, 2);
  const blob = new Blob([json], { type: 'application/json' });
  const url = URL.createObjectURL(blob);

  const a = document.createElement('a');
  a.href = url;
  a.download = `ourocodus-messages-${Date.now()}.json`;
  a.click();

  URL.revokeObjectURL(url);
}
```

**Clear Implementation:**
- Show confirmation modal: "Clear all messages?"
- On confirm: Clear internal message array and remove DOM elements
- Keep session info panel intact
- Reset search input

### 3. Notification Service

**File:** `internal/webapp/src/services/notification-service.ts`

**Responsibilities:**
- Display error notifications with optional retry
- Track operation context for retries
- Show loading state during retry

**API:**
```typescript
class NotificationService {
  constructor()
  showError(message: string, options?: {
    recoverable?: boolean,
    retryCallback?: () => void | Promise<void>
  }): void
  dismiss(notificationId: string): void
}
```

**Enhanced Notification HTML:**
```html
<div class="notification error" id="notification-{id}">
  <div class="notification-content">
    <span class="notification-message">{message}</span>
  </div>
  <div class="notification-actions">
    <button class="btn-retry">Retry</button>
    <button class="btn-dismiss">Dismiss</button>
  </div>
</div>
```

Retry button only shown when `retryCallback` provided.

**Retry Flow:**
1. User clicks "Retry" button
2. Button text changes to "Retrying..." with disabled state
3. Execute `retryCallback()`
4. On success: Dismiss notification
5. On failure: Re-enable button, show new error

**Integration Points:**
RelayConnection tracks operation context:
```typescript
// Agent spawn
this.lastSpawnRequest = { role, workspace };

// On spawn error
notificationService.showError('Failed to spawn agent', {
  recoverable: true,
  retryCallback: () => this.spawnAgent(
    this.lastSpawnRequest.role,
    this.lastSpawnRequest.workspace
  )
});
```

### 4. Loading Service

**File:** `internal/webapp/src/services/loading-service.ts`

**Responsibilities:**
- Show progress spinner on target element
- Display status messages
- Update messages during operation
- Hide on completion/error

**API:**
```typescript
class LoadingService {
  constructor()
  show(target: HTMLElement, message: string): void
  update(message: string): void
  hide(): void
}
```

**Loading Overlay HTML:**
```html
<div class="loading-overlay">
  <div class="loading-spinner"></div>
  <div class="loading-message">{message}</div>
</div>
```

**Agent Spawn Integration:**
Status messages during spawn:
1. "Initializing workspace..." (immediately)
2. "Starting ACP client..." (after workspace ready)
3. "Waiting for ready signal..." (after ACP started)
4. "Still working... (agent spawn can take up to 30s)" (after 10s timeout warning)

**CSS:**
```css
.loading-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--bg-primary);
  opacity: 0.95;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  z-index: 10;
}

.loading-spinner {
  width: 40px;
  height: 40px;
  border: 4px solid var(--border-color);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
```

## Implementation Strategy

### File Organization

**New Files:**
```
internal/webapp/src/services/
├── theme-service.ts       (~150 lines)
├── inspector-service.ts   (~200 lines)
├── notification-service.ts (~100 lines)
└── loading-service.ts     (~80 lines)
```

**Modified Files:**
- `internal/webapp/src/ui/state.ts` - Import and compose services
- `internal/webapp/web/styles.css` - Add CSS variables and theme selectors
- `internal/webapp/web/index.html` - Add theme toggle and inspector controls

### Implementation Order

1. **Theme Service** - Foundation for all visual changes, most independent
2. **Loading Service** - Simple, provides immediate UX improvement
3. **Notification Service** - Extends existing system, moderate complexity
4. **Inspector Service** - Most complex, builds on stable foundation

### App Integration

**state.ts changes:**
```typescript
import { ThemeService } from '../services/theme-service';
import { LoadingService } from '../services/loading-service';
import { NotificationService } from '../services/notification-service';
import { InspectorService } from '../services/inspector-service';

export class App {
  private theme: ThemeService;
  private loading: LoadingService;
  private notifications: NotificationService;
  private inspector: InspectorService;

  constructor() {
    this.theme = new ThemeService();
    this.loading = new LoadingService();
    this.notifications = new NotificationService();
    this.inspector = new InspectorService();

    this.setupEventListeners();
  }

  private setupEventListeners(): void {
    // Theme toggle
    const themeBtn = document.getElementById('themeToggle');
    themeBtn?.addEventListener('click', () => this.theme.toggle());

    // Agent spawn with loading
    const spawnBtn = document.getElementById('spawnAgentBtn');
    spawnBtn?.addEventListener('click', () => {
      const section = document.getElementById('agentSpawnSection');
      this.loading.show(section, 'Initializing workspace...');
      this.connection.spawnAgent(role, workspace);
    });
  }
}
```

## Testing Strategy

### Manual Testing Checklist

**Theme Service:**
- [ ] Toggle switches between light and dark themes
- [ ] Theme persists after page reload
- [ ] System preference detected on first visit
- [ ] Inspector iframe theme syncs with main PWA
- [ ] Smooth transitions (no flash)
- [ ] All UI elements readable in both themes

**Inspector Service:**
- [ ] Search filters messages correctly
- [ ] Search is case-insensitive
- [ ] Export downloads valid JSON file
- [ ] Clear shows confirmation modal
- [ ] Clear removes messages but keeps session info
- [ ] Search works after adding new messages

**Notification Service:**
- [ ] Retry button appears for recoverable errors
- [ ] Retry executes callback correctly
- [ ] Button shows "Retrying..." state
- [ ] Success dismisses notification
- [ ] Failure shows new error with retry option
- [ ] Non-recoverable errors show no retry button

**Loading Service:**
- [ ] Spinner appears during agent spawn
- [ ] Status messages update correctly
- [ ] Timeout warning shows after 10s
- [ ] Overlay hides on success
- [ ] Overlay hides on error
- [ ] Multiple operations don't conflict

### Accessibility Testing

- [ ] All buttons have proper aria-labels
- [ ] Theme toggle keyboard accessible (Enter/Space)
- [ ] Loading overlay announces status to screen readers
- [ ] Color contrast meets WCAG AA in both themes
- [ ] Focus indicators visible in both themes

## Risk Assessment

**Risk Level:** Low

**Mitigations:**
- All features are additive, no breaking changes
- Existing functionality unchanged
- Theme defaults to dark (current behavior)
- New UI elements are opt-in
- Services are independent (failure isolated)
- Comprehensive manual testing before merge

## Success Criteria

1. Users can toggle between dark and light themes with persistence
2. Inspector iframe theme syncs with main PWA automatically
3. Protocol Inspector search filters messages correctly
4. Export produces valid JSON with all message data
5. Retry button appears for recoverable errors and executes correctly
6. Agent spawn shows progress indicator with status messages
7. All features work together without conflicts
8. No regressions in existing functionality

## Future Enhancements

- Add keyboard shortcuts for theme toggle (Ctrl+Shift+T)
- Add more export formats (CSV, text log)
- Add regex search support to inspector
- Add time range filtering to inspector
- Add progress bars for file operations
- Add toast notifications for success states
