class WebSocket extends EventEmitter {
    constructor(url) {
        super();
        if (url === undefined || typeof url !== 'string') {
            throw new TypeError('URL must be a string, got ' + typeof url);
        }
        this.url = url;
        this.raw = module.NewWebSocket(this);
        this.readyState =  this.CONNECTING;
        this.validateEventListener = (eventName, callback) => {
            return ["open", "close", "message", "error"].includes(eventName);
        }
        this.on('open', () => {
            this.readyState = this.OPEN;
        });
        this.on('close', () => {
            this.readyState = this.CLOSED
        });
        this.raw.open();
    }
    send(data) {
        this.raw.send(data);
    }
    close() {
        this.readyState = this.CLOSING;
        this.raw.close();
    }

    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;
}

module.exports = {
    WebSocket: WebSocket,
}