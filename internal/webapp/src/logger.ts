type LogLevel = 'debug' | 'info' | 'warn' | 'error' | 'none';

interface LogLevels {
    debug: number;
    info: number;
    warn: number;
    error: number;
    none: number;
}

interface LoggerOptions {
    level?: LogLevel;
}

class Logger {
    static levels: LogLevels = { debug: 0, info: 1, warn: 2, error: 3, none: 4 };

    private component: string;
    private level: LogLevel;

    constructor(component: string, options: LoggerOptions = {}) {
        this.component = component;
        this.level = options.level || Logger.getDefaultLevel();
    }

    static getDefaultLevel(): LogLevel {
        const stored = localStorage.getItem('ourocodus.logLevel') as LogLevel | null;
        if (stored && stored in Logger.levels) {
            return stored;
        }
        return 'info';
    }

    static setLevel(level: LogLevel): void {
        if (level in Logger.levels) {
            localStorage.setItem('ourocodus.logLevel', level);
        }
    }

    private _shouldLog(level: LogLevel): boolean {
        return Logger.levels[level] >= Logger.levels[this.level];
    }

    private _log(level: LogLevel, color: string, msg: string, ...args: any[]): void {
        if (this._shouldLog(level)) {
            const timestamp = new Date().toISOString().split('T')[1].slice(0, -1);
            console.log(`%c[${timestamp}] [${this.component}] ${msg}`, `color: ${color}`, ...args);
        }
    }

    debug(msg: string, ...args: any[]): void {
        this._log('debug', '#999', msg, ...args);
    }

    info(msg: string, ...args: any[]): void {
        this._log('info', '#0066cc', msg, ...args);
    }

    warn(msg: string, ...args: any[]): void {
        this._log('warn', '#ff9900', msg, ...args);
    }

    error(msg: string, ...args: any[]): void {
        this._log('error', '#cc0000', msg, ...args);
    }
}

// Make Logger available globally
declare global {
    interface Window {
        Logger: typeof Logger;
    }
}

window.Logger = Logger;

export { Logger, LogLevel, LoggerOptions };
