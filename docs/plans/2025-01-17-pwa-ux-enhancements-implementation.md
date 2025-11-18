# PWA UX Enhancements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement theme toggle, inspector search/export, retry notifications, and agent spawn progress indicators for PWA.

**Architecture:** Modular services pattern with four independent services (theme, inspector, notification, loading) composed by App class. Each service owns its DOM and state.

**Tech Stack:** TypeScript, CSS custom properties, DOM APIs, localStorage, postMessage for iframe communication

---

## Task 1: Create Theme Service Foundation

**Files:**
- Create: `internal/webapp/src/services/theme-service.ts`
- Test: Manual testing (no TS tests in current codebase)

**Step 1: Create theme service file**

Create `internal/webapp/src/services/theme-service.ts`:

```typescript
/**
 * Theme Service - Manages dark/light theme switching with persistence
 */

export type Theme = 'dark' | 'light';

export class ThemeService {
    private currentTheme: Theme;

    constructor() {
        this.currentTheme = this.loadTheme();
        this.apply();
    }

    /**
     * Load theme preference from localStorage or system preference
     */
    private loadTheme(): Theme {
        // Check localStorage first
        const stored = localStorage.getItem('ourocodus.theme');
        if (stored === 'dark' || stored === 'light') {
            return stored;
        }

        // Fall back to system preference
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        return prefersDark ? 'dark' : 'light';
    }

    /**
     * Apply theme to document
     */
    private apply(): void {
        document.documentElement.setAttribute('data-theme', this.currentTheme);
        this.updateIcon();
        this.notifyInspector();
    }

    /**
     * Save theme to localStorage
     */
    private save(): void {
        localStorage.setItem('ourocodus.theme', this.currentTheme);
    }

    /**
     * Update theme toggle button icon
     */
    private updateIcon(): void {
        const icon = document.getElementById('themeIcon');
        if (icon) {
            // Show sun for light theme (switch to dark)
            // Show moon for dark theme (switch to light)
            icon.textContent = this.currentTheme === 'dark' ? '🌙' : '☀️';
        }
    }

    /**
     * Notify inspector iframe of theme change
     */
    private notifyInspector(): void {
        const inspectorFrame = document.querySelector('iframe');
        if (inspectorFrame?.contentWindow) {
            inspectorFrame.contentWindow.postMessage({
                type: 'theme:change',
                theme: this.currentTheme
            }, '*');
        }
    }

    /**
     * Toggle between dark and light themes
     */
    public toggle(): void {
        this.currentTheme = this.currentTheme === 'dark' ? 'light' : 'dark';
        this.apply();
        this.save();
    }

    /**
     * Get current theme
     */
    public getCurrentTheme(): Theme {
        return this.currentTheme;
    }
}
```

**Step 2: Build TypeScript to verify syntax**

Run: `cd internal/webapp && mise run build:ts`
Expected: Build succeeds with no errors

**Step 3: Commit theme service**

```bash
git add internal/webapp/src/services/theme-service.ts
git commit -m "feat: add theme service for dark/light mode switching"
```

---

## Task 2: Add CSS Theme Variables

**Files:**
- Modify: `internal/webapp/web/styles.css` (add theme variables at top)

**Step 1: Add CSS custom properties for both themes**

Add to top of `internal/webapp/web/styles.css` (after any existing :root):

```css
/* Theme Variables */
[data-theme="dark"] {
  --bg-primary: #0a0a0f;
  --bg-secondary: #13131a;
  --bg-tertiary: #1a1a24;
  --text-primary: #f5f5f7;
  --text-secondary: #a1a1aa;
  --text-muted: #71717a;
  --accent-primary: #7c3aed;
  --accent-secondary: #a78bfa;
  --accent-hover: #6d28d9;
  --border-color: #27272f;
  --error-color: #ef4444;
  --success-color: #10b981;
  --warning-color: #f59e0b;
  --info-color: #3b82f6;
}

[data-theme="light"] {
  --bg-primary: #ffffff;
  --bg-secondary: #f5f5f7;
  --bg-tertiary: #e5e5e5;
  --text-primary: #1a1a1a;
  --text-secondary: #525252;
  --text-muted: #737373;
  --accent-primary: #6d28d9;
  --accent-secondary: #8b5cf6;
  --accent-hover: #5b21b6;
  --border-color: #d4d4d4;
  --error-color: #dc2626;
  --success-color: #059669;
  --warning-color: #d97706;
  --info-color: #2563eb;
}

/* Smooth theme transitions */
body,
.container,
.card,
.modal-content,
.notification,
.btn {
  transition: background-color 0.3s ease, color 0.3s ease, border-color 0.3s ease;
}
```

**Step 2: Replace hardcoded colors with CSS variables**

Find all color values in `styles.css` and replace with variables. Examples:

```css
/* Before */
body {
  background: #0a0a0f;
  color: #f5f5f7;
}

/* After */
body {
  background: var(--bg-primary);
  color: var(--text-primary);
}

/* Before */
.card {
  background: #13131a;
  border: 1px solid #27272f;
}

/* After */
.card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
}

/* Before */
.btn-primary {
  background: #7c3aed;
}

/* After */
.btn-primary {
  background: var(--accent-primary);
}
```

Replace all instances throughout the file systematically.

**Step 3: Build assets to verify**

Run: `make build`
Expected: Build succeeds, CSS compiles

**Step 4: Commit CSS theme variables**

```bash
git add internal/webapp/web/styles.css
git commit -m "feat: add CSS theme variables and replace hardcoded colors"
```

---

## Task 3: Add Theme Toggle Button to UI

**Files:**
- Modify: `internal/webapp/web/index.html` (add button to header)
- Modify: `internal/webapp/src/ui/state.ts` (wire theme service)

**Step 1: Add theme toggle button to HTML**

In `internal/webapp/web/index.html`, find the header section (around line 18-30) and add theme button:

```html
<header class="header">
    <h1 class="title">Ourocodus</h1>
    <div class="connection-controls">
        <button id="themeToggle" class="btn btn-small" title="Toggle theme">
            <span id="themeIcon">🌙</span>
        </button>
        <div class="connection-status" id="connectionStatus">
            <span class="status-indicator" id="statusIndicator"></span>
            <span class="status-text" id="statusText">Disconnected</span>
        </div>
        <button class="btn btn-danger btn-small" id="disconnectBtn" style="display: none;" title="Close WebSocket connection only. Server will cleanup session automatically. UI state remains visible.">
            <span class="btn-icon">⏻</span>
            Disconnect
        </button>
    </div>
</header>
```

**Step 2: Import and initialize theme service in state.ts**

At top of `internal/webapp/src/ui/state.ts`, add import:

```typescript
import { ThemeService } from '../services/theme-service';
```

In the `App` class, add private field:

```typescript
export class App {
    private logger: Logger;
    private connection: RelayConnection;
    private theme: ThemeService;
    // ... existing fields
```

In `App.init()` method, initialize theme service and wire button:

```typescript
public init(): void {
    this.logger.info('Initializing application');

    // Initialize theme service
    this.theme = new ThemeService();

    // Wire theme toggle button
    const themeToggleBtn = document.getElementById('themeToggle');
    if (themeToggleBtn) {
        themeToggleBtn.addEventListener('click', () => {
            this.logger.debug('Theme toggle clicked');
            this.theme.toggle();
        });
    }

    // ... rest of existing init code
}
```

**Step 3: Build and test theme switching**

Run: `make build`
Expected: Build succeeds

Manual test:
1. Open `internal/webapp/web/index.html` in browser
2. Click theme toggle button
3. Verify theme switches between dark and light
4. Reload page, verify theme persists
5. Check browser console for localStorage: `localStorage.getItem('ourocodus.theme')`

**Step 4: Commit theme UI integration**

```bash
git add internal/webapp/web/index.html internal/webapp/src/ui/state.ts
git commit -m "feat: add theme toggle button and wire to theme service"
```

---

## Task 4: Create Loading Service

**Files:**
- Create: `internal/webapp/src/services/loading-service.ts`

**Step 1: Create loading service file**

Create `internal/webapp/src/services/loading-service.ts`:

```typescript
/**
 * Loading Service - Shows progress indicators for long operations
 */

export class LoadingService {
    private overlay: HTMLElement | null = null;
    private messageEl: HTMLElement | null = null;

    /**
     * Show loading overlay on target element
     */
    public show(target: HTMLElement, message: string): void {
        // Remove existing overlay if any
        this.hide();

        // Create overlay
        this.overlay = document.createElement('div');
        this.overlay.className = 'loading-overlay';

        // Create spinner
        const spinner = document.createElement('div');
        spinner.className = 'loading-spinner';

        // Create message
        this.messageEl = document.createElement('div');
        this.messageEl.className = 'loading-message';
        this.messageEl.textContent = message;

        // Assemble and append
        this.overlay.appendChild(spinner);
        this.overlay.appendChild(this.messageEl);

        // Position relative to target
        target.style.position = 'relative';
        target.appendChild(this.overlay);
    }

    /**
     * Update loading message
     */
    public update(message: string): void {
        if (this.messageEl) {
            this.messageEl.textContent = message;
        }
    }

    /**
     * Hide and remove loading overlay
     */
    public hide(): void {
        if (this.overlay) {
            this.overlay.remove();
            this.overlay = null;
            this.messageEl = null;
        }
    }
}
```

**Step 2: Add loading overlay CSS**

Add to `internal/webapp/web/styles.css`:

```css
/* Loading Overlay */
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
  gap: 1rem;
  z-index: 10;
  border-radius: 8px;
}

.loading-spinner {
  width: 40px;
  height: 40px;
  border: 4px solid var(--border-color);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.loading-message {
  color: var(--text-secondary);
  font-size: 0.875rem;
  text-align: center;
  max-width: 80%;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
```

**Step 3: Build to verify**

Run: `make build`
Expected: Build succeeds

**Step 4: Commit loading service**

```bash
git add internal/webapp/src/services/loading-service.ts internal/webapp/web/styles.css
git commit -m "feat: add loading service for progress indicators"
```

---

## Task 5: Integrate Loading Service with Agent Spawn

**Files:**
- Modify: `internal/webapp/src/ui/state.ts` (add loading to spawn)

**Step 1: Import loading service**

At top of `internal/webapp/src/ui/state.ts`, add:

```typescript
import { LoadingService } from '../services/loading-service';
```

**Step 2: Add loading service field to App class**

```typescript
export class App {
    private logger: Logger;
    private connection: RelayConnection;
    private theme: ThemeService;
    private loading: LoadingService;
    // ... existing fields
```

**Step 3: Initialize loading service in App.init()**

```typescript
public init(): void {
    this.logger.info('Initializing application');

    // Initialize services
    this.theme = new ThemeService();
    this.loading = new LoadingService();

    // ... rest of init
}
```

**Step 4: Show loading on agent spawn**

Find the agent spawn button event listener in `state.ts` (look for `spawnAgentBtn`) and wrap spawn call:

```typescript
const spawnAgentBtn = document.getElementById('spawnAgentBtn');
if (spawnAgentBtn) {
    spawnAgentBtn.addEventListener('click', () => {
        this.logger.debug('Spawn Agent button clicked');

        const roleInput = document.getElementById('agentRole') as HTMLInputElement;
        const workspaceInput = document.getElementById('agentWorkspace') as HTMLInputElement;

        const role = roleInput?.value || 'echo';
        const workspace = workspaceInput?.value || './workspaces/default';

        // Show loading indicator
        const spawnSection = document.getElementById('agentSpawnSection');
        if (spawnSection) {
            this.loading.show(spawnSection, 'Initializing workspace...');
        }

        this.connection.spawnAgent(role, workspace);
    });
}
```

**Step 5: Hide loading on agent ready/error**

Find where agent ready/error events are handled and add `this.loading.hide()`:

```typescript
// In agent:ready handler
private handleAgentReady(agentId: string): void {
    this.loading.hide();
    // ... existing handler code
}

// In agent spawn error handler
private handleSpawnError(error: any): void {
    this.loading.hide();
    // ... existing error handling
}
```

**Step 6: Build and test**

Run: `make build`
Expected: Build succeeds

Manual test:
1. Start relay server
2. Open PWA
3. Click "Spawn Agent"
4. Verify loading spinner appears
5. Verify loading hides when agent ready

**Step 7: Commit loading integration**

```bash
git add internal/webapp/src/ui/state.ts
git commit -m "feat: integrate loading service with agent spawn"
```

---

## Task 6: Create Notification Service with Retry

**Files:**
- Create: `internal/webapp/src/services/notification-service.ts`

**Step 1: Create notification service file**

Create `internal/webapp/src/services/notification-service.ts`:

```typescript
/**
 * Notification Service - Enhanced notifications with retry capability
 */

interface NotificationOptions {
    recoverable?: boolean;
    retryCallback?: () => void | Promise<void>;
}

export class NotificationService {
    private container: HTMLElement | null = null;
    private notificationId = 0;

    constructor() {
        this.ensureContainer();
    }

    /**
     * Ensure notification container exists
     */
    private ensureContainer(): void {
        this.container = document.getElementById('notificationContainer');
        if (!this.container) {
            this.container = document.createElement('div');
            this.container.id = 'notificationContainer';
            this.container.className = 'notification-container';
            document.body.appendChild(this.container);
        }
    }

    /**
     * Show error notification with optional retry
     */
    public showError(message: string, options?: NotificationOptions): void {
        const id = `notification-${this.notificationId++}`;

        const notification = document.createElement('div');
        notification.className = 'notification error';
        notification.id = id;

        const content = document.createElement('div');
        content.className = 'notification-content';

        const messageEl = document.createElement('span');
        messageEl.className = 'notification-message';
        messageEl.textContent = message;
        content.appendChild(messageEl);

        const actions = document.createElement('div');
        actions.className = 'notification-actions';

        // Add retry button if recoverable
        if (options?.recoverable && options?.retryCallback) {
            const retryBtn = document.createElement('button');
            retryBtn.className = 'btn btn-small btn-retry';
            retryBtn.textContent = 'Retry';
            retryBtn.addEventListener('click', async () => {
                retryBtn.disabled = true;
                retryBtn.textContent = 'Retrying...';

                try {
                    await options.retryCallback?.();
                    this.dismiss(id);
                } catch (error) {
                    retryBtn.disabled = false;
                    retryBtn.textContent = 'Retry';
                    // Error will trigger new notification
                }
            });
            actions.appendChild(retryBtn);
        }

        // Add dismiss button
        const dismissBtn = document.createElement('button');
        dismissBtn.className = 'btn btn-small btn-dismiss';
        dismissBtn.textContent = 'Dismiss';
        dismissBtn.addEventListener('click', () => this.dismiss(id));
        actions.appendChild(dismissBtn);

        notification.appendChild(content);
        notification.appendChild(actions);

        this.container?.appendChild(notification);

        // Auto-dismiss after 10 seconds if not recoverable
        if (!options?.recoverable) {
            setTimeout(() => this.dismiss(id), 10000);
        }
    }

    /**
     * Dismiss notification by ID
     */
    public dismiss(notificationId: string): void {
        const notification = document.getElementById(notificationId);
        if (notification) {
            notification.style.opacity = '0';
            setTimeout(() => notification.remove(), 300);
        }
    }
}
```

**Step 2: Add notification CSS**

Add to `internal/webapp/web/styles.css`:

```css
/* Notifications */
.notification-container {
  position: fixed;
  top: 20px;
  right: 20px;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-width: 400px;
}

.notification {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 1rem;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  opacity: 1;
  transition: opacity 0.3s ease;
}

.notification.error {
  border-left: 4px solid var(--error-color);
}

.notification-content {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
}

.notification-message {
  color: var(--text-primary);
  font-size: 0.875rem;
  line-height: 1.4;
}

.notification-actions {
  display: flex;
  gap: 0.5rem;
  justify-content: flex-end;
}

.btn-retry {
  background: var(--accent-primary);
  color: white;
}

.btn-retry:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-dismiss {
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}
```

**Step 3: Build to verify**

Run: `make build`
Expected: Build succeeds

**Step 4: Commit notification service**

```bash
git add internal/webapp/src/services/notification-service.ts internal/webapp/web/styles.css
git commit -m "feat: add notification service with retry capability"
```

---

## Task 7: Integrate Notification Service with Error Handling

**Files:**
- Modify: `internal/webapp/src/ui/state.ts` (replace error handling)
- Modify: `internal/webapp/src/connection.ts` (track operation context)

**Step 1: Import notification service in state.ts**

At top of `internal/webapp/src/ui/state.ts`:

```typescript
import { NotificationService } from '../services/notification-service';
```

**Step 2: Add notification service to App class**

```typescript
export class App {
    private logger: Logger;
    private connection: RelayConnection;
    private theme: ThemeService;
    private loading: LoadingService;
    private notifications: NotificationService;
    // ... existing fields
```

**Step 3: Initialize notifications in App.init()**

```typescript
public init(): void {
    this.logger.info('Initializing application');

    // Initialize services
    this.theme = new ThemeService();
    this.loading = new LoadingService();
    this.notifications = new NotificationService();

    // ... rest of init
}
```

**Step 4: Track last spawn request in connection.ts**

In `internal/webapp/src/connection.ts`, add field to RelayConnection class:

```typescript
export class RelayConnection {
    private ws: WebSocket | null = null;
    private logger: Logger;
    private lastSpawnRequest: { role: string; workspace: string } | null = null;
    // ... existing fields
```

In `spawnAgent()` method, track the request:

```typescript
public spawnAgent(role: string, workspace: string): void {
    this.lastSpawnRequest = { role, workspace };

    const message = {
        type: 'agent:spawn',
        role,
        workspace
    };

    this.send(message);
}
```

**Step 5: Replace error alerts with notifications**

Find error handling in `state.ts` and replace `alert()` calls with notifications:

```typescript
// Example: Replace spawn error alert
private handleSpawnError(error: any): void {
    this.loading.hide();

    this.notifications.showError(`Failed to spawn agent: ${error.message}`, {
        recoverable: true,
        retryCallback: () => {
            if (this.connection.lastSpawnRequest) {
                const { role, workspace } = this.connection.lastSpawnRequest;
                const spawnSection = document.getElementById('agentSpawnSection');
                if (spawnSection) {
                    this.loading.show(spawnSection, 'Retrying...');
                }
                this.connection.spawnAgent(role, workspace);
            }
        }
    });
}
```

**Step 6: Build and test**

Run: `make build`
Expected: Build succeeds

Manual test:
1. Cause agent spawn failure (invalid workspace)
2. Verify notification appears with "Retry" button
3. Click "Retry"
4. Verify retry executes

**Step 7: Commit notification integration**

```bash
git add internal/webapp/src/ui/state.ts internal/webapp/src/connection.ts
git commit -m "feat: integrate notification service with error handling and retry"
```

---

## Task 8: Create Inspector Service Foundation

**Files:**
- Create: `internal/webapp/src/services/inspector-service.ts`

**Step 1: Create inspector service file**

Create `internal/webapp/src/services/inspector-service.ts`:

```typescript
/**
 * Inspector Service - Protocol Inspector with search and export
 */

interface Message {
    timestamp: number;
    direction: 'recv' | 'send';
    type: string;
    payload: any;
}

export class InspectorService {
    private messages: Message[] = [];
    private searchQuery: string = '';

    /**
     * Add message to history
     */
    public addMessage(direction: 'recv' | 'send', type: string, payload: any): void {
        this.messages.push({
            timestamp: Date.now(),
            direction,
            type,
            payload
        });
    }

    /**
     * Search messages by query
     */
    public search(query: string): void {
        this.searchQuery = query.toLowerCase();
        this.filterMessages();
    }

    /**
     * Filter visible messages based on search query
     */
    private filterMessages(): void {
        const messageElements = document.querySelectorAll('.inspector-message');

        messageElements.forEach((el, index) => {
            if (!this.searchQuery) {
                (el as HTMLElement).style.display = '';
                return;
            }

            const message = this.messages[index];
            if (!message) return;

            const matchesType = message.type.toLowerCase().includes(this.searchQuery);
            const matchesPayload = JSON.stringify(message.payload).toLowerCase().includes(this.searchQuery);

            (el as HTMLElement).style.display = (matchesType || matchesPayload) ? '' : 'none';
        });
    }

    /**
     * Export messages as JSON
     */
    public exportJSON(): void {
        const data = this.messages.map(msg => ({
            timestamp: new Date(msg.timestamp).toISOString(),
            direction: msg.direction,
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

    /**
     * Clear all messages
     */
    public clear(): void {
        if (!confirm('Clear all protocol messages?')) {
            return;
        }

        this.messages = [];
        this.searchQuery = '';

        const container = document.querySelector('.inspector-messages');
        if (container) {
            const messages = container.querySelectorAll('.inspector-message');
            messages.forEach(msg => msg.remove());
        }
    }

    /**
     * Handle theme change from main PWA
     */
    public onThemeChange(theme: 'dark' | 'light'): void {
        document.documentElement.setAttribute('data-theme', theme);
    }
}
```

**Step 2: Build to verify**

Run: `make build`
Expected: Build succeeds

**Step 3: Commit inspector service**

```bash
git add internal/webapp/src/services/inspector-service.ts
git commit -m "feat: add inspector service with search and export"
```

---

## Task 9: Add Inspector Controls to UI

**Files:**
- Modify: `internal/webapp/web/protocol-inspector.html` (add controls)

**Step 1: Add inspector controls HTML**

In `internal/webapp/web/protocol-inspector.html`, find the messages container and add controls above it:

```html
<div class="inspector-controls">
    <input
        type="text"
        id="inspectorSearch"
        class="inspector-search"
        placeholder="Search messages..."
    >
    <div class="inspector-buttons">
        <button id="inspectorExport" class="btn btn-small">
            Export JSON
        </button>
        <button id="inspectorClear" class="btn btn-small btn-danger">
            Clear
        </button>
    </div>
</div>
```

**Step 2: Add inspector controls CSS**

Add to `internal/webapp/web/inspector.css`:

```css
/* Inspector Controls */
.inspector-controls {
  display: flex;
  gap: 0.75rem;
  padding: 0.75rem;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
  align-items: center;
}

.inspector-search {
  flex: 1;
  padding: 0.5rem 0.75rem;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 0.875rem;
}

.inspector-search:focus {
  outline: none;
  border-color: var(--accent-primary);
}

.inspector-buttons {
  display: flex;
  gap: 0.5rem;
}

.inspector-controls .btn {
  white-space: nowrap;
}
```

**Step 3: Build to verify**

Run: `make build`
Expected: Build succeeds, assets compile

**Step 4: Commit inspector UI**

```bash
git add internal/webapp/web/protocol-inspector.html internal/webapp/web/inspector.css
git commit -m "feat: add inspector controls UI (search, export, clear)"
```

---

## Task 10: Wire Inspector Service to UI

**Files:**
- Modify: Protocol inspector JavaScript file (likely in `internal/webapp/src/`)

**Step 1: Find and modify protocol inspector script**

Look for the protocol inspector initialization code. It's likely in a file like `internal/webapp/src/protocol-inspector.ts` or embedded in `protocol-inspector.html`.

Import inspector service:

```typescript
import { InspectorService } from './services/inspector-service';
```

Initialize service:

```typescript
const inspector = new InspectorService();
```

**Step 2: Wire search input**

```typescript
const searchInput = document.getElementById('inspectorSearch') as HTMLInputElement;
if (searchInput) {
    // Debounce search to avoid filtering on every keystroke
    let searchTimeout: number;
    searchInput.addEventListener('input', (e) => {
        clearTimeout(searchTimeout);
        searchTimeout = window.setTimeout(() => {
            inspector.search((e.target as HTMLInputElement).value);
        }, 150);
    });
}
```

**Step 3: Wire export button**

```typescript
const exportBtn = document.getElementById('inspectorExport');
if (exportBtn) {
    exportBtn.addEventListener('click', () => {
        inspector.exportJSON();
    });
}
```

**Step 4: Wire clear button**

```typescript
const clearBtn = document.getElementById('inspectorClear');
if (clearBtn) {
    clearBtn.addEventListener('click', () => {
        inspector.clear();
    });
}
```

**Step 5: Add theme listener**

```typescript
window.addEventListener('message', (event) => {
    if (event.data.type === 'theme:change') {
        inspector.onThemeChange(event.data.theme);
    }
});
```

**Step 6: Track messages in inspector**

When messages are displayed, also add to inspector service:

```typescript
function displayMessage(direction: 'recv' | 'send', type: string, payload: any) {
    // Add to inspector history
    inspector.addMessage(direction, type, payload);

    // ... existing display code
}
```

**Step 7: Build and test**

Run: `make build`
Expected: Build succeeds

Manual test:
1. Open protocol inspector
2. Send/receive messages
3. Type in search box, verify filtering
4. Click Export, verify JSON downloads
5. Click Clear, verify messages cleared

**Step 8: Commit inspector wiring**

```bash
git add internal/webapp/src/*.ts
git commit -m "feat: wire inspector service to UI controls"
```

---

## Task 11: Final Integration and Testing

**Files:**
- Test all features together

**Step 1: Run full build**

Run: `make build`
Expected: All assets compile successfully

**Step 2: Run Go tests**

Run: `go test ./...`
Expected: All tests pass

**Step 3: Manual integration testing**

Test checklist:

**Theme:**
- [ ] Toggle switches between dark and light
- [ ] Theme persists after reload
- [ ] Inspector theme syncs with main PWA
- [ ] All UI elements readable in both themes

**Loading:**
- [ ] Spinner shows during agent spawn
- [ ] Spinner hides on success
- [ ] Spinner hides on error

**Notifications:**
- [ ] Error notifications show
- [ ] Retry button appears for recoverable errors
- [ ] Retry executes correctly
- [ ] Dismiss removes notification

**Inspector:**
- [ ] Search filters messages
- [ ] Export downloads JSON
- [ ] Clear removes messages
- [ ] Theme changes apply to inspector

**Step 4: Fix any issues found**

If tests fail or manual testing reveals issues, fix them before committing.

**Step 5: Final commit**

```bash
git add -A
git commit -m "test: verify all PWA UX enhancements working together"
```

---

## Task 12: Create Pull Request

**Files:**
- N/A (GitHub operations)

**Step 1: Push branch**

Run: `git push -u origin feat/pwa-ux-enhancements`
Expected: Branch pushed to GitHub

**Step 2: Create PR**

Run:
```bash
gh pr create --title "feat: PWA UX enhancements (theme, inspector, retry, loading)" --body "$(cat <<'EOF'
## Summary
Implements four PWA UX enhancements for milestone 9:

1. **Theme Toggle (#66)** - Dark/light mode with persistence and system preference detection
2. **Inspector Search/Export (#62)** - Search messages and export as JSON
3. **Retry Button (#61)** - Retry button for recoverable errors
4. **Agent Spawn Progress (#60)** - Loading indicator during agent spawn

## Changes

### New Services (Modular Architecture)
- `theme-service.ts` - Theme management and persistence
- `loading-service.ts` - Progress indicators
- `notification-service.ts` - Enhanced notifications with retry
- `inspector-service.ts` - Search and export functionality

### Modified Files
- `state.ts` - Compose and wire all services
- `styles.css` - CSS variables for theming, new component styles
- `index.html` - Theme toggle button
- `protocol-inspector.html` - Inspector controls
- `inspector.css` - Inspector styling
- `connection.ts` - Track operation context for retry

## Testing
✅ All Go tests pass
✅ Build succeeds
✅ Manual testing completed:
  - Theme switching and persistence works
  - Inspector theme syncs with main PWA
  - Search filters messages correctly
  - Export produces valid JSON
  - Retry button executes correctly
  - Loading indicator shows/hides properly

## Design
See: `docs/plans/2025-01-17-pwa-ux-enhancements-design.md`

Closes #66
Closes #62
Closes #61
Closes #60

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

**Step 3: Verify PR created**

Run: `gh pr view --web`
Expected: PR opens in browser

---

## Success Criteria

1. ✅ Theme toggle switches between dark and light modes
2. ✅ Theme persists across page reloads
3. ✅ Inspector iframe theme syncs with main PWA
4. ✅ Inspector search filters messages by content
5. ✅ Inspector export downloads valid JSON
6. ✅ Retry button appears for recoverable errors
7. ✅ Retry button executes operation correctly
8. ✅ Loading indicator shows during agent spawn
9. ✅ All features work without conflicts
10. ✅ All tests pass
11. ✅ PR created and ready for review
