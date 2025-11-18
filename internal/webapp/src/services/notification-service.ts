/**
 * Notification Service - Enhanced notifications with retry capability
 */

interface NotificationOptions {
    recoverable?: boolean;
    retryCallback?: () => void | Promise<void>;
}

export class NotificationService {
    private container: HTMLElement | null = null;
    private notificationId = 0;

    constructor() {
        this.ensureContainer();
    }

    /**
     * Ensure notification container exists
     */
    private ensureContainer(): void {
        this.container = document.getElementById('notificationContainer');
        if (!this.container) {
            this.container = document.createElement('div');
            this.container.id = 'notificationContainer';
            this.container.className = 'notification-container';
            document.body.appendChild(this.container);
        }
    }

    /**
     * Show error notification with optional retry
     */
    public showError(message: string, options?: NotificationOptions): void {
        this.ensureContainer();
        const id = `notification-${this.notificationId++}`;

        const notification = document.createElement('div');
        notification.className = 'notification error';
        notification.id = id;

        const content = document.createElement('div');
        content.className = 'notification-content';

        const messageEl = document.createElement('span');
        messageEl.className = 'notification-message';
        messageEl.textContent = message;
        content.appendChild(messageEl);

        const actions = document.createElement('div');
        actions.className = 'notification-actions';

        // Add retry button if recoverable
        if (options?.recoverable && options?.retryCallback) {
            const retryBtn = document.createElement('button');
            retryBtn.className = 'btn btn-small btn-retry';
            retryBtn.textContent = 'Retry';
            retryBtn.addEventListener('click', async () => {
                retryBtn.disabled = true;
                retryBtn.textContent = 'Retrying...';

                try {
                    await options.retryCallback?.();
                    this.dismiss(id);
                } catch (error) {
                    retryBtn.disabled = false;
                    retryBtn.textContent = 'Retry';
                    // Error will trigger new notification
                }
            });
            actions.appendChild(retryBtn);
        }

        // Add dismiss button
        const dismissBtn = document.createElement('button');
        dismissBtn.className = 'btn btn-small btn-dismiss';
        dismissBtn.textContent = 'Dismiss';
        dismissBtn.addEventListener('click', () => this.dismiss(id));
        actions.appendChild(dismissBtn);

        notification.appendChild(content);
        notification.appendChild(actions);

        this.container?.appendChild(notification);

        // Auto-dismiss after 10 seconds if not recoverable
        if (!options?.recoverable) {
            setTimeout(() => this.dismiss(id), 10000);
        }
    }

    /**
     * Dismiss notification by ID
     */
    public dismiss(notificationId: string): void {
        const notification = document.getElementById(notificationId);
        if (notification) {
            notification.style.opacity = '0';
            setTimeout(() => notification.remove(), 300);
        }
    }
}
