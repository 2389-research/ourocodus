/**
 * Ourocodus PWA - WebSocket Connection Manager
 * Handles connection lifecycle, error handling, and reconnection strategies
 */

class RelayConnection {
    constructor() {
        this.ws = null;
        this.isConnected = false;
        this.shouldReconnect = true;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.reconnectDelay = 1000; // Start with 1 second
        this.maxReconnectDelay = 30000; // Max 30 seconds
        this.sessionId = null;
        this.reconnectTimeout = null;

        // Get WebSocket URL (same host as HTTP)
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.host;
        this.wsUrl = `${protocol}//${host}/ws`;

        console.log('[RelayConnection] Initialized with URL:', this.wsUrl);
    }

    /**
     * Connect to the relay WebSocket server
     */
    connect() {
        if (this.ws && (this.ws.readyState === WebSocket.CONNECTING || this.ws.readyState === WebSocket.OPEN)) {
            console.log('[RelayConnection] Already connected or connecting');
            return;
        }

        console.log('[RelayConnection] Connecting to relay...');
        this.updateConnectionStatus('connecting', 'Connecting...');

        try {
            this.ws = new WebSocket(this.wsUrl);
            this.setupEventHandlers();
        } catch (error) {
            console.error('[RelayConnection] Failed to create WebSocket:', error);
            this.updateConnectionStatus('disconnected', 'Connection failed');
            this.scheduleReconnect();
        }
    }

    /**
     * Setup WebSocket event handlers
     */
    setupEventHandlers() {
        this.ws.onopen = (event) => {
            console.log('[RelayConnection] Connected to relay', event);
            this.isConnected = true;
            this.reconnectAttempts = 0;
            this.reconnectDelay = 1000;
            this.updateConnectionStatus('connected', 'Connected');
        };

        this.ws.onmessage = (event) => {
            console.log('[RelayConnection] Message received:', event.data);
            this.handleMessage(event.data);
        };

        this.ws.onerror = (event) => {
            console.error('[RelayConnection] WebSocket error:', event);
            this.updateConnectionStatus('error', 'Connection error');
        };

        this.ws.onclose = (event) => {
            console.log('[RelayConnection] Connection closed:', {
                code: event.code,
                reason: event.reason,
                wasClean: event.wasClean
            });
            this.isConnected = false;
            this.updateConnectionStatus('disconnected', 'Disconnected');

            if (this.shouldReconnect) {
                this.scheduleReconnect();
            }
        };
    }

    /**
     * Handle incoming WebSocket messages
     */
    handleMessage(data) {
        try {
            const message = JSON.parse(data);
            console.log('[RelayConnection] Parsed message:', message);

            switch (message.type) {
                case 'connection:established':
                    console.log('[RelayConnection] Handshake received:', message);
                    break;

                case 'session:created':
                    console.log('[RelayConnection] Session created:', message.session_id);
                    this.sessionId = message.session_id;
                    this.handleSessionCreated(message);
                    break;

                case 'error':
                    console.error('[RelayConnection] Server error:', message);
                    this.handleError(message);
                    break;

                default:
                    console.log('[RelayConnection] Unknown message type:', message.type);
            }
        } catch (error) {
            console.error('[RelayConnection] Failed to parse message:', error, data);
        }
    }

    /**
     * Handle session:created response
     */
    handleSessionCreated(message) {
        const sessionInfoCard = document.getElementById('sessionInfo');
        const sessionIdEl = document.getElementById('sessionId');
        const sessionStatusEl = document.getElementById('sessionStatus');

        sessionInfoCard.style.display = 'block';
        sessionIdEl.textContent = message.session_id;
        sessionStatusEl.textContent = 'Active';

        console.log('[RelayConnection] Session UI updated');
    }

    /**
     * Handle error messages from server
     */
    handleError(message) {
        const sessionStatusEl = document.getElementById('sessionStatus');
        if (sessionStatusEl) {
            sessionStatusEl.textContent = `Error: ${message.message || 'Unknown error'}`;
        }
    }

    /**
     * Send a message to the relay
     */
    sendMessage(message) {
        if (!this.isConnected || !this.ws || this.ws.readyState !== WebSocket.OPEN) {
            console.error('[RelayConnection] Cannot send message: not connected');
            return false;
        }

        try {
            const payload = JSON.stringify(message);
            console.log('[RelayConnection] Sending message:', payload);
            this.ws.send(payload);
            return true;
        } catch (error) {
            console.error('[RelayConnection] Failed to send message:', error);
            return false;
        }
    }

    /**
     * Create a new session
     */
    createSession() {
        const message = {
            type: 'session:create'
        };

        if (!this.sendMessage(message)) {
            console.error('[RelayConnection] Failed to send session:create message');
            const sessionStatusEl = document.getElementById('sessionStatus');
            if (sessionStatusEl) {
                sessionStatusEl.textContent = 'Failed to create session';
            }
        }
    }

    /**
     * Schedule a reconnection attempt with exponential backoff
     */
    scheduleReconnect() {
        if (!this.shouldReconnect) {
            console.log('[RelayConnection] Reconnection disabled');
            return;
        }

        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('[RelayConnection] Max reconnection attempts reached');
            this.updateConnectionStatus('failed', 'Connection failed');
            return;
        }

        this.reconnectAttempts++;
        const delay = Math.min(this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1), this.maxReconnectDelay);

        console.log(`[RelayConnection] Scheduling reconnect attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts} in ${delay}ms`);
        this.updateConnectionStatus('reconnecting', `Reconnecting in ${Math.round(delay / 1000)}s...`);

        this.reconnectTimeout = setTimeout(() => {
            console.log('[RelayConnection] Attempting to reconnect...');
            this.connect();
        }, delay);
    }

    /**
     * Update the connection status UI
     */
    updateConnectionStatus(state, text) {
        const statusIndicator = document.getElementById('statusIndicator');
        const statusText = document.getElementById('statusText');

        if (!statusIndicator || !statusText) {
            return;
        }

        // Update text
        statusText.textContent = text;

        // Update indicator styling
        statusIndicator.classList.remove('connected');
        if (state === 'connected') {
            statusIndicator.classList.add('connected');
        }

        console.log('[RelayConnection] Status updated:', state, text);
    }

    /**
     * Disconnect from the relay
     */
    disconnect() {
        console.log('[RelayConnection] Disconnecting...');
        this.shouldReconnect = false;

        if (this.reconnectTimeout) {
            clearTimeout(this.reconnectTimeout);
            this.reconnectTimeout = null;
        }

        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }

        this.isConnected = false;
        this.updateConnectionStatus('disconnected', 'Disconnected');
    }
}

/**
 * Application initialization
 */
class App {
    constructor() {
        this.connection = new RelayConnection();
        this.init();
    }

    init() {
        console.log('[App] Initializing Ourocodus PWA');

        // Setup New Project button handler
        const newProjectBtn = document.getElementById('newProjectBtn');
        if (newProjectBtn) {
            newProjectBtn.addEventListener('click', () => {
                console.log('[App] New Project button clicked');
                this.handleNewProject();
            });
        }

        console.log('[App] Initialization complete');
    }

    handleNewProject() {
        const btn = document.getElementById('newProjectBtn');

        if (!this.connection.isConnected) {
            console.log('[App] Not connected, initiating connection...');
            btn.disabled = true;
            btn.textContent = 'Connecting...';

            // Connect to relay
            this.connection.connect();

            // Wait for connection, then create session
            const checkConnection = setInterval(() => {
                if (this.connection.isConnected) {
                    clearInterval(checkConnection);
                    console.log('[App] Connected, creating session...');
                    this.connection.createSession();
                    btn.disabled = false;
                    btn.innerHTML = '<span class="btn-icon">+</span> New Project';
                }
            }, 100);

            // Timeout after 10 seconds
            setTimeout(() => {
                clearInterval(checkConnection);
                if (!this.connection.isConnected) {
                    console.error('[App] Connection timeout');
                    btn.disabled = false;
                    btn.innerHTML = '<span class="btn-icon">+</span> New Project';
                }
            }, 10000);
        } else {
            console.log('[App] Already connected, creating session...');
            this.connection.createSession();
        }
    }
}

// Initialize app when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        window.app = new App();
    });
} else {
    window.app = new App();
}
