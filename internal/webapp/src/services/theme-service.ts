/**
 * Theme Service - Manages dark/light theme switching with persistence
 */

export type Theme = 'dark' | 'light';

export class ThemeService {
    private currentTheme: Theme;

    constructor() {
        this.currentTheme = this.loadTheme();
        this.apply();
    }

    /**
     * Load theme preference from localStorage or system preference
     */
    private loadTheme(): Theme {
        // Check localStorage first
        const stored = localStorage.getItem('ourocodus.theme');
        if (stored === 'dark' || stored === 'light') {
            return stored;
        }

        // Fall back to system preference
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        return prefersDark ? 'dark' : 'light';
    }

    /**
     * Apply theme to document
     */
    private apply(): void {
        document.documentElement.setAttribute('data-theme', this.currentTheme);
        this.updateIcon();
        this.notifyInspector();
    }

    /**
     * Save theme to localStorage
     */
    private save(): void {
        localStorage.setItem('ourocodus.theme', this.currentTheme);
    }

    /**
     * Update theme toggle button icon
     */
    private updateIcon(): void {
        const icon = document.getElementById('themeIcon');
        if (icon) {
            // Show sun for light theme (switch to dark)
            // Show moon for dark theme (switch to light)
            icon.textContent = this.currentTheme === 'dark' ? '🌙' : '☀️';
        }
    }

    /**
     * Notify inspector iframe of theme change
     */
    private notifyInspector(): void {
        const inspectorFrame = document.querySelector('iframe');
        if (inspectorFrame?.contentWindow) {
            inspectorFrame.contentWindow.postMessage({
                type: 'theme:change',
                theme: this.currentTheme
            }, '*');
        }
    }

    /**
     * Toggle between dark and light themes
     */
    public toggle(): void {
        this.currentTheme = this.currentTheme === 'dark' ? 'light' : 'dark';
        this.apply();
        this.save();
    }

    /**
     * Get current theme
     */
    public getCurrentTheme(): Theme {
        return this.currentTheme;
    }
}
