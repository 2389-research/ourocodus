# PWA Protocol Inspector Demo Tool

**Date:** 2025-10-29
**Status:** Design
**Purpose:** Demonstrate PWA connection reliability and protocol correctness to the team

## Overview

The Protocol Inspector provides a split-screen view of the PWA's WebSocket communication with the relay server. It shows the live PWA interface alongside a real-time message stream, connection state indicator, and session details. This tool serves three purposes: team motivation (visual proof of progress), technical validation (verify protocol correctness), and live demonstration (explain WebSocket flow to stakeholders).

The inspector captures all WebSocket traffic without modifying the PWA's production code. It injects a shim that intercepts messages and broadcasts them to the demo interface via `postMessage`. The PWA runs identically whether inspected or not.

## Architecture

### Component Layout

```
+------------------------+------------------------+
|                        |                        |
|   PWA (iframe)         |   Message Stream       |
|                        |   (scrolling log)      |
|   index.html           |                        |
|                        +------------------------+
|                        |                        |
|                        |   Connection State     |
|                        |   (text indicator)     |
|                        +------------------------+
|                        |                        |
|                        |   Session Inspector    |
|                        |   (details panel)      |
|                        |                        |
+------------------------+------------------------+
    50% width                   50% width
```

### Message Flow

1. User opens `http://localhost:8080/demo.html`
2. `demo.html` loads PWA in iframe with `?demo=true` parameter
3. `demo.js` detects parameter and injects `demo-shim.js` into iframe
4. `demo-shim.js` wraps native WebSocket constructor
5. All WebSocket events broadcast to parent via `postMessage`
6. `demo.js` receives messages and updates three panels

### File Structure

```
web/
├── demo.html          # Protocol Inspector UI (new)
├── demo.css           # Styling for inspector panels (new)
├── demo.js            # Inspector logic (new)
├── demo-shim.js       # WebSocket interceptor (new)
├── index.html         # PWA (unchanged)
├── app.js             # PWA logic (unchanged)
└── styles.css         # PWA styles (unchanged)
```

## Implementation

### 1. demo-shim.js (WebSocket Interceptor)

This script wraps the native WebSocket constructor to intercept all traffic.

```javascript
// Store original constructor
const OriginalWebSocket = window.WebSocket;

// Replace with wrapper
window.WebSocket = function(url, protocols) {
    const ws = new OriginalWebSocket(url, protocols);

    // Broadcast connection open
    ws.addEventListener('open', e => {
        window.parent.postMessage({
            type: 'ws:open',
            url: url,
            timestamp: new Date().toISOString()
        }, '*');
    });

    // Broadcast incoming messages
    ws.addEventListener('message', e => {
        window.parent.postMessage({
            type: 'ws:message',
            direction: 'received',
            data: e.data,
            timestamp: new Date().toISOString()
        }, '*');
    });

    // Intercept send() to broadcast outgoing messages
    const originalSend = ws.send;
    ws.send = function(data) {
        window.parent.postMessage({
            type: 'ws:message',
            direction: 'sent',
            data: data,
            timestamp: new Date().toISOString()
        }, '*');
        return originalSend.call(this, data);
    };

    // Broadcast connection close
    ws.addEventListener('close', e => {
        window.parent.postMessage({
            type: 'ws:close',
            code: e.code,
            reason: e.reason,
            timestamp: new Date().toISOString()
        }, '*');
    });

    // Broadcast errors
    ws.addEventListener('error', e => {
        window.parent.postMessage({
            type: 'ws:error',
            timestamp: new Date().toISOString()
        }, '*');
    });

    return ws;
};
```

### 2. demo.js (Inspector Logic)

This script receives messages from the iframe and updates the UI.

```javascript
class ProtocolInspector {
    constructor() {
        this.messageList = document.getElementById('message-list');
        this.stateIndicator = document.getElementById('state-indicator');
        this.sessionPanel = document.getElementById('session-panel');
        this.currentState = 'disconnected';
        this.sessionId = null;
        this.reconnectAttempts = 0;

        window.addEventListener('message', this.handleMessage.bind(this));
    }

    handleMessage(event) {
        const msg = event.data;

        switch (msg.type) {
            case 'ws:open':
                this.updateState('connected');
                this.appendMessage('Connection opened', 'system', msg.timestamp);
                break;

            case 'ws:message':
                this.appendMessage(msg.data, msg.direction, msg.timestamp);
                this.processProtocolMessage(msg.data);
                break;

            case 'ws:close':
                this.updateState('disconnected');
                this.appendMessage(`Connection closed: ${msg.code}`, 'system', msg.timestamp);
                break;

            case 'ws:error':
                this.updateState('error');
                this.appendMessage('Connection error', 'error', msg.timestamp);
                break;
        }
    }

    appendMessage(data, type, timestamp) {
        const entry = document.createElement('div');
        entry.className = `message-entry message-${type}`;

        const time = new Date(timestamp).toLocaleTimeString();
        const direction = type === 'sent' ? '→' : type === 'received' ? '←' : '•';

        let content = data;
        if (type === 'sent' || type === 'received') {
            try {
                const json = JSON.parse(data);
                content = this.formatJSON(json);
            } catch (e) {
                content = data;
            }
        }

        entry.innerHTML = `
            <span class="timestamp">${time}</span>
            <span class="direction">${direction}</span>
            <pre class="content">${content}</pre>
        `;

        this.messageList.appendChild(entry);
        this.messageList.scrollTop = this.messageList.scrollHeight;
    }

    formatJSON(obj) {
        return JSON.stringify(obj, null, 2);
    }

    updateState(state) {
        this.currentState = state;
        this.stateIndicator.textContent = `Connection: ${state}`;
        this.stateIndicator.className = `state-${state}`;
    }

    processProtocolMessage(data) {
        try {
            const msg = JSON.parse(data);

            if (msg.type === 'session:created') {
                this.sessionId = msg.sessionId;
                this.updateSessionPanel();
            }

            if (msg.type === 'error') {
                this.updateSessionPanel(msg.code, msg.message);
            }
        } catch (e) {
            // Not JSON, ignore
        }
    }

    updateSessionPanel(errorCode = null, errorMsg = null) {
        let html = '<h3>Session</h3>';

        if (this.sessionId) {
            html += `<div><strong>ID:</strong> ${this.sessionId}</div>`;
            html += `<div><strong>State:</strong> ${this.currentState}</div>`;
        } else {
            html += '<div>No session</div>';
        }

        if (errorCode) {
            html += `<div class="error"><strong>Error:</strong> ${errorCode}</div>`;
            html += `<div class="error">${errorMsg}</div>`;
        }

        this.sessionPanel.innerHTML = html;
    }
}

// Initialize when DOM loads
document.addEventListener('DOMContentLoaded', () => {
    new ProtocolInspector();

    // Check if iframe has demo parameter, inject shim if so
    const iframe = document.getElementById('pwa-frame');
    iframe.addEventListener('load', () => {
        const isDemoMode = new URLSearchParams(iframe.contentWindow.location.search).get('demo') === 'true';
        if (isDemoMode) {
            fetch('/demo-shim.js')
                .then(r => r.text())
                .then(code => {
                    const script = iframe.contentDocument.createElement('script');
                    script.textContent = code;
                    iframe.contentDocument.head.appendChild(script);
                });
        }
    });
});
```

### 3. demo.html (UI Structure)

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Protocol Inspector - Ourocodus PWA</title>
    <link rel="stylesheet" href="/demo.css">
</head>
<body>
    <div class="container">
        <div class="left-panel">
            <iframe id="pwa-frame" src="/?demo=true"></iframe>
        </div>
        <div class="right-panel">
            <div class="message-stream">
                <h2>Message Stream</h2>
                <div id="message-list"></div>
            </div>
            <div class="connection-state">
                <h2>Connection</h2>
                <div id="state-indicator" class="state-disconnected">Connection: disconnected</div>
            </div>
            <div class="session-info">
                <div id="session-panel">
                    <h3>Session</h3>
                    <div>No session</div>
                </div>
            </div>
        </div>
    </div>
    <script src="/demo.js"></script>
</body>
</html>
```

### 4. demo.css (Styling)

```css
* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #1a1a1a;
    color: #e0e0e0;
    height: 100vh;
    overflow: hidden;
}

.container {
    display: flex;
    height: 100vh;
}

.left-panel {
    flex: 1;
    border-right: 2px solid #333;
}

.right-panel {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

#pwa-frame {
    width: 100%;
    height: 100%;
    border: none;
}

.message-stream {
    flex: 2;
    display: flex;
    flex-direction: column;
    border-bottom: 2px solid #333;
    overflow: hidden;
}

.message-stream h2 {
    padding: 1rem;
    background: #252525;
    font-size: 1rem;
    border-bottom: 1px solid #333;
}

#message-list {
    flex: 1;
    overflow-y: auto;
    padding: 1rem;
    font-family: 'Courier New', monospace;
    font-size: 0.875rem;
}

.message-entry {
    margin-bottom: 1rem;
    padding: 0.5rem;
    border-radius: 4px;
}

.message-sent {
    background: rgba(0, 255, 0, 0.05);
    border-left: 3px solid #00ff00;
}

.message-received {
    background: rgba(0, 150, 255, 0.05);
    border-left: 3px solid #0096ff;
}

.message-system {
    background: rgba(128, 128, 128, 0.05);
    border-left: 3px solid #808080;
}

.message-error {
    background: rgba(255, 0, 0, 0.05);
    border-left: 3px solid #ff0000;
}

.timestamp {
    color: #888;
    margin-right: 0.5rem;
}

.direction {
    font-weight: bold;
    margin-right: 0.5rem;
}

.message-sent .direction {
    color: #00ff00;
}

.message-received .direction {
    color: #0096ff;
}

.content {
    margin-top: 0.25rem;
    white-space: pre-wrap;
    word-break: break-all;
}

.connection-state {
    flex: 0 0 auto;
    padding: 1rem;
    background: #252525;
    border-bottom: 2px solid #333;
}

.connection-state h2 {
    font-size: 1rem;
    margin-bottom: 0.5rem;
}

#state-indicator {
    padding: 0.5rem;
    border-radius: 4px;
    font-weight: bold;
}

.state-connected {
    background: rgba(0, 255, 0, 0.1);
    color: #00ff00;
}

.state-disconnected,
.state-error {
    background: rgba(255, 0, 0, 0.1);
    color: #ff5555;
}

.state-connecting,
.state-reconnecting {
    background: rgba(255, 165, 0, 0.1);
    color: #ffaa00;
}

.session-info {
    flex: 1;
    padding: 1rem;
    overflow-y: auto;
}

.session-info h3 {
    font-size: 0.875rem;
    margin-bottom: 0.5rem;
    color: #888;
}

.session-info div {
    margin-bottom: 0.5rem;
}

.error {
    color: #ff5555;
}
```

## Usage

### Starting the Demo

1. Start relay server:
   ```bash
   ANTHROPIC_API_KEY=test-key ./bin/relay
   ```

2. Open Protocol Inspector:
   ```
   http://localhost:8080/demo.html
   ```

### Demo Script

**Scene 1: Happy Path (Session Creation)**

1. Point out the split screen: "PWA on left, protocol inspector on right"
2. Click "New Project" button in PWA
3. Watch message stream:
   - Green `→` shows outgoing messages
   - Blue `←` shows incoming messages
4. Highlight key protocol fields:
   - `version: "1.0"` in outgoing messages
   - `sessionId` in response
5. Show connection state changes: disconnected → connecting → connected
6. Show session panel updates with session ID

**Scene 2: Reconnection**

1. Stop relay server (`Ctrl+C`)
2. Watch connection state change to "disconnected"
3. Watch PWA status indicator turn red
4. Restart relay server
5. Watch automatic reconnection succeed
6. Show that session remains intact

**Scene 3: Error Handling**

1. Manually send malformed message using browser console:
   ```javascript
   // Access iframe's WebSocket (for demo purposes)
   iframe.contentWindow.ws.send('{"type":"session:create"}')
   ```
2. Watch error appear in message stream (red border)
3. Show error details in session panel
4. Demonstrate that PWA handles error gracefully

**Scene 4: Protocol Correctness**

1. Scroll through message history
2. Point out consistent JSON structure
3. Show timestamps on all messages
4. Highlight version field on all client messages
5. Show proper message sequencing: open → established → create → created

## Testing

### Manual Verification

1. **Inspector loads correctly**
   - Open demo.html
   - Verify PWA appears in left iframe
   - Verify right panel shows three sections
   - Verify no console errors

2. **Message capture works**
   - Click "New Project"
   - Verify messages appear in stream
   - Verify colors: green for sent, blue for received
   - Verify JSON is formatted (indented)

3. **State tracking works**
   - Verify initial state shows "disconnected"
   - Click "New Project"
   - Verify state changes to "connected"
   - Stop relay server
   - Verify state changes to "disconnected"

4. **Session tracking works**
   - Create session
   - Verify session ID appears in session panel
   - Verify session state matches connection state

5. **PWA operates normally**
   - Compare PWA behavior with and without `?demo=true`
   - Verify no functional differences
   - Verify no performance degradation

### Browser Compatibility

Test in:
- Chrome (primary)
- Firefox (secondary)
- Safari (if available)

Verify:
- `postMessage` works correctly
- iframe sandbox doesn't block script injection
- WebSocket interception doesn't break PWA

## Deployment

Add demo files to relay's static file serving:

```go
// In pkg/relay/server.go, add routes
mux.Handle("/demo.html", http.FileServer(http.Dir("web")))
mux.Handle("/demo.css", http.FileServer(http.Dir("web")))
mux.Handle("/demo.js", http.FileServer(http.Dir("web")))
mux.Handle("/demo-shim.js", http.FileServer(http.Dir("web")))
```

Or use existing static file handler if it serves entire `web/` directory.

## Future Enhancements

Deferred to keep initial implementation simple:

- Message filtering (show/hide connection:established)
- Pause/resume message stream
- Download message log as JSON
- Visual state machine diagram (SVG animation)
- Manual message sending (test interface)
- Light theme toggle for projectors
- Adjustable panel split ratio

## References

- **PWA Documentation:** `docs/PWA.md`
- **WebSocket Protocol:** `docs/PWA.md` (Message Types section)
- **Relay Implementation:** `pkg/relay/server.go`, `pkg/relay/message.go`
