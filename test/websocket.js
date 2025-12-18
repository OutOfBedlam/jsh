const {WebSocket} = require('@jsh/ws');
const args = require('process').argv.slice(2);

let u = runtime.args[0];
if (!u) {
    console.println('Usage: websocket.js <ws://host:port/path>');
    runtime.exit(1);
}

const ws = new WebSocket(u);
ws.addEventListener('open', () => {
    console.printf('WebSocket connection opened\n');
});
ws.addEventListener('message', (event) => {
    console.printf('%v', event.data);
});
ws.addEventListener('close', (event) => {
    console.println('WebSocket connection closed:', event);
});
ws.addEventListener('error', (event) => {
    console.println('WebSocket error:', event);
});

runtime.addShutdownHook(() => {
    console.println('Shutting down WebSocket client...');
});
