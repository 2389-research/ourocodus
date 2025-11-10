class Logger {
    static levels = { debug: 0, info: 1, warn: 2, error: 3, none: 4 };

    constructor(component, options = {}) {
        this.component = component;
        this.level = options.level || Logger.getDefaultLevel();
    }

    static getDefaultLevel() {
        const stored = localStorage.getItem('ourocodus.logLevel');
        if (stored && stored in Logger.levels) {
            return stored;
        }
        return 'info';
    }

    static setLevel(level) {
        if (level in Logger.levels) {
            localStorage.setItem('ourocodus.logLevel', level);
        }
    }

    _shouldLog(level) {
        return Logger.levels[level] >= Logger.levels[this.level];
    }

    _log(level, color, msg, ...args) {
        if (this._shouldLog(level)) {
            const timestamp = new Date().toISOString().split('T')[1].slice(0, -1);
            console.log(`%c[${timestamp}] [${this.component}] ${msg}`, `color: ${color}`, ...args);
        }
    }

    debug(msg, ...args) {
        this._log('debug', '#999', msg, ...args);
    }

    info(msg, ...args) {
        this._log('info', '#0066cc', msg, ...args);
    }

    warn(msg, ...args) {
        this._log('warn', '#ff9900', msg, ...args);
    }

    error(msg, ...args) {
        this._log('error', '#cc0000', msg, ...args);
    }
}

// Make Logger available globally
window.Logger = Logger;
