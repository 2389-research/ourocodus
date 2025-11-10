/**
 * Ourocodus PWA - WebSocket Connection Manager
 * Handles connection lifecycle, error handling, and reconnection strategies
 */

/**
 * Modal Manager - Handles modal display and interaction
 */
class ModalManager {
    constructor(modalId) {
        this.logger = new Logger('ModalManager');
        this.modal = document.getElementById(modalId);
        this.overlay = null;
        this.confirmBtn = null;
        this.cancelBtn = null;
        this.onConfirm = null;
        this.onCancel = null;

        if (!this.modal) {
            this.logger.error('Modal not found:', modalId);
            return;
        }

        // Find overlay and buttons
        this.overlay = this.modal.querySelector('.modal-overlay');
        this.confirmBtn = this.modal.querySelector('[id^="confirm"]');
        this.cancelBtn = this.modal.querySelector('[id^="cancel"]');

        // Setup event listeners
        this.setupListeners();
    }

    setupListeners() {
        // Confirm button
        if (this.confirmBtn) {
            this.confirmBtn.addEventListener('click', () => {
                if (this.onConfirm) {
                    this.onConfirm();
                }
                this.hide();
            });
        }

        // Cancel button
        if (this.cancelBtn) {
            this.cancelBtn.addEventListener('click', () => {
                if (this.onCancel) {
                    this.onCancel();
                }
                this.hide();
            });
        }

        // Overlay click
        if (this.overlay) {
            this.overlay.addEventListener('click', () => {
                if (this.onCancel) {
                    this.onCancel();
                }
                this.hide();
            });
        }
    }

    show(options = {}) {
        if (!this.modal) return;

        // Update callbacks
        this.onConfirm = options.onConfirm || null;
        this.onCancel = options.onCancel || null;

        // Update dynamic content if provided
        if (options.updateContent) {
            options.updateContent(this.modal);
        }

        // Show modal
        this.modal.style.display = 'flex';
    }

    hide() {
        if (!this.modal) return;
        this.modal.style.display = 'none';
    }
}

class RelayConnection {
    constructor() {
        this.logger = new Logger('RelayConnection');
        this.ws = null;
        this.isConnected = false;
        this.shouldReconnect = true;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.reconnectDelay = 1000; // Start with 1 second
        this.maxReconnectDelay = 30000; // Max 30 seconds
        this.userSessionId = null;
        this.reconnectTimeout = null;

        // Get WebSocket URL (same host as HTTP)
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.host;
        this.wsUrl = `${protocol}//${host}/ws`;

        this.logger.info('Initialized with URL:', this.wsUrl);
    }

    /**
     * Connect to the relay WebSocket server
     */
    connect() {
        if (this.ws && (this.ws.readyState === WebSocket.CONNECTING || this.ws.readyState === WebSocket.OPEN)) {
            this.logger.debug('Already connected or connecting');
            return;
        }

        this.logger.info('Connecting to relay...');
        this.updateConnectionStatus('connecting', 'Connecting...');

        try {
            this.ws = new WebSocket(this.wsUrl);
            this.setupEventHandlers();
        } catch (error) {
            this.logger.error('Failed to create WebSocket:', error);
            this.updateConnectionStatus('disconnected', 'Connection failed');
            this.scheduleReconnect();
        }
    }

    /**
     * Setup WebSocket event handlers
     */
    setupEventHandlers() {
        this.ws.onopen = (event) => {
            this.logger.info('Connected to relay', event);
            this.isConnected = true;
            this.reconnectAttempts = 0;
            this.reconnectDelay = 1000;
            this.updateConnectionStatus('connected', 'Connected');
        };

        this.ws.onmessage = (event) => {
            this.logger.debug('Message received:', event.data);
            this.handleMessage(event.data);
        };

        this.ws.onerror = (event) => {
            this.logger.error('WebSocket error:', event);
            this.updateConnectionStatus('error', 'Connection error');
        };

        this.ws.onclose = (event) => {
            this.logger.info('Connection closed:', {
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
            this.logger.debug('Parsed message:', message);

            switch (message.type) {
                case 'connection:established':
                    this.logger.info('Handshake received:', message);
                    break;

                case 'session:created':
                    this.logger.info('Session created:', message.userSessionId);
                    this.userSessionId = message.userSessionId;
                    this.handleSessionCreated(message);
                    break;

                case 'agent:ready':
                    this.logger.info('Agent ready:', message);
                    this.handleAgentReady(message);
                    break;

                case 'agent:response':
                    this.logger.debug('Agent response:', message);
                    this.handleAgentResponse(message);
                    break;

                case 'error':
                    this.logger.error('Server error:', message);
                    this.handleError(message);
                    break;

                default:
                    this.logger.warn('Unknown message type:', message.type);
            }
        } catch (error) {
            this.logger.error('Failed to parse message:', error, data);
        }
    }

    /**
     * Handle session:created response
     */
    handleSessionCreated(message) {
        const sessionInfoCard = document.getElementById('sessionInfo');
        const sessionIdEl = document.getElementById('userSessionId');
        const sessionStatusEl = document.getElementById('sessionStatus');
        const welcomeCard = document.getElementById('welcomeCard');

        if (!sessionInfoCard || !sessionIdEl || !sessionStatusEl) {
            this.logger.error('Session UI elements not found');
            return;
        }

        // Hide welcome card, show session info
        if (welcomeCard) {
            welcomeCard.style.display = 'none';
        }

        sessionInfoCard.style.display = 'block';
        sessionIdEl.textContent = message.userSessionId;
        sessionStatusEl.textContent = 'Active';

        this.logger.debug('Session UI updated');
    }

    /**
     * Handle agent:ready response
     */
    handleAgentReady(message) {
        const agentCard = document.getElementById('agentCard');
        const agentRoleDisplay = document.getElementById('agentRoleDisplay');
        const agentSpawnSection = document.getElementById('agentSpawnSection');

        if (!agentCard || !agentRoleDisplay) {
            this.logger.error('Agent card elements not found');
            return;
        }

        // Hide spawn section, show agent card
        if (agentSpawnSection) {
            agentSpawnSection.style.display = 'none';
        }

        agentCard.style.display = 'block';
        agentRoleDisplay.textContent = message.agentId;
        this.currentAgentRole = message.agentId;

        // Add welcome message from agent
        this.displayMessage('agent', `Hi! I'm ${message.agentId}. I'm here to help. Send me a message to get started!`);

        this.logger.debug('Agent UI displayed for agentId:', message.agentId);
    }

    /**
     * Handle agent:response message
     */
    handleAgentResponse(message) {
        this.displayMessage('agent', message.content);
        this.logger.debug('Agent response displayed');
    }

    /**
     * Display a message in the message history
     */
    displayMessage(sender, content) {
        const messageHistory = document.getElementById('messageHistory');
        if (!messageHistory) {
            this.logger.error('Message history element not found');
            return;
        }

        const messageEl = document.createElement('div');
        messageEl.className = `message message-${sender}`;

        const senderEl = document.createElement('div');
        senderEl.className = 'message-sender';
        senderEl.textContent = sender;

        const contentEl = document.createElement('div');
        contentEl.className = 'message-content';
        contentEl.textContent = content;

        const timeEl = document.createElement('div');
        timeEl.className = 'message-time';
        timeEl.textContent = new Date().toLocaleTimeString();

        messageEl.appendChild(senderEl);
        messageEl.appendChild(contentEl);
        messageEl.appendChild(timeEl);

        messageHistory.appendChild(messageEl);
        messageHistory.scrollTop = messageHistory.scrollHeight;
    }

    /**
     * Handle error messages from server
     */
    handleError(message) {
        this.logger.error('Server error:', message);

        // Extract error details
        const errorCode = message.error?.code || message.code || 'UNKNOWN_ERROR';
        const errorMessage = message.error?.message || message.message || 'Unknown error occurred';
        const recoverable = message.error?.recoverable !== false;

        // Show error in a prominent way
        this.showErrorNotification(errorCode, errorMessage, recoverable);
    }

    /**
     * Show error notification
     */
    showErrorNotification(code, message, recoverable) {
        // Create error notification element
        const errorDiv = document.createElement('div');
        errorDiv.className = 'error-notification';
        errorDiv.innerHTML = `
            <div class="error-header">
                <span class="error-icon">⚠️</span>
                <span class="error-code">${code}</span>
                <button class="error-close">&times;</button>
            </div>
            <div class="error-message">${message}</div>
            ${recoverable ? '<div class="error-hint">You can retry this operation</div>' : '<div class="error-hint error-fatal">This error is not recoverable</div>'}
        `;

        // Add to page
        document.body.appendChild(errorDiv);

        // Auto-dismiss after 10 seconds
        setTimeout(() => {
            errorDiv.classList.add('error-fade-out');
            setTimeout(() => errorDiv.remove(), 300);
        }, 10000);

        // Close button
        errorDiv.querySelector('.error-close').addEventListener('click', () => {
            errorDiv.classList.add('error-fade-out');
            setTimeout(() => errorDiv.remove(), 300);
        });
    }

    /**
     * Send a message to the relay
     */
    sendMessage(message) {
        this.logger.debug('sendMessage debug - isConnected:', this.isConnected, 'ws:', !!this.ws, 'readyState:', this.ws?.readyState, 'OPEN:', WebSocket.OPEN);
        if (!this.isConnected || !this.ws || this.ws.readyState !== WebSocket.OPEN) {
            this.logger.error('Cannot send message: not connected');
            return false;
        }

        try {
            const payload = JSON.stringify(message);
            this.logger.debug('Sending message:', payload);
            this.ws.send(payload);
            return true;
        } catch (error) {
            this.logger.error('Failed to send message:', error);
            return false;
        }
    }

    /**
     * Create a new session
     */
    createSession() {
        const message = {
            type: 'session:create',
            version: '1.0'
        };

        if (!this.sendMessage(message)) {
            this.logger.error('Failed to send session:create message');
            const sessionStatusEl = document.getElementById('sessionStatus');
            if (sessionStatusEl) {
                sessionStatusEl.textContent = 'Failed to create session';
            }
        }
    }

    /**
     * Spawn an agent in the current session
     */
    spawnAgent(role, workspace) {
        if (!this.userSessionId) {
            this.logger.error('Cannot spawn agent: no session');
            return false;
        }

        const message = {
            type: 'agent:spawn',
            version: '1.0',
            userSessionId: this.userSessionId,
            agentId: role,
            workspace: workspace
        };

        this.logger.info('Spawning agent:', message);
        return this.sendMessage(message);
    }

    /**
     * Send a message to an agent
     */
    sendAgentMessage(role, content) {
        if (!this.userSessionId) {
            this.logger.error('Cannot send message: no session');
            return false;
        }

        const message = {
            type: 'agent:message',
            version: '1.0',
            userSessionId: this.userSessionId,
            agentId: role,
            content: content
        };

        // Display user message immediately
        this.displayMessage('user', content);

        this.logger.debug('Sending agent message:', message);
        return this.sendMessage(message);
    }

    /**
     * End the current session (Phase 3 feature)
     * Sends session:end message to server for graceful shutdown
     */
    endSession() {
        if (!this.userSessionId) {
            this.logger.error('Cannot end session: no session');
            return false;
        }

        const message = {
            type: 'session:end',
            version: '1.0',
            userSessionId: this.userSessionId
        };

        this.logger.info('Ending session:', message);
        return this.sendMessage(message);
    }

    /**
     * Terminate a specific agent (Phase 3 feature)
     * Sends agent:terminate message to server
     */
    terminateAgent(role) {
        if (!this.userSessionId) {
            this.logger.error('Cannot terminate agent: no session');
            return false;
        }

        const message = {
            type: 'agent:terminate',
            version: '1.0',
            userSessionId: this.userSessionId,
            agentId: role
        };

        this.logger.info('Terminating agent:', message);
        return this.sendMessage(message);
    }

    /**
     * Schedule a reconnection attempt with exponential backoff
     */
    scheduleReconnect() {
        if (!this.shouldReconnect) {
            this.logger.debug('Reconnection disabled');
            return;
        }

        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            this.logger.error('Max reconnection attempts reached');
            this.updateConnectionStatus('failed', 'Connection failed');
            return;
        }

        this.reconnectAttempts++;
        const delay = Math.min(this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1), this.maxReconnectDelay);

        this.logger.info(`Scheduling reconnect attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts} in ${delay}ms`);
        this.updateConnectionStatus('reconnecting', `Reconnecting in ${Math.round(delay / 1000)}s...`);

        this.reconnectTimeout = setTimeout(() => {
            this.logger.info('Attempting to reconnect...');
            this.connect();
        }, delay);
    }

    /**
     * Update the connection status UI
     */
    updateConnectionStatus(state, text) {
        const statusIndicator = document.getElementById('statusIndicator');
        const statusText = document.getElementById('statusText');
        const disconnectBtn = document.getElementById('disconnectBtn');

        if (!statusIndicator || !statusText) {
            return;
        }

        // Update text
        statusText.textContent = text;

        // Update indicator styling - remove all state classes first
        statusIndicator.classList.remove('connected', 'connecting');

        if (state === 'connected') {
            statusIndicator.classList.add('connected');
        } else if (state === 'connecting' || state === 'reconnecting') {
            statusIndicator.classList.add('connecting');
        }

        // Show/hide disconnect button
        if (disconnectBtn) {
            disconnectBtn.style.display = (state === 'connected') ? 'flex' : 'none';
        }

        this.logger.debug('Status updated:', state, text);
    }

    /**
     * Disconnect from the relay
     * @param {number} code - WebSocket close code (1000 = normal, 1001 = going away)
     * @param {string} reason - Human-readable close reason
     */
    disconnect(code = 1000, reason = 'Client disconnected') {
        this.logger.info('Disconnecting...', code, reason);
        this.shouldReconnect = false;

        if (this.reconnectTimeout) {
            clearTimeout(this.reconnectTimeout);
            this.reconnectTimeout = null;
        }

        if (this.ws) {
            // Only close if WebSocket is in OPEN or CONNECTING state
            if (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING) {
                try {
                    // Some browsers don't support reason parameter, so just use code
                    this.ws.close(code);
                } catch (e) {
                    this.logger.error('Error closing WebSocket:', e);
                }
            }
            this.ws = null;
        }

        this.isConnected = false;
        this.userSessionId = null;
        this.currentAgentRole = null;
        this.updateConnectionStatus('disconnected', 'Disconnected');
    }
}

/**
 * Application initialization
 */
class App {
    constructor() {
        this.logger = new Logger('App');
        this.connection = new RelayConnection();
        this.connectionCheckInterval = null;
        this.connectionCheckTimeout = null;
        this.isConnecting = false;

        // Initialize modal managers
        this.disconnectModal = new ModalManager('disconnectModal');
        this.endSessionModal = new ModalManager('endSessionModal');
        this.terminateAgentModal = new ModalManager('terminateAgentModal');

        this.init();
    }

    init() {
        this.logger.info('Initializing Ourocodus PWA');

        // Setup New Project button handler
        const newProjectBtn = document.getElementById('newProjectBtn');
        if (newProjectBtn) {
            newProjectBtn.addEventListener('click', () => {
                this.logger.debug('New Project button clicked');
                this.handleNewProject();
            });
        }

        // Setup Spawn Agent button handler
        const spawnAgentBtn = document.getElementById('spawnAgentBtn');
        if (spawnAgentBtn) {
            spawnAgentBtn.addEventListener('click', () => {
                this.logger.debug('Spawn Agent button clicked');
                this.handleSpawnAgent();
            });
        }

        // Setup Send Message button handler
        const sendMessageBtn = document.getElementById('sendMessageBtn');
        if (sendMessageBtn) {
            sendMessageBtn.addEventListener('click', () => {
                this.logger.debug('Send Message button clicked');
                this.handleSendMessage();
            });
        }

        // Setup Enter key in message input
        const messageInput = document.getElementById('messageInput');
        if (messageInput) {
            messageInput.addEventListener('keydown', (e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    this.handleSendMessage();
                }
            });
        }

        // Setup Disconnect button handler (in header)
        const disconnectBtn = document.getElementById('disconnectBtn');
        if (disconnectBtn) {
            disconnectBtn.addEventListener('click', () => {
                this.logger.debug('Disconnect button clicked');
                this.disconnectModal.show({
                    onConfirm: () => {
                        this.logger.debug('Disconnect confirmed');
                        this.handleDisconnect();
                    },
                    onCancel: () => {
                        this.logger.debug('Disconnect cancelled');
                    }
                });
            });
        }

        // Setup End Session button handler (in session card)
        const endSessionBtn = document.getElementById('endSessionBtn');
        if (endSessionBtn) {
            endSessionBtn.addEventListener('click', () => {
                this.logger.debug('End Session button clicked');
                this.endSessionModal.show({
                    onConfirm: () => {
                        this.logger.debug('End Session confirmed');
                        this.handleEndSession();
                    },
                    onCancel: () => {
                        this.logger.debug('End Session cancelled');
                    }
                });
            });
        }

        // Setup Terminate Agent button handler (in agent card)
        const terminateAgentBtn = document.getElementById('terminateAgentBtn');
        if (terminateAgentBtn) {
            terminateAgentBtn.addEventListener('click', () => {
                this.logger.debug('Terminate Agent button clicked');
                this.terminateAgentModal.show({
                    onConfirm: () => {
                        this.logger.debug('Terminate Agent confirmed');
                        this.handleTerminateAgent();
                    },
                    onCancel: () => {
                        this.logger.debug('Terminate Agent cancelled');
                    },
                    updateContent: (modal) => {
                        // Update role display in modal
                        const roleDisplay = modal.querySelector('#terminateAgentRole');
                        if (roleDisplay) {
                            roleDisplay.textContent = this.connection.currentAgentRole || '-';
                        }
                    }
                });
            });
        }

        this.logger.info('Initialization complete');
    }

    handleDisconnect() {
        this.logger.info('Disconnecting from relay...');

        // Disable disconnect button to prevent double-clicks
        const disconnectBtn = document.getElementById('disconnectBtn');
        if (disconnectBtn) {
            disconnectBtn.disabled = true;
        }

        // Disconnect WebSocket with code 1000 (normal closure)
        // Note: Server will still cleanup session/agents on disconnect
        this.connection.disconnect(1000, 'User disconnected');

        this.logger.info('Disconnect complete');

        // Re-enable after a delay (button will be hidden anyway once disconnected)
        setTimeout(() => {
            if (disconnectBtn) {
                disconnectBtn.disabled = false;
            }
        }, 1000);
    }

    handleEndSession() {
        this.logger.info('Ending session and resetting...');

        // Disable end session button to prevent double-clicks
        const endSessionBtn = document.getElementById('endSessionBtn');
        if (endSessionBtn) {
            endSessionBtn.disabled = true;
        }

        // Try to send session:end message (Phase 3 feature)
        // If server doesn't support it yet, will get error but we still disconnect
        if (this.connection.endSession()) {
            this.logger.info('session:end message sent');
        } else {
            this.logger.warn('Could not send session:end (may not be connected)');
        }

        // Give server a moment to process, then disconnect
        setTimeout(() => {
            // Disconnect from relay with code 1001 (going away)
            this.connection.disconnect(1001, 'Session ended by user');

            // Reset all UI state
            this.resetUI();

            this.logger.info('End session complete');
        }, 100);
    }

    handleTerminateAgent() {
        this.logger.info('Terminating agent...');

        const role = this.connection.currentAgentRole;
        if (!role) {
            this.logger.error('No agent to terminate');
            return;
        }

        // Disable terminate button to prevent double-clicks
        const terminateAgentBtn = document.getElementById('terminateAgentBtn');
        if (terminateAgentBtn) {
            terminateAgentBtn.disabled = true;
        }

        // Try to send agent:terminate message (Phase 3 feature)
        // If server doesn't support it yet, will get error but we still update UI
        if (this.connection.terminateAgent(role)) {
            this.logger.info('agent:terminate message sent for agentId:', role);
        } else {
            this.logger.warn('Could not send agent:terminate (may not be connected)');
        }

        // Update UI to hide agent card and show spawn section
        setTimeout(() => {
            const agentCard = document.getElementById('agentCard');
            if (agentCard) {
                agentCard.style.display = 'none';
            }

            const agentSpawnSection = document.getElementById('agentSpawnSection');
            if (agentSpawnSection) {
                agentSpawnSection.style.display = 'block';
            }

            // Clear message history
            const messageHistory = document.getElementById('messageHistory');
            if (messageHistory) {
                messageHistory.innerHTML = '';
            }

            // Clear message input
            const messageInput = document.getElementById('messageInput');
            if (messageInput) {
                messageInput.value = '';
            }

            // Reset spawn button
            const spawnAgentBtn = document.getElementById('spawnAgentBtn');
            if (spawnAgentBtn) {
                spawnAgentBtn.disabled = false;
                spawnAgentBtn.innerHTML = '<span class="btn-icon">🤖</span> Spawn Agent';
            }

            // Re-enable terminate button (will be hidden anyway)
            if (terminateAgentBtn) {
                terminateAgentBtn.disabled = false;
            }

            // Clear current agent role
            this.connection.currentAgentRole = null;

            this.logger.info('Agent terminated and UI updated');
        }, 100);
    }

    resetUI() {
        // Show welcome card
        const welcomeCard = document.getElementById('welcomeCard');
        if (welcomeCard) {
            welcomeCard.style.display = 'block';
        }

        // Hide session card
        const sessionInfo = document.getElementById('sessionInfo');
        if (sessionInfo) {
            sessionInfo.style.display = 'none';
        }

        // Hide agent card
        const agentCard = document.getElementById('agentCard');
        if (agentCard) {
            agentCard.style.display = 'none';
        }

        // Show spawn section (for next session)
        const agentSpawnSection = document.getElementById('agentSpawnSection');
        if (agentSpawnSection) {
            agentSpawnSection.style.display = 'block';
        }

        // Clear message history
        const messageHistory = document.getElementById('messageHistory');
        if (messageHistory) {
            messageHistory.innerHTML = '';
        }

        // Clear message input
        const messageInput = document.getElementById('messageInput');
        if (messageInput) {
            messageInput.value = '';
        }

        // Reset button states
        const newProjectBtn = document.getElementById('newProjectBtn');
        if (newProjectBtn) {
            newProjectBtn.disabled = false;
            newProjectBtn.innerHTML = '<span class="btn-icon">+</span> New Project';
        }

        const spawnAgentBtn = document.getElementById('spawnAgentBtn');
        if (spawnAgentBtn) {
            spawnAgentBtn.disabled = false;
            spawnAgentBtn.innerHTML = '<span class="btn-icon">🤖</span> Spawn Agent';
        }

        // Reset connection state
        this.isConnecting = false;
        if (this.connectionCheckInterval) {
            clearInterval(this.connectionCheckInterval);
            this.connectionCheckInterval = null;
        }
        if (this.connectionCheckTimeout) {
            clearTimeout(this.connectionCheckTimeout);
            this.connectionCheckTimeout = null;
        }

        this.logger.debug('UI reset complete');
    }

    handleNewProject() {
        const btn = document.getElementById('newProjectBtn');

        // Reentrancy guard: prevent multiple simultaneous connection attempts
        if (this.isConnecting) {
            this.logger.debug('Connection attempt already in progress, ignoring click');
            return;
        }

        if (!this.connection.isConnected) {
            this.logger.info('Not connected, initiating connection...');
            this.isConnecting = true;
            btn.disabled = true;
            btn.textContent = 'Connecting...';

            // Connect to relay
            this.connection.connect();

            // Wait for connection, then create session
            this.connectionCheckInterval = setInterval(() => {
                if (this.connection.isConnected) {
                    // Clean up interval and timeout
                    clearInterval(this.connectionCheckInterval);
                    clearTimeout(this.connectionCheckTimeout);
                    this.connectionCheckInterval = null;
                    this.connectionCheckTimeout = null;
                    this.isConnecting = false;

                    this.logger.info('Connected, creating session...');
                    this.connection.createSession();
                    btn.disabled = false;
                    btn.innerHTML = '<span class="btn-icon">+</span> New Project';
                }
            }, 100);

            // Timeout after 10 seconds
            this.connectionCheckTimeout = setTimeout(() => {
                // Clean up interval and reset state
                clearInterval(this.connectionCheckInterval);
                this.connectionCheckInterval = null;
                this.connectionCheckTimeout = null;
                this.isConnecting = false;

                if (!this.connection.isConnected) {
                    this.logger.error('Connection timeout');
                    btn.disabled = false;
                    btn.innerHTML = '<span class="btn-icon">+</span> New Project';
                }
            }, 10000);
        } else {
            this.logger.info('Already connected, creating session...');
            this.connection.createSession();
        }
    }

    handleSpawnAgent() {
        const roleInput = document.getElementById('agentRole');
        const workspaceInput = document.getElementById('agentWorkspace');
        const btn = document.getElementById('spawnAgentBtn');

        if (!roleInput || !workspaceInput) {
            this.logger.error('Agent spawn inputs not found');
            return;
        }

        const role = roleInput.value.trim();
        const workspace = workspaceInput.value.trim();

        if (!role || !workspace) {
            alert('Please provide both agent role and workspace');
            return;
        }

        btn.disabled = true;
        btn.textContent = 'Spawning...';

        if (this.connection.spawnAgent(role, workspace)) {
            this.logger.info('Agent spawn initiated');
            // Button will be re-enabled when agent:ready is received
        } else {
            this.logger.error('Failed to spawn agent');
            btn.disabled = false;
            btn.innerHTML = '<span class="btn-icon">🤖</span> Spawn Agent';
        }
    }

    handleSendMessage() {
        const messageInput = document.getElementById('messageInput');
        const btn = document.getElementById('sendMessageBtn');

        if (!messageInput) {
            this.logger.error('Message input not found');
            return;
        }

        const content = messageInput.value.trim();
        if (!content) {
            return;
        }

        const role = this.connection.currentAgentRole;
        if (!role) {
            this.logger.error('No active agent');
            return;
        }

        btn.disabled = true;
        messageInput.disabled = true;

        if (this.connection.sendAgentMessage(role, content)) {
            this.logger.debug('Message sent');
            messageInput.value = '';
            // Re-enable after a short delay
            setTimeout(() => {
                btn.disabled = false;
                messageInput.disabled = false;
                messageInput.focus();
            }, 500);
        } else {
            this.logger.error('Failed to send message');
            btn.disabled = false;
            messageInput.disabled = false;
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
