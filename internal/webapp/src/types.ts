// Type definitions for Ourocodus PWA

import { Logger } from './logger';

// Protocol message types
export interface BaseMessage {
    type: string;
    version: string;
}

export interface SessionCreateMessage extends BaseMessage {
    type: 'session:create';
}

export interface SessionCreatedMessage extends BaseMessage {
    type: 'session:created';
    userSessionId: string;
    timestamp: string;
}

export interface AgentSpawnMessage extends BaseMessage {
    type: 'agent:spawn';
    userSessionId: string;
    agentId: string;
    workspace: string;
}

export interface AgentReadyMessage extends BaseMessage {
    type: 'agent:ready';
    agentId: string;
    workspace?: string;
}

export interface AgentMessageRequest extends BaseMessage {
    type: 'agent:message';
    userSessionId: string;
    agentId: string;
    content: string;
}

export interface AgentResponseMessage extends BaseMessage {
    type: 'agent:response';
    agentId: string;
    content: string;
    done: boolean;
}

export interface SessionEndMessage extends BaseMessage {
    type: 'session:end';
    userSessionId: string;
}

export interface AgentTerminateMessage extends BaseMessage {
    type: 'agent:terminate';
    userSessionId: string;
    agentId: string;
}

export interface ConnectionEstablishedMessage extends BaseMessage {
    type: 'connection:established';
    serverId: string;
    timestamp: string;
}

// Error message types
export interface ErrorMessage extends BaseMessage {
    type: 'error';
    // Nested error object (preferred format)
    error?: {
        code?: string;
        message?: string;
        recoverable?: boolean;
    };
    // Top-level error properties (alternative format)
    code?: string;
    message?: string;
}

export type ProtocolMessage =
    | SessionCreateMessage
    | SessionCreatedMessage
    | AgentSpawnMessage
    | AgentReadyMessage
    | AgentMessageRequest
    | AgentResponseMessage
    | SessionEndMessage
    | AgentTerminateMessage
    | ConnectionEstablishedMessage
    | ErrorMessage;

// Agent state types
export interface ChatMessage {
    sender: 'user' | 'agent';
    content: string;
    timestamp: Date;
}

export interface AgentState {
    role: string;
    status: 'ready' | 'processing' | 'terminated';
    messages: ChatMessage[];
    workspace: string;
}

// WebSocket connection state
export interface WebSocketConn {
    send(data: string): void;
    close(): void;
    readyState: number;
}

type AppType = import('./ui/state').App;

// Global window declarations
declare global {
    interface Window {
        Logger: typeof Logger;
        app?: AppType;
    }
}

// Re-export for convenience
export type { Logger };
