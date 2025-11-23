/**
 * Application state and modal management
 */

import { Logger } from '../logger';
import { RelayConnection } from '../connection';
import { ThemeService } from '../services/theme-service';
import { LoadingService } from '../services/loading-service';
import { NotificationService } from '../services/notification-service';

/**
 * Modal Manager - Handles modal display and interaction
 */
interface ModalOptions {
    onConfirm?: () => void;
    onCancel?: () => void;
    updateContent?: (modal: HTMLElement) => void;
}

class ModalManager {
    private logger: Logger;
    private modal: HTMLElement | null;
    private overlay: HTMLElement | null;
    private confirmBtn: HTMLElement | null;
    private cancelBtn: HTMLElement | null;
    private onConfirm: (() => void) | null;
    private onCancel: (() => void) | null;

    constructor(modalId: string) {
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

    setupListeners(): void {
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

    show(options: ModalOptions = {}): void {
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

    hide(): void {
        if (!this.modal) return;
        this.modal.style.display = 'none';
    }
}

/**
 * Application initialization
 */
export class App {
    private static readonly AGENT_SPAWN_SECTION_ID = 'agentSpawnSection';

    private logger: Logger;
    connection: RelayConnection;
    private theme: ThemeService;
    private loading: LoadingService;
    private notifications: NotificationService;
    private connectionCheckInterval: ReturnType<typeof setInterval> | null;
    private connectionCheckTimeout: ReturnType<typeof setTimeout> | null;
    private isConnecting: boolean;
    private disconnectModal: ModalManager;
    private endSessionModal: ModalManager;
    private terminateAgentModal: ModalManager;

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

        // Initialize services
        this.theme = new ThemeService();
        this.loading = new LoadingService();
        this.notifications = new NotificationService();

        // Set up connection callbacks for loading and error states
        this.connection.onAgentReady = () => {
            try {
                this.loading.hide();
            } catch (error) {
                this.logger.error('Error in onAgentReady callback:', error);
            }
        };
        this.connection.onError = () => {
            try {
                this.loading.hide();
            } catch (error) {
                this.logger.error('Error in onError callback:', error);
            }
        };
        this.connection.onShowError = (message: string, recoverable: boolean, retryCallback?: () => void) => {
            this.notifications.showError(message, {
                recoverable,
                retryCallback: retryCallback ? async () => {
                    try {
                        // Show loading when retrying
                        const spawnSection = document.getElementById(App.AGENT_SPAWN_SECTION_ID);
                        if (spawnSection) {
                            this.loading.show(spawnSection, 'Retrying...');
                        }
                        await retryCallback();
                    } catch (err) {
                        this.logger.error('Error during retryCallback:', err);
                        this.loading.hide();
                    }
                } : undefined
            });
        };
        this.connection.onShowSuccess = (message: string) => {
            this.notifications.showSuccess(message);
        };

        // Wire theme toggle button
        const themeToggleBtn = document.getElementById('themeToggle');
        if (themeToggleBtn) {
            themeToggleBtn.addEventListener('click', () => {
                this.logger.debug('Theme toggle clicked');
                this.theme.toggle();
            });
        }

        // Register service worker for offline support
        this.registerServiceWorker();

        // Setup New Project button handler
        const newProjectBtn = document.getElementById('newProjectBtn');
        if (newProjectBtn) {
            newProjectBtn.addEventListener('click', () => {
                this.logger.debug('New Project button clicked');
                this.handleNewProject();
            });
        }

        // Phase 3: Setup Discover Agents button handler
        const discoverAgentsBtn = document.getElementById('discoverAgentsBtn');
        if (discoverAgentsBtn) {
            discoverAgentsBtn.addEventListener('click', () => {
                this.logger.debug('Discover Agents button clicked');
                this.handleDiscoverAgents();
            });
        }

        // Phase 3: Setup Attach Agent modal handlers
        const confirmAttachBtn = document.getElementById('confirmAttachAgent');
        const cancelAttachBtn = document.getElementById('cancelAttachAgent');
        const attachModal = document.getElementById('attachAgentModal');
        const attachModalOverlay = attachModal?.querySelector('.modal-overlay');

        if (confirmAttachBtn) {
            confirmAttachBtn.addEventListener('click', () => {
                this.logger.debug('Confirm Attach button clicked');
                this.handleConfirmAttach();
            });
        }

        if (cancelAttachBtn) {
            cancelAttachBtn.addEventListener('click', () => {
                this.logger.debug('Cancel Attach button clicked');
                if (attachModal) {
                    attachModal.style.display = 'none';
                }
            });
        }

        if (attachModalOverlay) {
            attachModalOverlay.addEventListener('click', () => {
                this.logger.debug('Attach modal overlay clicked');
                if (attachModal) {
                    attachModal.style.display = 'none';
                }
            });
        }

        // Phase 3: Setup token visibility toggle
        const toggleTokenBtn = document.getElementById('toggleTokenVisibility');
        const tokenInput = document.getElementById('attachTokenInput') as HTMLInputElement;
        const toggleIcon = document.getElementById('toggleTokenIcon');

        if (toggleTokenBtn && tokenInput && toggleIcon) {
            toggleTokenBtn.addEventListener('click', () => {
                if (tokenInput.type === 'password') {
                    tokenInput.type = 'text';
                    toggleIcon.textContent = '🙈';
                } else {
                    tokenInput.type = 'password';
                    toggleIcon.textContent = '👁️';
                }
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

        // Setup Enter key in chat input
        const chatInput = document.getElementById('chatInput');
        if (chatInput) {
            chatInput.addEventListener('keypress', (e) => {
                if (e.key === 'Enter') {
                    e.preventDefault();
                    this.connection.sendChatMessage();
                }
            });
        }

        // Setup Terminate All button
        const terminateAllBtn = document.getElementById('terminateAll');
        if (terminateAllBtn) {
            terminateAllBtn.addEventListener('click', () => {
                this.logger.debug('Terminate All button clicked');
                this.connection.terminateAll();
            });
        }

        // Setup Close Chat button
        const closeChatBtn = document.getElementById('closeChatBtn');
        if (closeChatBtn) {
            closeChatBtn.addEventListener('click', () => {
                this.logger.debug('Close Chat button clicked');
                this.connection.closeChat();
            });
        }

        // Setup Send Chat Message button
        const sendChatMessageBtn = document.getElementById('sendChatMessageBtn');
        if (sendChatMessageBtn) {
            sendChatMessageBtn.addEventListener('click', () => {
                this.logger.debug('Send Chat Message button clicked');
                this.connection.sendChatMessage();
            });
        }

        // Setup Disconnect button
        const disconnectBtn = document.getElementById('disconnectBtn');
        if (disconnectBtn) {
            disconnectBtn.addEventListener('click', () => {
                this.logger.debug('Disconnect button clicked');
                this.disconnectModal.show({
                    onConfirm: () => {
                        this.logger.info('User confirmed disconnect');
                        // Just close WS, server will cleanup
                        this.connection.disconnect(1000, 'User requested disconnect');
                    },
                    onCancel: () => {
                        this.logger.info('User cancelled disconnect');
                    }
                });
            });
        }

        // Setup End Session button
        const endSessionBtn = document.getElementById('endSessionBtn');
        if (endSessionBtn) {
            endSessionBtn.addEventListener('click', () => {
                this.logger.debug('End Session button clicked');
                this.endSessionModal.show({
                    onConfirm: () => {
                        this.logger.info('User confirmed end session');
                        const btnEl = document.getElementById('endSessionBtn') as HTMLButtonElement | null;
                        if (btnEl) {
                            btnEl.disabled = true;
                            btnEl.textContent = 'Ending...';
                        }
                        this.connection.endSession();
                    },
                    onCancel: () => {
                        this.logger.info('User cancelled end session');
                    }
                });
            });
        }

        // Setup Terminate Agent button
        const terminateAgentBtn = document.getElementById('terminateAgentBtn');
        if (terminateAgentBtn) {
            terminateAgentBtn.addEventListener('click', () => {
                this.logger.debug('Terminate Agent button clicked');
                this.terminateAgentModal.show({
                    onConfirm: () => {
                        this.logger.info('User confirmed terminate agent');
                        const role = this.connection.currentChatRole || this.connection.currentAgentRole;
                        if (role) {
                            this.connection.terminateAgent(role);
                        }
                    },
                    onCancel: () => {
                        this.logger.info('User cancelled terminate agent');
                    },
                    updateContent: (modal) => {
                        // Update modal with current agent role
                        const roleEl = modal.querySelector('#terminateAgentRole');
                        if (roleEl) {
                            roleEl.textContent = this.connection.currentChatRole || this.connection.currentAgentRole || '-';
                        }
                    }
                });
            });
        }

        this.logger.info('Ourocodus PWA initialized');
    }

    handleNewProject() {
        const btn = document.getElementById('newProjectBtn') as HTMLButtonElement | null;
        if (!btn) {
            this.logger.error('New Project button not found');
            return;
        }

        // Disable button while connecting
        btn.disabled = true;
        btn.textContent = 'Connecting...';

        if (!this.connection.isConnected) {
            this.logger.info('Not connected, attempting connection...');
            this.isConnecting = true;

            // Start connection
            this.connection.connect();

            // Poll for connection (with timeout)
            this.connectionCheckInterval = setInterval(() => {
                this.logger.debug('Connection check...', {
                    isConnected: this.connection.isConnected,
                    wsReadyState: this.connection.ws?.readyState,
                });

                if (this.connection.isConnected) {
                    // Connected! Clean up and create session
                    clearInterval(this.connectionCheckInterval!);
                    clearTimeout(this.connectionCheckTimeout!);
                    this.connectionCheckInterval = null;
                    this.connectionCheckTimeout = null;
                    this.isConnecting = false;

                    this.logger.info('Connection established, creating session...');
                    this.connection.createSession();
                    btn.disabled = false;
                    btn.innerHTML = '<span class="btn-icon">+</span> New Project';
                }
            }, 100);

            // Timeout after 10 seconds
            this.connectionCheckTimeout = setTimeout(() => {
                // Clean up interval and reset state
                clearInterval(this.connectionCheckInterval!);
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
        const spawnSection = document.getElementById(App.AGENT_SPAWN_SECTION_ID);

        if (!roleInput || !workspaceInput) {
            this.logger.error('Agent spawn inputs not found');
            return;
        }

        const role = (roleInput as HTMLInputElement).value.trim();
        const workspace = (workspaceInput as HTMLInputElement).value.trim();

        if (!role || !workspace) {
            alert('Please provide both agent role and workspace');
            return;
        }

        (btn as HTMLButtonElement).disabled = true;
        btn!.textContent = 'Spawning...';

        // Show loading indicator
        if (spawnSection) {
            this.loading.show(spawnSection, 'Initializing workspace...');
        }

        if (this.connection.spawnAgent(role, workspace)) {
            this.logger.info('Agent spawn initiated');
            // Button will be re-enabled when agent:ready is received
            // Loading will be hidden when agent:ready or error is received
        } else {
            this.logger.error('Failed to spawn agent');
            (btn as HTMLButtonElement).disabled = false;
            btn!.innerHTML = '<span class="btn-icon">🤖</span> Spawn Agent';
            this.loading.hide();
        }
    }

    /**
     * Phase 3: Handle Discover Agents button click
     */
    handleDiscoverAgents() {
        this.logger.info('Discovering CLI-spawned agents');
        if (this.connection.discoverAgents()) {
            this.logger.info('Agent discovery initiated');
        } else {
            this.logger.error('Failed to initiate agent discovery');
            alert('Failed to discover agents. Please ensure you are connected.');
        }
    }

    /**
     * Phase 3: Handle Confirm Attach button click
     */
    handleConfirmAttach() {
        const modal = document.getElementById('attachAgentModal');
        const tokenInput = document.getElementById('attachTokenInput') as HTMLInputElement;
        const errorEl = document.getElementById('attachError');
        const agentId = (modal as any)?._pendingAgentId;

        if (!tokenInput || !agentId) {
            this.logger.error('Attach modal elements not found or no pending agent ID');
            return;
        }

        const token = tokenInput.value.trim();
        if (!token) {
            if (errorEl) {
                errorEl.textContent = '⚠️ Please enter an attach token';
                errorEl.style.display = 'block';
            }
            return;
        }

        this.logger.info('Attempting to attach to agent:', agentId);
        if (this.connection.attachAgent(agentId, token)) {
            this.logger.info('Agent attachment initiated');
            // Clear token immediately after use for security
            tokenInput.value = '';
            // Modal will be closed by handleAgentAttached on success
        } else {
            this.logger.error('Failed to initiate agent attachment');
            // Clear token on failure too
            tokenInput.value = '';
            if (errorEl) {
                errorEl.textContent = '⚠️ Failed to attach. Please ensure you are connected.';
                errorEl.style.display = 'block';
            }
        }
    }

    handleSendMessage() {
        const messageInput = document.getElementById('messageInput');
        const btn = document.getElementById('sendMessageBtn');

        if (!messageInput) {
            this.logger.error('Message input not found');
            return;
        }

        const content = (messageInput as HTMLInputElement).value.trim();
        if (!content) {
            return;
        }

        const role = this.connection.currentChatRole || this.connection.currentAgentRole;
        if (!role) {
            this.logger.error('No active agent');
            return;
        }

        (btn as HTMLButtonElement).disabled = true;
        (messageInput as HTMLInputElement).disabled = true;

        if (this.connection.sendAgentMessage(role, content)) {
            this.logger.debug('Message sent');
            (messageInput as HTMLInputElement).value = '';
            // Re-enable after a short delay
            setTimeout(() => {
                (btn as HTMLButtonElement).disabled = false;
                (messageInput as HTMLInputElement).disabled = false;
                (messageInput as HTMLInputElement).focus();
            }, 500);
        } else {
            this.logger.error('Failed to send message');
            (btn as HTMLButtonElement).disabled = false;
            (messageInput as HTMLInputElement).disabled = false;
        }
    }

    registerServiceWorker() {
        if (!('serviceWorker' in navigator)) {
            this.logger.info('Service Worker not supported');
            return;
        }

        navigator.serviceWorker.register('/sw.js')
            .then(registration => {
                this.logger.info('Service Worker registered:', registration.scope);

                // Check for updates on page load
                registration.update();

                // Listen for updates
                registration.addEventListener('updatefound', () => {
                    const newWorker = registration.installing;
                    if (newWorker) {
                        newWorker.addEventListener('statechange', () => {
                            if (newWorker.state === 'installed' && navigator.serviceWorker.controller) {
                                this.logger.info('New version available - reload to update');
                            }
                        });
                    }
                });
            })
            .catch(error => {
                this.logger.error('Service Worker registration failed:', error);
            });
    }
}
