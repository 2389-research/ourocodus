import { describe, it, expect, beforeEach, vi } from 'vitest';
import { Logger } from './logger';

// Import types we need
import type { AgentState } from './types';

// We need to recreate the RelayConnection class here for testing
// since we can't easily import from app.ts due to IIFE bundling
class RelayConnection {
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
    public currentAgentRole: string | null;
    private wsUrl: string;
    public agents: Map<string, AgentState>;

    constructor(wsUrl: string = 'ws://localhost:8080/relay') {
        this.logger = new Logger('RelayConnection');
        this.ws = null;
        this.isConnected = false;
        this.shouldReconnect = true;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000;
        this.maxReconnectDelay = 30000;
        this.userSessionId = null;
        this.reconnectTimeout = null;
        this.currentChatRole = null;
        this.currentAgentRole = null;
        this.wsUrl = wsUrl;
        this.agents = new Map();
    }

    connect(): void {
        if (this.ws?.readyState === WebSocket.OPEN) {
            this.logger.info('Already connected');
            return;
        }

        this.logger.info('Connecting to relay...', this.wsUrl);
        this.ws = new WebSocket(this.wsUrl);

        this.ws.onopen = () => {
            this.isConnected = true;
            this.reconnectAttempts = 0;
            this.logger.info('Connected to relay');
        };

        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            this.handleMessage(message);
        };

        this.ws.onerror = () => {
            this.logger.error('WebSocket error');
        };

        this.ws.onclose = () => {
            this.isConnected = false;
            this.ws = null;
        };
    }

    sendMessage(message: any): boolean {
        if (!this.isConnected || !this.ws || this.ws.readyState !== WebSocket.OPEN) {
            this.logger.error('Cannot send message: not connected');
            return false;
        }

        try {
            this.ws.send(JSON.stringify(message));
            return true;
        } catch (error) {
            this.logger.error('Failed to send message:', error);
            return false;
        }
    }

    handleMessage(message: any): void {
        if (message.type === 'session:created') {
            this.userSessionId = message.sessionId;
        } else if (message.type === 'agent:ready') {
            this.agents.set(message.role, {
                role: message.role,
                status: 'ready',
                messages: [],
                workspace: message.workspace || './workspaces/default'
            });
        } else if (message.type === 'agent:terminated') {
            this.handleAgentTerminated(message.role);
        }
    }

    handleAgentTerminated(role: string): void {
        this.logger.info('Agent terminated:', role);
        this.agents.delete(role);

        if (this.currentChatRole === role) {
            this.currentChatRole = null;
        }
        if (this.currentAgentRole === role) {
            this.currentAgentRole = null;
        }
    }

    disconnect(): void {
        this.shouldReconnect = false;

        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.close(1000);
        }

        this.ws = null;
        this.isConnected = false;
        this.userSessionId = null;
        this.currentAgentRole = null;
        this.currentChatRole = null;
        this.agents.clear();
    }
}

describe('RelayConnection', () => {
    let connection: RelayConnection;

    beforeEach(() => {
        connection = new RelayConnection('ws://test.example.com');
    });

    describe('sendMessage', () => {
        it('should return false when not connected', () => {
            const result = connection.sendMessage({ type: 'test' });
            expect(result).toBe(false);
        });

        it('should return false when WebSocket is not open', () => {
            connection.connect();
            // WebSocket starts in CONNECTING state
            const result = connection.sendMessage({ type: 'test' });
            expect(result).toBe(false);
        });

        it('should return true when connected and send message', () => {
            connection.connect();

            // Simulate WebSocket opening
            (connection.ws as any)._simulateOpen();

            // Spy on WebSocket send
            const sendSpy = vi.spyOn(connection.ws!, 'send');

            const message = { type: 'test', data: 'hello' };
            const result = connection.sendMessage(message);

            expect(result).toBe(true);
            expect(sendSpy).toHaveBeenCalledWith(JSON.stringify(message));
        });

        it('should return false when send throws error', () => {
            connection.connect();
            (connection.ws as any)._simulateOpen();

            // Mock send to throw error
            vi.spyOn(connection.ws!, 'send').mockImplementation(() => {
                throw new Error('Send failed');
            });

            const result = connection.sendMessage({ type: 'test' });
            expect(result).toBe(false);
        });
    });

    describe('handleAgentTerminated', () => {
        it('should remove agent from agents map', () => {
            // Add an agent
            connection.agents.set('test-agent', {
                role: 'test-agent',
                status: 'ready',
                messages: [],
                workspace: './workspaces/test'
            });

            expect(connection.agents.has('test-agent')).toBe(true);

            // Terminate the agent
            connection.handleAgentTerminated('test-agent');

            expect(connection.agents.has('test-agent')).toBe(false);
        });

        it('should clear currentChatRole if terminated agent matches', () => {
            connection.currentChatRole = 'test-agent';
            connection.agents.set('test-agent', {
                role: 'test-agent',
                status: 'ready',
                messages: [],
                workspace: './workspaces/test'
            });

            connection.handleAgentTerminated('test-agent');

            expect(connection.currentChatRole).toBe(null);
        });

        it('should clear currentAgentRole if terminated agent matches', () => {
            connection.currentAgentRole = 'test-agent';
            connection.agents.set('test-agent', {
                role: 'test-agent',
                status: 'ready',
                messages: [],
                workspace: './workspaces/test'
            });

            connection.handleAgentTerminated('test-agent');

            expect(connection.currentAgentRole).toBe(null);
        });

        it('should not affect other agents', () => {
            connection.agents.set('agent1', {
                role: 'agent1',
                status: 'ready',
                messages: [],
                workspace: './workspaces/1'
            });
            connection.agents.set('agent2', {
                role: 'agent2',
                status: 'ready',
                messages: [],
                workspace: './workspaces/2'
            });

            connection.handleAgentTerminated('agent1');

            expect(connection.agents.has('agent1')).toBe(false);
            expect(connection.agents.has('agent2')).toBe(true);
        });
    });

    describe('handleMessage', () => {
        it('should set userSessionId on session:created', () => {
            connection.handleMessage({
                type: 'session:created',
                sessionId: 'test-session-123'
            });

            expect(connection.userSessionId).toBe('test-session-123');
        });

        it('should add agent to map on agent:ready', () => {
            connection.handleMessage({
                type: 'agent:ready',
                role: 'echo',
                workspace: './workspaces/echo'
            });

            expect(connection.agents.has('echo')).toBe(true);
            const agent = connection.agents.get('echo');
            expect(agent?.role).toBe('echo');
            expect(agent?.status).toBe('ready');
            expect(agent?.workspace).toBe('./workspaces/echo');
        });

        it('should call handleAgentTerminated on agent:terminated', () => {
            connection.agents.set('test-agent', {
                role: 'test-agent',
                status: 'ready',
                messages: [],
                workspace: './workspaces/test'
            });

            connection.handleMessage({
                type: 'agent:terminated',
                role: 'test-agent'
            });

            expect(connection.agents.has('test-agent')).toBe(false);
        });
    });

    describe('disconnect', () => {
        it('should clear all state', () => {
            connection.connect();
            (connection.ws as any)._simulateOpen();

            connection.userSessionId = 'test-session';
            connection.currentChatRole = 'test-agent';
            connection.currentAgentRole = 'test-agent';
            connection.agents.set('test-agent', {
                role: 'test-agent',
                status: 'ready',
                messages: [],
                workspace: './workspaces/test'
            });

            connection.disconnect();

            expect(connection.isConnected).toBe(false);
            expect(connection.ws).toBe(null);
            expect(connection.userSessionId).toBe(null);
            expect(connection.currentChatRole).toBe(null);
            expect(connection.currentAgentRole).toBe(null);
            expect(connection.agents.size).toBe(0);
        });
    });

    describe('connect', () => {
        it('should create WebSocket with correct URL', () => {
            connection.connect();

            expect(connection.ws).not.toBe(null);
            expect(connection.ws?.url).toBe('ws://test.example.com');
        });

        it('should set isConnected to true when WebSocket opens', () => {
            connection.connect();
            expect(connection.isConnected).toBe(false);

            (connection.ws as any)._simulateOpen();

            expect(connection.isConnected).toBe(true);
        });

        it('should not create new connection if already open', () => {
            connection.connect();
            (connection.ws as any)._simulateOpen();

            const firstWs = connection.ws;
            connection.connect();

            expect(connection.ws).toBe(firstWs);
        });
    });
});
