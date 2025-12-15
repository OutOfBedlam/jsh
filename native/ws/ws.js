class WebSocket extends EventEmitter {
    constructor(url) {
        super();
        if (url === undefined || typeof url !== 'string') {
            throw new TypeError('URL must be a string, got ' + typeof url);
        }
        this.supportedEvents.push("open", "close", "message", "error");
        this.raw = module.NewWebSocket(this);
        this.readyState =  WebSocket.CONNECTING;
        this.url = url;

        const {eventLoop} = require('process');
        eventLoop.runOnLoop(() => {
            try {
                const conn = this.raw.connect(this.url);
                if (conn instanceof Error) {
                    this.emit('error', conn);
                } else {
                    this.conn = conn;
                    this.readyState = WebSocket.OPEN;
                    this.emit('open');
                    this.raw.start(this.conn, eventLoop);
                }
            }catch(e){
                this.emit('error', new Error(e.value));
            }
        });      
    }

    send(data) {
        if (!this.conn) {
            let err = new Error("websocket is not open");
            this.emit('error', err);
            return;
        }
        try {
            this.raw.send(this.conn, WebSocket.TextMessage, data);
        } catch (err) {
            this.emit('error', err);
        }
    }
    close() {
        this.readyState = this.CLOSING;
        if(this.conn) {
            this.conn.close();
        }
        this.readyState = this.CLOSED;
        this.emit('close');
    }

    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;

    static TextMessage = 1;
    static BinaryMessage = 2;
}

module.exports = {
    WebSocket: WebSocket,
}