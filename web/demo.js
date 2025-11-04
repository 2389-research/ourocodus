// Protocol Inspector Logic
// Receives WebSocket events from the demo-shim and updates the UI

class ProtocolInspector {
    constructor() {
        this.messageList = document.getElementById('message-list');
        this.stateIndicator = document.getElementById('state-indicator');
        this.sessionPanel = document.getElementById('session-panel');
        this.currentState = 'disconnected';
        this.userSessionId = null;
        this.reconnectAttempts = 0;

        if (!this.messageList || !this.stateIndicator || !this.sessionPanel) {
            console.error('[Protocol Inspector] Required DOM elements not found');
            return;
        }

        window.addEventListener('message', this.handleMessage.bind(this));
        console.log('[Protocol Inspector] Initialized');
    }

    handleMessage(event) {
        const msg = event.data;

        // Ignore messages that aren't from our demo shim
        if (!msg.type || !msg.type.startsWith('ws:')) {
            return;
        }

        console.log('[Protocol Inspector] Received message from shim:', msg.type);

        switch (msg.type) {
            case 'ws:open':
                this.updateState('connected');
                this.appendMessage('Connection opened to ' + msg.url, 'system', msg.timestamp);
                break;

            case 'ws:message':
                this.appendMessage(msg.data, msg.direction, msg.timestamp);
                this.processProtocolMessage(msg.data);
                break;

            case 'ws:close':
                this.updateState('disconnected');
                const reason = msg.reason || 'No reason provided';
                this.appendMessage('Connection closed: ' + msg.code + ' (' + reason + ')', 'system', msg.timestamp);
                break;

            case 'ws:error':
                this.updateState('error');
                this.appendMessage('Connection error occurred', 'error', msg.timestamp);
                break;
        }
    }

    appendMessage(data, type, timestamp) {
        const entry = document.createElement('div');
        entry.className = 'message-entry message-' + type;

        const time = new Date(timestamp).toLocaleTimeString();
        let direction = '•';

        if (type === 'sent') {
            direction = '→';
        } else if (type === 'received') {
            direction = '←';
        }

        let content = data;
        if (type === 'sent' || type === 'received') {
            try {
                const json = JSON.parse(data);
                content = this.formatJSON(json);
            } catch (e) {
                content = this.escapeHtml(data);
            }
        } else {
            content = this.escapeHtml(content);
        }

        entry.innerHTML = '<span class="timestamp">' + time + '</span>' +
                         '<span class="direction">' + direction + '</span>' +
                         '<pre class="content">' + content + '</pre>';

        // Add click to copy
        entry.addEventListener('click', function() {
            navigator.clipboard.writeText(data).then(function() {
                console.log('[Protocol Inspector] Copied to clipboard');
            }).catch(function(err) {
                console.error('[Protocol Inspector] Failed to copy:', err);
            });
        });
        entry.style.cursor = 'pointer';
        entry.title = 'Click to copy';

        this.messageList.appendChild(entry);
        this.messageList.scrollTop = this.messageList.scrollHeight;
    }

    formatJSON(obj) {
        return this.escapeHtml(JSON.stringify(obj, null, 2));
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    updateState(state) {
        this.currentState = state;
        this.stateIndicator.textContent = 'Connection: ' + state;
        this.stateIndicator.className = 'state-' + state;
    }

    processProtocolMessage(data) {
        try {
            const msg = JSON.parse(data);
            console.log('[Protocol Inspector] Processing message:', msg);

            if (msg.type === 'session:created') {
                console.log('[Protocol Inspector] Session created! ID:', msg.userSessionId);
                this.userSessionId = msg.userSessionId;
                this.updateSessionPanel();
                console.log('[Protocol Inspector] Session panel updated with ID:', this.userSessionId);
            }

            if (msg.type === 'error') {
                console.log('[Protocol Inspector] Error message:', msg.code, msg.message);
                this.updateSessionPanel(msg.code, msg.message);
            }
        } catch (e) {
            console.log('[Protocol Inspector] Failed to parse message (not JSON):', e);
        }
    }

    updateSessionPanel(errorCode, errorMsg) {
        console.log('[Protocol Inspector] updateSessionPanel called. SessionId:', this.userSessionId, 'ErrorCode:', errorCode);

        if (!this.sessionPanel) {
            console.error('[Protocol Inspector] sessionPanel element not found!');
            return;
        }

        let html = '<h3>Session</h3>';

        if (this.userSessionId) {
            html += '<div><strong>ID:</strong> ' + this.userSessionId + '</div>';
            html += '<div><strong>State:</strong> ' + this.currentState + '</div>';
        } else {
            html += '<div>No session</div>';
        }

        if (errorCode) {
            html += '<div class="error"><strong>Error:</strong> ' + errorCode + '</div>';
            html += '<div class="error">' + this.escapeHtml(errorMsg || '') + '</div>';
        }

        this.sessionPanel.innerHTML = html;
        console.log('[Protocol Inspector] Session panel HTML updated:', html);
    }
}

// Initialize when DOM loads
document.addEventListener('DOMContentLoaded', function() {
    const inspector = new ProtocolInspector();

    // Inject shim into iframe
    const iframe = document.getElementById('pwa-frame');
    if (!iframe) {
        console.error('[Protocol Inspector] iframe not found');
        return;
    }

    iframe.addEventListener('load', function() {
        console.log('[Protocol Inspector] iframe loaded, injecting shim');

        fetch('/demo-shim.js')
            .then(function(r) {
                if (!r.ok) {
                    throw new Error('Failed to load demo-shim.js: ' + r.status);
                }
                return r.text();
            })
            .then(function(code) {
                const script = iframe.contentDocument.createElement('script');
                script.textContent = code;
                iframe.contentDocument.head.appendChild(script);
                console.log('[Protocol Inspector] Shim injected successfully');
            })
            .catch(function(err) {
                console.error('[Protocol Inspector] Failed to inject shim:', err);
            });
    });
});
