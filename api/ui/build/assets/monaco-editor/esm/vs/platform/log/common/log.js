import { Emitter } from '../../../base/common/event.js';
import { Disposable } from '../../../base/common/lifecycle.js';
import { createDecorator } from '../../instantiation/common/instantiation.js';
export const ILogService = createDecorator('logService');
export var LogLevel;
(function (LogLevel) {
    LogLevel[LogLevel["Trace"] = 0] = "Trace";
    LogLevel[LogLevel["Debug"] = 1] = "Debug";
    LogLevel[LogLevel["Info"] = 2] = "Info";
    LogLevel[LogLevel["Warning"] = 3] = "Warning";
    LogLevel[LogLevel["Error"] = 4] = "Error";
    LogLevel[LogLevel["Critical"] = 5] = "Critical";
    LogLevel[LogLevel["Off"] = 6] = "Off";
})(LogLevel || (LogLevel = {}));
export const DEFAULT_LOG_LEVEL = LogLevel.Info;
export class AbstractLogger extends Disposable {
    constructor() {
        super(...arguments);
        this.level = DEFAULT_LOG_LEVEL;
        this._onDidChangeLogLevel = this._register(new Emitter());
    }
    setLevel(level) {
        if (this.level !== level) {
            this.level = level;
            this._onDidChangeLogLevel.fire(this.level);
        }
    }
    getLevel() {
        return this.level;
    }
}
export class ConsoleLogger extends AbstractLogger {
    constructor(logLevel = DEFAULT_LOG_LEVEL) {
        super();
        this.setLevel(logLevel);
    }
    trace(message, ...args) {
        if (this.getLevel() <= LogLevel.Trace) {
            console.log('%cTRACE', 'color: #888', message, ...args);
        }
    }
    debug(message, ...args) {
        if (this.getLevel() <= LogLevel.Debug) {
            console.log('%cDEBUG', 'background: #eee; color: #888', message, ...args);
        }
    }
    info(message, ...args) {
        if (this.getLevel() <= LogLevel.Info) {
            console.log('%c INFO', 'color: #33f', message, ...args);
        }
    }
    error(message, ...args) {
        if (this.getLevel() <= LogLevel.Error) {
            console.log('%c  ERR', 'color: #f33', message, ...args);
        }
    }
    dispose() {
        // noop
    }
}
export class LogService extends Disposable {
    constructor(logger) {
        super();
        this.logger = logger;
        this._register(logger);
    }
    getLevel() {
        return this.logger.getLevel();
    }
    trace(message, ...args) {
        this.logger.trace(message, ...args);
    }
    debug(message, ...args) {
        this.logger.debug(message, ...args);
    }
    info(message, ...args) {
        this.logger.info(message, ...args);
    }
    error(message, ...args) {
        this.logger.error(message, ...args);
    }
}
