class EventEmitter {
    constructor() {
        this.events = {};
        this.supportedEvents = [];
        this.addEventListener = this.on;
    }

    on(eventName, callback) {
        if (this.supportedEvents.includes(eventName) === false) {
            throw new Error(`"${eventName}" is not supported event type`);
        }
        if (typeof callback !== 'function') {
            throw new Error('callback must be a function');
        }
        if (!this.events[eventName]) {
            this.events[eventName] = [];
        }
        this.events[eventName].push(callback);
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
}