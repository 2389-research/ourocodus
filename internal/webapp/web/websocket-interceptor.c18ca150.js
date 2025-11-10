// WebSocket Interceptor for Protocol Inspector
// This script wraps the native WebSocket constructor to capture all traffic
// and broadcast it to the parent window for inspection.

(function() {
    'use strict';

    // Store original constructor
    const OriginalWebSocket = window.WebSocket;

    // Replace with wrapper
    window.WebSocket = function(url, protocols) {
        const ws = new OriginalWebSocket(url, protocols);

        // Broadcast connection open
        ws.addEventListener('open', function(e) {
            window.parent.postMessage({
                type: 'ws:open',
                url: url,
                timestamp: new Date().toISOString()
            }, '*');
        });

        // Broadcast incoming messages
        ws.addEventListener('message', function(e) {
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
        ws.addEventListener('close', function(e) {
            window.parent.postMessage({
                type: 'ws:close',
                code: e.code,
                reason: e.reason,
                timestamp: new Date().toISOString()
            }, '*');
        });

        // Broadcast errors
        ws.addEventListener('error', function(e) {
            window.parent.postMessage({
                type: 'ws:error',
                timestamp: new Date().toISOString()
            }, '*');
        });

        return ws;
    };

    // Preserve prototype
    window.WebSocket.prototype = OriginalWebSocket.prototype;

    // Copy static constants (CONNECTING, OPEN, CLOSING, CLOSED)
    window.WebSocket.CONNECTING = OriginalWebSocket.CONNECTING;
    window.WebSocket.OPEN = OriginalWebSocket.OPEN;
    window.WebSocket.CLOSING = OriginalWebSocket.CLOSING;
    window.WebSocket.CLOSED = OriginalWebSocket.CLOSED;

    console.log('[WebSocket Interceptor] WebSocket interceptor installed');
})();
