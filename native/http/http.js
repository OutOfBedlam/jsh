'use strict';

class Agent {
    constructor(opts = {}) {
        this.options = opts;
        this.raw = _http.NewClient();
    }
    destroy() {
        this.raw.close();
    }
}

// http.request(options[, callback])
// http.request(url[, options][, callback])
//  - url: string | URL
//  - options: RequestOptions
//  - callback: function(response)
function request() {
    let options = {};
    let url = undefined;
    if (arguments.length == 0) {
        throw new TypeError("At least one argument is required");
    }
    if (typeof arguments[0] === "string") {
        url = new URL(arguments[0]);
        if (typeof arguments[1] === "object") {
            options = arguments[1] || {};
        }
    } else if (arguments[0] instanceof URL) {
        url = arguments[0];
        if (typeof arguments[1] === "object") {
            options = arguments[1] || {};
        }
    } else if (arguments[0] instanceof Object) {
        options = arguments[0] || {};
        if (typeof options.url === "string") {
            url = new URL(options.url);
        } else if (options.url instanceof URL) {
            url = options.url;
        }
    } else {
        throw new TypeError("The first argument must be of type string, URL, or object, but got " + typeof arguments[0]);
    }

    if (url) {
        options = {
            ...options,
            protocol: url.protocol,
            host: url.host,
            hostname: url.hostname.trimRight('/'),
            port: url.port,
            path: url.pathname + url.search,
        };
    }
    return new ClientRequest(options);
}

// events: "response", "error", "end"
class OutgoingMessage extends EventEmitter {
    constructor() {
        super();
    }
}

// events: "response", "error", "end"
class ClientRequest extends OutgoingMessage {
    constructor(options) {
        super();
        if (!options.method || typeof options.method !== "string" || options.method.length === 0) {
            options.method = "GET";
        }
        const url = new URL(
            (options.protocol || "http:") + "//" +
            (options.hostname || options.host || "localhost") +
            (options.port ? (":" + options.port) : "") +
            (options.path || "/"));
        const req = _http.NewRequest(options.method.toUpperCase(), url.toString());
        for (const key in options) {
            if (key === "headers") {
                for (const hkey in options.headers) {
                    req.header.set(hkey, options.headers[hkey].toString());
                }
            } else if (key === "auth") {
                req.header.set("Authorization", "Basic " + _http.base64Encode(options.auth));
            }
        }
        this.raw = req;
        this.options = options;
    }
    destroy(err) {
        this.raw = null;
        if (err) {
            this.emit('error', err);
        }
    }
    // request.write(chunk[, encoding][, callback])
    // chunk - string | Buffer | Uint8Array
    // encoding - string
    // callback - function
    // Returns: true | false
    write() {
        const chunk = arguments[0];
        const encoding = (typeof arguments[1] === 'string') ? arguments[1] : 'utf-8';
        const callback = (typeof arguments[arguments.length - 1] === 'function') ? arguments[arguments.length - 1] : null;

        let err = null;
        if (typeof chunk === 'string') {
            err = this.raw.writeString(chunk, encoding);
        } else if (chunk instanceof Uint8Array) {
            err = this.raw.write(chunk);
        } else {
            this.emit('error', new TypeError("The chunk argument must be of type string, Buffer, or Uint8Array, but got " + typeof chunk));
            return false;
        }
        if (err instanceof Error) {
            this.emit('error', new Error("Failed to write chunk to request stream"));
            return false;
        }
        if (callback) {
            callback();
        }
        return true;
    }
    // request.end([data[, encoding]][, callback])
    // data - string | Buffer | Uint8Array
    // encoding - string
    // callback - function
    // Returns: this
    end() {
        const callback = (typeof arguments[arguments.length - 1] === 'function') ? arguments[arguments.length - 1] : null;

        let argLen = callback ? arguments.length - 1 : arguments.length;
        const data = (argLen > 0) ? arguments[0] : undefined;
        const encoding = (argLen > 1) ? arguments[1] : undefined;

        if (data) {
            this.write(data, encoding);
        }
        if (callback) {
            this.on('response', (response) => {
                callback(response);
            });
        }
        setImmediate(() => {
            const agent = this.options.agent ? this.options.agent : new Agent();
            let rsp = undefined;
            try {
                rsp = agent.raw.do(this.raw);
                this.emit('response', rsp);
            } catch (err) {
                this.emit('error', err);
            } finally {
                if (rsp) {
                    rsp.close();
                }
                this.emit("end")
            }
        })

        // const p = new Promise((resolve, reject) => {
        //     setImmediate(() => {
        //         const agent = this.options.agent ? this.options.agent : new Agent();
        //         try {
        //             const rsp = agent.raw.do(this.raw);
        //             this.emit('response', rsp);
        //             resolve(rsp);
        //         } catch (err) {
        //             this.emit('error', err);
        //             reject(err);
        //         }
        //     })
        // });
        return this;
    }
}

// http.get(options[, callback])
// http.get(url[, options][, callback])
//  - url: string | URL
//  - options: RequestOptions
//  - callback: function(response)
function get() {
    const callback = (typeof arguments[arguments.length - 1] === 'function') ? arguments[arguments.length - 1] : null;
    const args = callback ? Array.prototype.slice.call(arguments, 0, -1) : arguments;
    const req = request(...args);
    if (callback) {
        req.end(callback);
    } else {
        req.end();
    }
    return req;
}

module.exports = {
    Agent,
    get,
    request,
}