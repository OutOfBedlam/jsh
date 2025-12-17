'use strict';

class Client {
    constructor(conf) {
        this.db = _mach.NewDatabase(JSON.stringify(conf));
        this.ctx = this.db.ctx;
    }
    close() {
        this.db.close();
    }
    connect() {
        let conn = this.db.connect();
        return new Connection(this.ctx, conn);
    }
}

class Connection {
    constructor(ctx, dbConn) {
        this.ctx = ctx;
        this.conn = dbConn;
    }
    close() {
        this.conn.close();
    }
    query() {
        let rows = this.conn.query(this.ctx, ...arguments);
        return new Rows(this.ctx, rows);
    }
    queryRow() {
        let row = this.conn.queryRow(this.ctx, ...arguments);
        let cols = row.columns();
        let names = cols.names();
        let buffer = cols.makeBuffer();
        row.scan(...buffer);
        let value = {_ROWNUM: 1};
        for (let i = 0; i < names.length; i++) {
            value[names[i]] = buffer[i];
        }
        return value;
    }
    exec() {
        let result = this.conn.exec(this.ctx, ...arguments);
        if (result.err()) {
            throw new Error(result.err());
        }
        return {
            rowsAffected: result.rowsAffected(),
            message: result.message()
        };
    }
}

class Rows {
    constructor(ctx, dbRows) {
        this.ctx = ctx;
        this.rows = dbRows;
        this.cols = dbRows.columns();
        this.names = this.cols.names();
        this.rownum = 0;
        this.message = dbRows.message();
    }
    close() {
        this.rows.close();
    }
    [Symbol.iterator]() {
        return {
            next: () => {
                let hasNext = this.rows.next(this.ctx);
                if (!hasNext) {
                    return { done: true };
                }
                let buffer = this.cols.makeBuffer();
                this.rows.scan(...buffer);
                this.rownum += 1;
                let value = {_ROWNUM: this.rownum};
                for (let i = 0; i < this.names.length; i++) {
                    value[this.names[i]] = buffer[i];
                }
                return { value: value, done: false };
            }
        };
    }
}

module.exports = {
    Client
};