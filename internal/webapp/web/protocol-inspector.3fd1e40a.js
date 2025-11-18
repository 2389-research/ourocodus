// Protocol Inspector Logic
// Receives WebSocket events from the WebSocket interceptor and updates the UI

class ProtocolInspector {
    constructor() {
        this.messageList = document.getElementById('message-list');
        this.stateIndicator = document.getElementById('state-indicator');
        this.sessionPanel = document.getElementById('session-panel');
        this.currentState = 'disconnected';
        this.userSessionId = null;
        this.reconnectAttempts = 0;
        this.messages = []; // Store messages for search/export
        this.searchQuery = '';

        if (!this.messageList || !this.stateIndicator || !this.sessionPanel) {
            console.error('[Protocol Inspector] Required DOM elements not found');
            return;
        }

        // Wire up inspector controls
        this.setupControls();

        window.addEventListener('message', this.handleMessage.bind(this));
        console.log('[Protocol Inspector] Initialized');
    }

    setupControls() {
        // Search functionality
        const searchInput = document.getElementById('inspectorSearch');
        if (searchInput) {
            let searchTimeout;
            searchInput.addEventListener('input', (e) => {
                clearTimeout(searchTimeout);
                searchTimeout = setTimeout(() => {
                    this.searchQuery = e.target.value.toLowerCase();
                    this.filterMessages();
                }, 150);
            });
        }

        // Export functionality
        const exportBtn = document.getElementById('inspectorExport');
        if (exportBtn) {
            exportBtn.addEventListener('click', () => {
                this.exportJSON();
            });
        }

        // Clear functionality
        const clearBtn = document.getElementById('inspectorClear');
        if (clearBtn) {
            clearBtn.addEventListener('click', () => {
                this.clearMessages();
            });
        }
    }

    filterMessages() {
        const messageElements = this.messageList.querySelectorAll('.message-entry');

        messageElements.forEach((el) => {
            if (!this.searchQuery) {
                el.style.display = '';
                return;
            }

            const timestampStr = el.getAttribute('data-timestamp');
            if (!timestampStr) {
                el.style.display = '';
                return;
            }

            const timestamp = Number(timestampStr);
            const message = this.messages.find(msg => msg.timestamp === timestamp);
            if (!message) {
                el.style.display = '';
                return;
            }

            // Search in raw data and type
            const matchesContent = message.data.toLowerCase().includes(this.searchQuery);
            const matchesType = message.type.toLowerCase().includes(this.searchQuery);

            el.style.display = (matchesContent || matchesType) ? '' : 'none';
        });
    }

    exportJSON() {
        const data = this.messages.map(msg => ({
            timestamp: new Date(msg.timestamp).toISOString(),
            type: msg.type,
            direction: msg.direction,
            data: msg.data
        }));

        const json = JSON.stringify(data, null, 2);
        const blob = new Blob([json], { type: 'application/json' });
        const url = URL.createObjectURL(blob);

        const a = document.createElement('a');
        a.href = url;
        a.download = `ourocodus-protocol-${Date.now()}.json`;
        a.click();

        URL.revokeObjectURL(url);
        console.log('[Protocol Inspector] Exported', this.messages.length, 'messages');
    }

    clearMessages() {
        if (!confirm('Clear all protocol messages?')) {
            return;
        }

        this.messages = [];
        this.searchQuery = '';
        this.messageList.innerHTML = '';

        // Clear search input
        const searchInput = document.getElementById('inspectorSearch');
        if (searchInput) {
            searchInput.value = '';
        }

        console.log('[Protocol Inspector] Messages cleared');
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

            case 'ws:close': {
                this.updateState('disconnected');
                const reason = msg.reason || 'No reason provided';
                this.appendMessage('Connection closed: ' + msg.code + ' (' + reason + ')', 'system', msg.timestamp);
                break;
            }

            case 'ws:error':
                this.updateState('error');
                this.appendMessage('Connection error occurred', 'error', msg.timestamp);
                break;
        }
    }

    appendMessage(data, type, timestamp) {
        // Store message for search/export
        this.messages.push({
            data: data,
            type: type,
            direction: type === 'sent' ? 'send' : (type === 'received' ? 'recv' : 'system'),
            timestamp: timestamp
        });

        const entry = document.createElement('div');
        entry.className = 'message-entry message-' + type;
        entry.setAttribute('data-timestamp', timestamp);

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

        fetch('/websocket-interceptor.js')
            .then(function(r) {
                if (!r.ok) {
                    throw new Error('Failed to load websocket-interceptor.js: ' + r.status);
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
