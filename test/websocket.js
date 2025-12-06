const {WebSocket} = require('@jsh/ws');

const ws = new WebSocket('ws://127.0.0.1:3000/term/logs/data');
ws.addEventListener('open', () => {
    console.printf('WebSocket connection opened\n');
});
ws.addEventListener('message', (event) => {
    console.printf('%v', event.data);
});
ws.addEventListener('close', () => {
    console.printf('WebSocket connection closed\n');
});
ws.addEventListener('error', (event) => {
    console.printf('WebSocket error: %v\n', event.data);
});

runtime.addShutdownHook(() => {
    console.println('Shutting down WebSocket connection...');
    ws.close();
});

runtime.start();
