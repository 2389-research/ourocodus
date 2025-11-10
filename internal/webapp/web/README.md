# Ourocodus PWA

This directory contains the Progressive Web Application (PWA) for Ourocodus, including the Protocol Inspector tool for visualizing WebSocket communication.

## Overview

The PWA showcases the full lifecycle of multi-agent interactions:
- WebSocket connection management
- Session creation and lifecycle
- Agent spawning, messaging, and termination
- Real-time protocol inspection
- Error handling and recovery

## Running the Application

### Prerequisites

- Ourocodus relay server running on `localhost:3000`
- Echo agent binary built and available

### Basic Setup

1. **Build the echo agent:**
   ```bash
   make build
   ```

2. **Start the relay server:**
   ```bash
   OUROCODUS_ACP_BINARY=/Users/clint/code/ourocodus/bin/echo-agent ./bin/relay
   ```

3. **Open the application:**
   - **PWA interface**: http://localhost:3000/
   - **Protocol Inspector**: http://localhost:3000/protocol-inspector.html

### Environment Variables

- `OUROCODUS_ACP_BINARY`: Path to the ACP agent binary (defaults to `claude-code-acp`)

## Files

### PWA Application

- **`index.html`**: Main PWA interface
- **`app.js`**: Application logic and WebSocket connection manager
  - `RelayConnection` class: Handles WebSocket lifecycle
  - `ModalManager` class: Reusable modal management
  - `App` class: UI coordination and event handling
- **`styles.css`**: Dark theme with purple accents, animations, and responsive design

### Protocol Inspector

- **`protocol-inspector.html`**: Split-screen inspector UI
- **`protocol-inspector.js`**: Protocol Inspector class for message visualization
- **`websocket-interceptor.js`**: WebSocket interceptor for non-invasive traffic capture
- **`inspector.css`**: Inspector-specific styling

## Features

### Connection Management

The application supports three distinct connection control actions:

1. **Disconnect** (Header button)
   - Closes WebSocket connection only
   - Server automatically cleans up session/agents
   - UI state remains visible for review
   - Tooltip: "Close WebSocket connection only. Server will cleanup session automatically. UI state remains visible."

2. **End Session** (Session card button)
   - Sends `session:end` protocol message
   - Terminates all agents
   - Disconnects from relay
   - Resets to initial state
   - Tooltip: "Terminate all agents, end session, and disconnect"

3. **Terminate Agent** (Agent card button)
   - Sends `agent:terminate` protocol message
   - Terminates only the current agent
   - Session remains active
   - Can spawn new agents
   - Tooltip: "Terminate only this agent. Session remains active, you can spawn a new agent."

### Visual Feedback

- **Loading States**: Spinner animation during connection/reconnection
- **Status Indicator**:
  - Red pulsing: Disconnected
  - Orange spinning: Connecting
  - Green solid: Connected
- **Error Notifications**: Beautiful sliding notifications with:
  - Error code and message
  - Recoverable vs fatal indicators
  - Auto-dismiss after 10 seconds
  - Click to dismiss

### Accessibility

All modals include:
- `role="dialog"` and `aria-modal="true"`
- `aria-labelledby` and `aria-describedby` for screen readers
- `aria-label` on action buttons
- `aria-hidden="true"` on decorative icons

### Button State Management

All action buttons properly disable during operations to prevent:
- Double-clicks
- Race conditions
- Multiple simultaneous operations

## Protocol Inspector

The Protocol Inspector provides real-time WebSocket protocol visualization without modifying the production PWA code.

### Architecture

```
┌─────────────────┬──────────────────┐
│   PWA (iframe)  │   Inspector      │
│                 │                  │
│  WebSocket      │  Message Stream  │
│  ↓ Shim wraps   │  • Timestamp     │
│  ↓ all traffic  │  • Direction     │
│  ↓              │  • JSON payload  │
│  postMessage ─────→ Real-time      │
│                 │    updates       │
│                 │                  │
│                 │  Session Panel   │
│                 │  • Session ID    │
│                 │  • State         │
│                 │  • Errors        │
└─────────────────┴──────────────────┘
```

### How It Works

1. **protocol-inspector.html** loads the PWA in an iframe
2. **websocket-interceptor.js** is injected into the iframe
3. Interceptor wraps native `WebSocket` constructor
4. All WebSocket events broadcast via `postMessage()`
5. **protocol-inspector.js** receives and displays messages in real-time

### Key Features

- **Non-invasive**: No PWA code changes required
- **Real-time**: Zero-latency message display
- **Complete capture**: Open, message, close, error events
- **Click to copy**: Click any message to copy to clipboard
- **Session tracking**: Automatic session ID extraction and display
- **Error highlighting**: Visual emphasis on error messages

## Protocol Messages

### Supported Message Types

#### Client → Server

- **`session:create`**: Create a new session
- **`agent:spawn`**: Spawn an agent with role and workspace
- **`agent:message`**: Send message to an agent
- **`session:end`**: End session (Phase 3, server support pending)
- **`agent:terminate`**: Terminate specific agent (Phase 3, server support pending)

#### Server → Client

- **`connection:established`**: WebSocket handshake confirmation
- **`session:created`**: Session created with ID
- **`agent:ready`**: Agent spawned and ready
- **`agent:response`**: Agent response message
- **`error`**: Error with code, message, and recoverable flag

### Error Codes

- **`UNKNOWN_MESSAGE_TYPE`**: Server doesn't recognize message type
- **`AGENT_SPAWN_FAILED`**: Failed to spawn agent
- **`WORKSPACE_VALIDATION_ERROR`**: Invalid workspace path

## User Flows

### Complete Agent Interaction

1. Click "New Project"
   - Connects to relay
   - Creates session
   - Session card appears

2. Enter agent role and workspace
   - Default role: `echo`
   - Default workspace: `./workspaces/demo`

3. Click "Spawn Agent"
   - Agent card appears
   - Welcome message displayed

4. Send messages
   - Type in textarea
   - Press Enter or click "Send Message"
   - User messages on right (purple)
   - Agent messages on left (green)

5. Terminate agent (optional)
   - Click "Terminate" in agent card
   - Confirm in modal
   - Returns to spawn section

6. End session
   - Click "End Session" in session card
   - Confirm in modal
   - Returns to initial welcome state

## Development

### Code Organization

- **RelayConnection**: WebSocket lifecycle and reconnection logic
- **ModalManager**: Reusable modal management (DRY principle)
- **App**: UI coordination, button handlers, state management
- **ProtocolInspector**: Message capture and visualization

### Reconnection Strategy

- Exponential backoff with base delay of 1 second
- Max delay capped at 30 seconds
- Max 10 reconnection attempts
- Automatic reconnection on disconnect (unless explicitly disabled)

### Modal Pattern

```javascript
this.disconnectModal.show({
    onConfirm: () => {
        // Confirmation logic
    },
    onCancel: () => {
        // Cancellation logic
    },
    updateContent: (modal) => {
        // Dynamic content updates
    }
});
```

## Styling

### Color Palette

```css
--bg-primary: #0a0a0f;      /* Deep black background */
--accent-primary: #7c3aed;   /* Purple accent */
--success: #10b981;          /* Green for success */
--error: #f43f5e;            /* Red for errors */
--warning: #f59e0b;          /* Orange for warnings */
```

### Animations

- **pulse**: Status indicator disconnected state
- **spin**: Status indicator connecting state
- **slideIn**: Error notification entrance
- **fadeOut**: Error notification exit
- **fadeIn**: Modal backdrop
- **slideUp**: Modal content entrance

## Troubleshooting

### Connection Fails

- Verify relay server is running on port 3000
- Check WebSocket URL in browser console
- Ensure no firewall blocking WebSocket connections

### Agent Spawn Fails

- Verify `OUROCODUS_ACP_BINARY` environment variable is set
- Check agent binary path is correct and executable
- Ensure workspace path is under `./workspaces/`
- Review relay server logs for detailed error messages

### Protocol Messages Not Working

Some protocol messages are Phase 3 features:
- `session:end`: Client ready, server support pending
- `agent:terminate`: Client ready, server support pending

These will return `UNKNOWN_MESSAGE_TYPE` errors until server implements handlers.

### Inspector Not Showing Messages

- Ensure you're using the Protocol Inspector (demo.html)
- Check browser console for shim injection errors
- Verify iframe loaded successfully
- Try hard refresh (Cmd+Shift+R / Ctrl+Shift+F5)

## Future Enhancements

### Planned Features (Phase 3)

- Server-side `session:end` handler
- Server-side `agent:terminate` handler
- Response messages: `session:ended`, `agent:terminated`
- Multiple concurrent agents
- Agent state persistence
- Workspace isolation improvements

### Potential Improvements

- Retry button on error notifications
- Agent spawn progress indicator
- Message search/filter in inspector
- Export message history
- Dark/light theme toggle
- Mobile-responsive improvements

## Testing

### Manual Test Scenarios

1. **Connection Lifecycle**
   - Connect → disconnect → reconnect
   - Verify status indicator states
   - Check automatic reconnection

2. **Session Management**
   - Create session
   - End session
   - Verify UI reset

3. **Agent Lifecycle**
   - Spawn agent
   - Send messages
   - Terminate agent
   - Spawn new agent

4. **Error Handling**
   - Invalid workspace path
   - Missing agent binary
   - Network interruption
   - Verify error notifications

5. **Protocol Inspector**
   - All messages captured
   - Click to copy works
   - Session panel updates
   - Error messages highlighted

## Contributing

When modifying the demo:

1. Test all three connection control actions
2. Verify accessibility with screen readers
3. Check button states prevent duplicate actions
4. Ensure tooltips explain action scope
5. Test Protocol Inspector captures new messages
6. Update this documentation

## License

See parent directory LICENSE file.
