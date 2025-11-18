/**
 * WebSocket relay connection management
 */

import { Logger } from './logger';
import type { AgentState, SessionCreatedMessage, AgentReadyMessage, AgentResponseMessage, ErrorMessage } from './types';

const SPAWN_BUTTON_DEFAULT = '<span class="btn-icon">🤖</span> Spawn Agent';

export class RelayConnection {
    private logger: Logger;
    public ws: WebSocket | null;
    public isConnected: boolean;
    private shouldReconnect: boolean;
    private reconnectAttempts: number;
    private maxReconnectAttempts: number;
    private reconnectDelay: number;
    private maxReconnectDelay: number;
    public userSessionId: string | null;
    private reconnectTimeout: number | null;
    public currentChatRole: string | null;
    public currentAgentRole: string | null; // Currently selected agent for single-agent card view
    private wsUrl: string;
    private pendingSpawnRole: string | null;
    public onAgentReady?: () => void;
    public onError?: () => void;

    // Map-based state for multiple agents: role → { role, status, messages, workspace }
    public agents: Map<string, AgentState>;

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
        this.currentChatRole = null;
        this.currentAgentRole = null;
        this.pendingSpawnRole = null;

        // Map-based state for multiple agents
        this.agents = new Map<string, AgentState>();

        // Get WebSocket URL (same host as HTTP)
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.host;
        this.wsUrl = `${protocol}//${host}/ws`;

        this.logger.info('Initialized with URL:', this.wsUrl);
    }

    /**
     * Connect to the relay WebSocket server
     */
    connect(): void {
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
    setupEventHandlers(): void {
        this.ws!.onopen = (event) => {
            this.logger.info('WebSocket onopen fired - Setting isConnected to true', {
                readyState: this.ws.readyState,
                expectedOpen: WebSocket.OPEN
            });
            this.isConnected = true;
            this.logger.info('Connected to relay - isConnected now:', this.isConnected);
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
            this.resetSessionState();
            this.updateConnectionStatus('disconnected', 'Disconnected');

            if (this.shouldReconnect) {
                this.scheduleReconnect();
            }
        };
    }

    /**
     * Handle incoming WebSocket messages
     */
    handleMessage(data: string): void {
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

                case 'agent:terminated':
                    this.logger.info('Agent terminated:', message);
                    this.handleAgentTerminated(message);
                    break;

                case 'session:ended':
                    this.logger.info('Session ended:', message);
                    this.handleSessionEnded(message);
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
    handleSessionCreated(message: SessionCreatedMessage): void {
        this.logger.info('[UI UPDATE] handleSessionCreated called with:', message);

        const sessionInfoCard = document.getElementById('sessionInfo');
        const sessionIdEl = document.getElementById('userSessionId');
        const sessionStatusEl = document.getElementById('sessionStatus');
        const welcomeCard = document.getElementById('welcomeCard');

        this.logger.info('[UI UPDATE] Elements found:', {
            sessionInfoCard: !!sessionInfoCard,
            sessionIdEl: !!sessionIdEl,
            sessionStatusEl: !!sessionStatusEl,
            welcomeCard: !!welcomeCard
        });

        if (!sessionInfoCard || !sessionIdEl || !sessionStatusEl) {
            this.logger.error('[UI UPDATE] Session UI elements not found - cannot update UI');
            return;
        }

        // Hide welcome card, show session info
        if (welcomeCard) {
            this.logger.info('[UI UPDATE] Hiding welcome card');
            welcomeCard.style.display = 'none';
        }

        this.logger.info('[UI UPDATE] Showing session info card');
        sessionInfoCard.style.display = 'block';
        sessionIdEl.textContent = message.userSessionId;
        sessionStatusEl.textContent = 'Active';

        // Enable controls that depend on an active session
        this.setSessionControls(true);
        this.updateCleanupBanner();

        this.logger.info('[UI UPDATE] Session UI updated successfully');
    }

    /**
     * Handle agent:ready response
     */
    handleAgentReady(message: AgentReadyMessage): void {
        const role = message.agentId;
        const workspace = message.workspace || '';

        // Add agent to Map
        this.agents.set(role, {
            role: role,
            status: 'ready',
            messages: [],
            workspace: workspace
        });

        // Add welcome message to agent's messages
        const agent = this.agents.get(role);
        if (agent) {
            agent.messages.push({
                sender: 'agent',
                content: `Hi! I'm ${role}. I'm here to help. Send me a message to get started!`,
                timestamp: new Date()
            });
        }

        // Render agent cards
        this.renderAgentCards();

        // Keep spawn section visible for multi-agent support
        this.logger.debug('Agent added to Map for agentId:', role);

        this.pendingSpawnRole = null;
        this.resetSpawnButton();

        // Notify app that agent is ready
        if (this.onAgentReady) {
            this.onAgentReady();
        }
    }

    /**
     * Handle agent:response message
     */
    handleAgentResponse(message: AgentResponseMessage): void {
        const role = message.agentId;
        const agent = this.agents.get(role);

        if (agent) {
            agent.messages.push({
                sender: 'agent',
                content: message.content,
                timestamp: new Date()
            });

            // If this is the currently open chat, render messages
            if (this.currentChatRole === role) {
                this.renderChatMessages(role);
            }

            // Update agent cards to reflect new message count
            this.renderAgentCards();
        }

        this.logger.debug('Agent response processed for role:', role);
    }

    /**
     * Handle agent:terminated message
     */
    handleAgentTerminated(message: any): void {
        const role = message.agentId;
        this.logger.info('Agent terminated from server:', role);

        // Remove agent from Map
        this.agents.delete(role);

        // If this was the current chat, close it
        if (this.currentChatRole === role) {
            this.currentChatRole = null;
            this.currentAgentRole = null;
            const agentCard = document.getElementById('agentCard');
            if (agentCard) {
                agentCard.style.display = 'none';
            }
        }

        // Re-render agent cards
        this.renderAgentCards();
    }

    /**
     * Handle session:ended message
     */
    handleSessionEnded(message: any): void {
        this.logger.info('Session ended, all agents terminated:', message);

        const cleanupStatus = message.cleanupStatus || 'complete';
        this.resetSessionState(cleanupStatus);

        if (cleanupStatus && cleanupStatus !== 'complete') {
            const recoverable = cleanupStatus === 'partial';
            this.showErrorNotification(
                'SESSION_CLEANUP',
                `Session cleanup reported as ${cleanupStatus}. Review relay logs before starting a new session.`,
                recoverable
            );
        }
    }

    /**
     * Display a message in the message history
     */
    displayMessage(sender: string, content: string): void {
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
    handleError(message: ErrorMessage): void {
        this.logger.error('Server error:', message);

        // Extract error details
        const errorCode = message.error?.code || message.code || 'UNKNOWN_ERROR';
        const errorMessage = message.error?.message || message.message || 'Unknown error occurred';
        const recoverable = message.error?.recoverable !== false;

        // Show error in a prominent way
        this.showErrorNotification(errorCode, errorMessage, recoverable);

        if (errorCode === 'AGENT_SPAWN_FAILED' || (recoverable && this.pendingSpawnRole)) {
            this.logger.info('Resetting spawn button after agent spawn error');
            this.pendingSpawnRole = null;
            this.resetSpawnButton();

            // Notify app of error
            if (this.onError) {
                this.onError();
            }
        }
    }

    /**
     * Show error notification
     */
    showErrorNotification(code: string, message: string, recoverable: boolean): void {
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
        this.logger.debug('sendMessage called:', {
            isConnected: this.isConnected,
            hasWs: !!this.ws,
            readyState: this.ws?.readyState,
            expectedOpen: WebSocket.OPEN,
            messageType: message.type
        });

        if (!this.isConnected) {
            this.logger.error('Cannot send message: isConnected is false');
            return false;
        }

        if (!this.ws) {
            this.logger.error('Cannot send message: ws is null');
            return false;
        }

        if (this.ws.readyState !== WebSocket.OPEN) {
            this.logger.error('Cannot send message: WebSocket not in OPEN state', {
                readyState: this.ws.readyState,
                expected: WebSocket.OPEN,
                states: {
                    CONNECTING: WebSocket.CONNECTING,
                    OPEN: WebSocket.OPEN,
                    CLOSING: WebSocket.CLOSING,
                    CLOSED: WebSocket.CLOSED
                }
            });
            return false;
        }

        try {
            const payload = JSON.stringify(message);
            this.logger.info('Sending message:', {
                type: message.type,
                payloadLength: payload.length
            });
            this.ws.send(payload);
            this.logger.info('Message sent successfully:', message.type);
            return true;
        } catch (error) {
            this.logger.error('Failed to send message - exception:', {
                error: error.message,
                type: message.type
            });
            return false;
        }
    }

    /**
     * Create a new session
     */
    createSession() {
        this.logger.info('createSession called - Pre-send state:', {
            isConnected: this.isConnected,
            hasWs: !!this.ws,
            readyState: this.ws?.readyState,
            userSessionId: this.userSessionId
        });

        const message = {
            type: 'session:create',
            version: '1.0'
        };

        if (!this.sendMessage(message)) {
            this.logger.error('Failed to send session:create message - sendMessage returned false');
            const sessionStatusEl = document.getElementById('sessionStatus');
            if (sessionStatusEl) {
                sessionStatusEl.textContent = 'Failed to create session';
            }
        } else {
            this.logger.info('session:create message sent successfully, waiting for response');
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
        const sent = this.sendMessage(message);
        if (sent) {
            this.pendingSpawnRole = role;
        } else {
            this.pendingSpawnRole = null;
        }
        return sent;
    }

    /**
     * Send a message to an agent
     */
    sendAgentMessage(role, content) {
        if (!this.userSessionId) {
            this.logger.error('Cannot send message: no session');
            return false;
        }

        // Add to agent's message history
        const agent = this.agents.get(role);
        if (agent) {
            agent.messages.push({
                sender: 'user',
                content: content,
                timestamp: new Date()
            });
        }

        const message = {
            type: 'agent:message',
            version: '1.0',
            userSessionId: this.userSessionId,
            agentId: role,
            content: content
        };

        // Update agent cards to reflect new message count
        this.renderAgentCards();

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
     * Render agent cards in the UI
     */
    renderAgentCards() {
        const container = document.getElementById('agentCards');
        if (!container) {
            this.logger.error('Agent cards container not found');
            return;
        }

        // Clear existing cards
        container.innerHTML = '';

        // Render each agent
        this.agents.forEach((agent, role) => {
            const card = document.createElement('div');
            card.className = 'agent-card-item';

            // Create info section using DOM APIs to prevent XSS
            const info = document.createElement('div');
            info.className = 'agent-card-info';

            const roleEl = document.createElement('span');
            roleEl.className = 'agent-card-role';
            roleEl.textContent = `Agent: ${role}`;

            const statusEl = document.createElement('span');
            statusEl.className = `agent-card-status ${agent.status}`;
            statusEl.textContent = agent.status;

            const countEl = document.createElement('span');
            countEl.textContent = `Messages: ${agent.messages.length}`;

            info.append(roleEl, statusEl, countEl);

            // Create actions section
            const actions = document.createElement('div');
            actions.className = 'agent-card-actions';

            const terminateBtn = document.createElement('button');
            terminateBtn.setAttribute('data-role', role);
            terminateBtn.setAttribute('title', 'Terminate agent');
            terminateBtn.textContent = '×';

            actions.appendChild(terminateBtn);
            card.append(info, actions);

            // Click card to show chat (but not the button)
            card.addEventListener('click', (e) => {
                if (!(e.target instanceof HTMLElement && e.target.closest('button'))) {
                    this.showAgentChat(role);
                }
            });

            // Click terminate button
            terminateBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                this.terminateAgentCard(role);
            });

            container.appendChild(card);
        });

        // Show/hide terminate all button based on agent count
        const terminateAllBtn = document.getElementById('terminateAll');
        if (terminateAllBtn) {
            terminateAllBtn.style.display = this.agents.size > 1 ? 'block' : 'none';
        }

        this.logger.debug('Rendered', this.agents.size, 'agent cards');
    }

    /**
     * Show chat interface for a specific agent
     */
    showAgentChat(role) {
        const agent = this.agents.get(role);
        if (!agent) {
            this.logger.error('Agent not found:', role);
            return;
        }

        this.currentChatRole = role;
        this.currentAgentRole = role; // Track currently displayed agent

        // Update chat header with agent name
        const chatAgentName = document.getElementById('chatAgentName');
        if (chatAgentName) {
            chatAgentName.textContent = `Agent: ${role}`;
        }

        // Show chat container
        const chatContainer = document.getElementById('chatContainer');
        if (chatContainer) {
            chatContainer.style.display = 'flex';
        }

        // Render all messages for this agent
        this.renderChatMessages(role);

        // Focus input
        const chatInput = document.getElementById('chatInput');
        if (chatInput) {
            chatInput.focus();
        }

        this.logger.debug('Showing chat for agent:', role);
    }

    /**
     * Close chat interface
     */
    closeChat() {
        const chatContainer = document.getElementById('chatContainer');
        if (chatContainer) {
            chatContainer.style.display = 'none';
        }
        this.currentChatRole = null;
        this.currentAgentRole = null; // Clear currently displayed agent
        this.logger.debug('Chat closed');
    }

    /**
     * Render chat messages for a specific agent
     */
    renderChatMessages(role) {
        const agent = this.agents.get(role);
        if (!agent) {
            this.logger.error('Agent not found:', role);
            return;
        }

        const container = document.getElementById('chatMessages');
        if (!container) {
            this.logger.error('Chat messages container not found');
            return;
        }

        // Clear existing messages
        container.innerHTML = '';

        // Render each message
        agent.messages.forEach(msg => {
            const div = document.createElement('div');
            div.className = `chat-message ${msg.sender}`;
            div.textContent = msg.content;
            container.appendChild(div);
        });

        // Auto-scroll to bottom
        container.scrollTop = container.scrollHeight;

        this.logger.debug('Rendered', agent.messages.length, 'messages for agent:', role);
    }

    /**
     * Send message from chat interface
     */
    sendChatMessage() {
        const input = document.getElementById('chatInput') as HTMLInputElement;
        if (!input) {
            this.logger.error('Chat input not found');
            return;
        }

        const content = input.value.trim();
        if (!content || !this.currentChatRole) {
            return;
        }

        const agent = this.agents.get(this.currentChatRole);
        if (!agent) {
            this.logger.error('No agent found for current chat');
            return;
        }

        // Add user message to agent's messages
        agent.messages.push({
            sender: 'user',
            content: content,
            timestamp: new Date()
        });

        // Send to server via sendMessage (includes connection checks)
        const message = {
            type: 'agent:message',
            version: '1.0',
            userSessionId: this.userSessionId,
            agentId: this.currentChatRole,
            content: content
        };

        this.logger.debug('Sending message via chat interface:', message);
        const success = this.sendMessage(message);

        if (!success) {
            this.logger.error('Failed to send chat message');
            // Optionally notify user of send failure
            return;
        }

        // Clear input and re-render
        input.value = '';
        this.renderChatMessages(this.currentChatRole);
        this.renderAgentCards(); // Update message count

        this.logger.debug('Message sent from chat interface');
    }

    /**
     * Terminate a specific agent from card UI
     */
    terminateAgentCard(role: string): boolean {
        this.logger.info('Terminating agent from card:', role);

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

        this.logger.info('Sending agent:terminate:', message);
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
     * Terminate all agents (Multi-agent support)
     * Sends session:end message to server which terminates all agents
     */
    terminateAll() {
        if (!this.userSessionId) {
            this.logger.error('Cannot terminate all: no session');
            return false;
        }

        if (!confirm(`Terminate all ${this.agents.size} agents?`)) {
            return false;
        }

        const message = {
            type: 'session:end',
            version: '1.0',
            userSessionId: this.userSessionId
        };

        this.logger.info('Terminating all agents:', message);
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
    updateConnectionStatus(state: 'connected' | 'connecting' | 'reconnecting' | 'disconnected' | 'error' | 'failed', text: string): void {
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
        this.resetSessionState();
        this.updateConnectionStatus('disconnected', 'Disconnected');
    }

    private resetSessionState(cleanupStatus?: string): void {
        this.logger.info('[UI UPDATE] Resetting session state', {
            cleanupStatus
        });

        this.agents.clear();
        this.userSessionId = null;
        this.currentAgentRole = null;
        this.currentChatRole = null;
        this.pendingSpawnRole = null;

        this.closeChat();
        this.renderAgentCards();
        this.setSessionControls(false);

        const sessionInfoCard = document.getElementById('sessionInfo');
        if (sessionInfoCard) {
            sessionInfoCard.style.display = 'none';
        }

        const welcomeCard = document.getElementById('welcomeCard');
        if (welcomeCard) {
            welcomeCard.style.display = 'block';
        }

        const sessionIdEl = document.getElementById('userSessionId');
        if (sessionIdEl) {
            sessionIdEl.textContent = '-';
        }

        const sessionStatusEl = document.getElementById('sessionStatus');
        if (sessionStatusEl) {
            sessionStatusEl.textContent = cleanupStatus ? `Ended (${cleanupStatus})` : 'No active session';
        }

        this.updateCleanupBanner(cleanupStatus);

        const newProjectBtn = document.getElementById('newProjectBtn') as HTMLButtonElement | null;
        if (newProjectBtn) {
            newProjectBtn.disabled = false;
            newProjectBtn.innerHTML = '<span class="btn-icon">+</span> New Project';
        }
    }

    private updateCleanupBanner(cleanupStatus?: string): void {
        const bannerId = 'cleanupStatusBanner';
        let banner = document.getElementById(bannerId);

        if (!cleanupStatus || cleanupStatus === 'complete') {
            if (banner) {
                banner.remove();
            }
            return;
        }

        if (!banner) {
            banner = document.createElement('div');
            banner.id = bannerId;
            banner.className = 'cleanup-status-banner';
            banner.style.cssText = 'margin-top:12px;padding:12px;border:1px solid #f6ad55;background:#fff7ed;border-radius:8px;font-size:0.9rem;';
            const welcomeCard = document.getElementById('welcomeCard');
            if (welcomeCard) {
                welcomeCard.insertAdjacentElement('afterbegin', banner);
            } else {
                document.body.appendChild(banner);
            }
        }

        banner.textContent = `Previous session cleanup reported as ${cleanupStatus}. Check relay logs before starting a new session.`;
    }

    private setSessionControls(enabled: boolean): void {
        const endSessionBtn = document.getElementById('endSessionBtn') as HTMLButtonElement | null;
        if (endSessionBtn) {
            endSessionBtn.disabled = !enabled;
            endSessionBtn.textContent = 'End Session';
        }

        const spawnAgentBtn = document.getElementById('spawnAgentBtn') as HTMLButtonElement | null;
        if (spawnAgentBtn) {
            if (!enabled) {
                spawnAgentBtn.innerHTML = SPAWN_BUTTON_DEFAULT;
            }
            spawnAgentBtn.disabled = !enabled;
        }

        const terminateAgentBtn = document.getElementById('terminateAgentBtn') as HTMLButtonElement | null;
        if (terminateAgentBtn) {
            terminateAgentBtn.disabled = !enabled;
        }

        const sendMessageBtn = document.getElementById('sendMessageBtn') as HTMLButtonElement | null;
        if (sendMessageBtn) {
            sendMessageBtn.disabled = !enabled;
        }
    }

    private resetSpawnButton(): void {
        const spawnAgentBtn = document.getElementById('spawnAgentBtn') as HTMLButtonElement | null;
        if (!spawnAgentBtn) {
            return;
        }

        spawnAgentBtn.innerHTML = SPAWN_BUTTON_DEFAULT;
        spawnAgentBtn.disabled = !this.userSessionId;
    }
}
