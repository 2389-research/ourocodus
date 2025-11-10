/**
 * Ourocodus PWA - Application Entry Point
 * Initializes the application when DOM is ready
 */

import { App } from './ui/state';

// Declare app on window object for debugging
declare global {
    interface Window {
        app: App;
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
