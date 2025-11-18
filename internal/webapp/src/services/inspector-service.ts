/**
 * Inspector Service - Protocol Inspector with search and export
 */

interface Message {
    timestamp: number;
    direction: 'recv' | 'send';
    type: string;
    payload: any;
}

export class InspectorService {
    private messages: Message[] = [];
    private searchQuery: string = '';

    /**
     * Add message to history
     */
    public addMessage(direction: 'recv' | 'send', type: string, payload: any): void {
        this.messages.push({
            timestamp: Date.now(),
            direction,
            type,
            payload
        });
    }

    /**
     * Search messages by query
     */
    public search(query: string): void {
        this.searchQuery = query.toLowerCase();
        this.filterMessages();
    }

    /**
     * Filter visible messages based on search query
     */
    private filterMessages(): void {
        const messageElements = document.querySelectorAll('.inspector-message');

        messageElements.forEach((el, index) => {
            if (!this.searchQuery) {
                (el as HTMLElement).style.display = '';
                return;
            }

            const message = this.messages[index];
            if (!message) return;

            const matchesType = message.type.toLowerCase().includes(this.searchQuery);
            const matchesPayload = JSON.stringify(message.payload).toLowerCase().includes(this.searchQuery);

            (el as HTMLElement).style.display = (matchesType || matchesPayload) ? '' : 'none';
        });
    }

    /**
     * Export messages as JSON
     */
    public exportJSON(): void {
        const data = this.messages.map(msg => ({
            timestamp: new Date(msg.timestamp).toISOString(),
            direction: msg.direction,
            type: msg.type,
            payload: msg.payload
        }));

        const json = JSON.stringify(data, null, 2);
        const blob = new Blob([json], { type: 'application/json' });
        const url = URL.createObjectURL(blob);

        const a = document.createElement('a');
        a.href = url;
        a.download = `ourocodus-messages-${Date.now()}.json`;
        a.click();

        URL.revokeObjectURL(url);
    }

    /**
     * Clear all messages
     */
    public clear(): void {
        if (!confirm('Clear all protocol messages?')) {
            return;
        }

        this.messages = [];
        this.searchQuery = '';

        const container = document.querySelector('.inspector-messages');
        if (container) {
            const messages = container.querySelectorAll('.inspector-message');
            messages.forEach(msg => msg.remove());
        }
    }

    /**
     * Handle theme change from main PWA
     */
    public onThemeChange(theme: 'dark' | 'light'): void {
        document.documentElement.setAttribute('data-theme', theme);
    }
}
