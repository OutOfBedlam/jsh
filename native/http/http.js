'use strict';

class Agent {
    constructor(opts={}) {
       this.options = opts;
        this.raw = module.NewAgent();
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
    let url;
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
        url = options.url instanceof URL ? options.url : new URL(options.url);
    } else {
        throw new TypeError("The first argument must be of type string, URL, or object, but got " + typeof arguments[0]);
    }

    options = {...options, 
        protocol: url.protocol,
        host: url.host,
        hostname: url.hostname.trimRight('/'),
        port: url.port,
        path: url.pathname+url.search,
    };
    return new ClientRequest(options);
}

class OutgoingMessage extends EventEmitter {
    constructor() {
        super();
        this.supportedEvents.push( "response", "drain", "finish", "prefinish");
    }
}

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
        const req = module.NewRequest(options.method.toUpperCase(), url.toString(), null);
        for (const key in options) {
            if (key === "headers") {
                for (const hkey in options.headers) {
                    req.header.set(hkey, options.headers[hkey].toString());
                }
            } else if (key === "auth") {
                req.header.set("Authorization", "Basic " + module.base64Encode(options.auth));
            }
        }
        this.raw = req;
        this.options = options;
        this.supportedEvents.push(
            'close', 'connect', 'continue', 'finish', 
            'information', 'socket', 'timeout', 'upgrade');
    }
    end() {
        const {eventLoop} = require('process');
        const p = new Promise((resolve, reject) => {
            eventLoop.runOnLoop(() => {
                const agent = this.options.agent ? this.options.agent : new Agent();
                try {
                    const rsp = agent.raw.do(this.raw);
                    this.emit('response', rsp);
                    resolve(rsp);
                } catch (err) {
                    this.emit('error', err);
                    reject(err);
                }
            })
        });
        return p;
    }
}

// http.get(options[, callback])
// http.get(url[, options][, callback])
//  - url: string | URL
//  - options: RequestOptions
//  - callback: function(response)
function get() {
    const callback = (typeof arguments[arguments.length - 1] === 'function') ? arguments[arguments.length - 1] : null;
    const req = request(...arguments);
    const rsp = req.end();
    if (callback) {
        rsp.then((response) => {
            callback(response);
        }).catch((err) => {
            callback(err);
        });
    }
    return rsp;
}

module.exports = {
    Agent,
    get,
    request,
}