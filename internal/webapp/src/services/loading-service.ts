/**
 * Loading Service - Shows progress indicators for long operations
 */

export class LoadingService {
    private overlay: HTMLElement | null = null;
    private messageEl: HTMLElement | null = null;

    /**
     * Show loading overlay on target element
     */
    public show(target: HTMLElement, message: string): void {
        // Remove existing overlay if any
        this.hide();

        // Create overlay
        this.overlay = document.createElement('div');
        this.overlay.className = 'loading-overlay';

        // Create spinner
        const spinner = document.createElement('div');
        spinner.className = 'loading-spinner';

        // Create message
        this.messageEl = document.createElement('div');
        this.messageEl.className = 'loading-message';
        this.messageEl.textContent = message;

        // Assemble and append
        this.overlay.appendChild(spinner);
        this.overlay.appendChild(this.messageEl);

        // Position relative to target
        target.style.position = 'relative';
        target.appendChild(this.overlay);
    }

    /**
     * Update loading message
     */
    public update(message: string): void {
        if (this.messageEl) {
            this.messageEl.textContent = message;
        }
    }

    /**
     * Hide and remove loading overlay
     */
    public hide(): void {
        if (this.overlay) {
            this.overlay.remove();
            this.overlay = null;
            this.messageEl = null;
        }
    }
}
