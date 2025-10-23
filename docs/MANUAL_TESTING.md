# Manual Testing Guide

## Relay WebSocket Server

### Prerequisites
```bash
brew install websocat
```

### Start the Relay Server
```bash
cd /Users/clint/code/ourocodus
go run cmd/relay/main.go
```

Expected output:
```
Relay server starting on port 8080
WebSocket endpoint: ws://localhost:8080/ws
```

### Test 1: Connection Handshake
```bash
# In a new terminal
websocat ws://localhost:8080/ws
```

Expected response (immediate):
```json
{"version":"1.0","type":"connection:established","serverId":"<uuid>","timestamp":"2025-10-22T..."}
```

### Test 2: Echo Message
After connecting, type:
```json
{"version":"1.0","type":"test:echo","message":"hello world"}
```

Expected response:
```json
{"version":"1.0","type":"test:echo","message":"hello world","timestamp":"2025-10-22T..."}
```

### Test 3: Version Mismatch
```json
{"version":"2.0","type":"test:echo"}
```

Expected response:
```json
{"version":"1.0","type":"error","code":"VERSION_MISMATCH","message":"Unsupported protocol version","timestamp":"2025-10-22T..."}

### Test 4: Missing Version Field
```json
{"type":"test:echo","message":"test"}
```

Expected: No response (message rejected, check server logs for error)

### Test 5: Graceful Shutdown
Press `Ctrl+C` in the server terminal.

Expected output:
```
Shutdown signal received, gracefully stopping server...
Server stopped
```

## Automated Tests

All functionality is covered by automated tests:
```bash
go test ./pkg/relay -v
```

Tests include:
- Message validation (missing fields, version mismatch)
- WebSocket connection handshake
- Echo functionality with timestamp
- Connection lifecycle
