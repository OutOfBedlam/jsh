class EventEmitter {
    constructor() {
        this.events = {};
        this.addEventListener = this.on;
        this.removeEventListener = this.off;
    }

    on(eventName, callback) {
        if (typeof callback !== 'function') {
            throw new Error('callback must be a function');
        }
        if (!this.events[eventName]) {
            this.events[eventName] = [];
        }
        this.events[eventName].push(callback);
    }

    once(eventName, callback) {
        const wrapper = (...args) => {
            callback(...args);
            this.off(eventName, wrapper);
        };
        this.on(eventName, wrapper);
    }

    emit(eventName, ...args) {
        const listeners = this.events[eventName];
        if (listeners) {
            for (const listener of listeners) {
                listener(...args);
            }
        }
    }

    off(eventName, callback) {
        const listeners = this.events[eventName];
        if (listeners) {
            this.events[eventName] = listeners.filter(listener => listener !== callback);
        }
    }

    removeAllListeners(eventName) {
        if (this.events[eventName]) {
            delete this.events[eventName];
        }
    }
}