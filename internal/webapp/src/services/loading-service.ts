/**
 * Loading Service - Shows progress indicators for long operations
 */

export class LoadingService {
    private overlay: HTMLElement | null = null;
    private messageEl: HTMLElement | null = null;
    private targetElement: HTMLElement | null = null;
    private originalPosition: string = '';

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

        // Store original position and set relative if needed
        this.targetElement = target;
        const computedPosition = window.getComputedStyle(target).position;
        if (computedPosition === 'static') {
            this.originalPosition = '';
            target.style.position = 'relative';
        } else {
            this.originalPosition = computedPosition;
        }

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

        // Restore original position if it was static
        if (this.targetElement && this.originalPosition === '') {
            this.targetElement.style.position = 'static';
        }
        this.targetElement = null;
        this.originalPosition = '';
    }
}
