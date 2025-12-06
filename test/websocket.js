const {WebSocket} = require('@jsh/ws');

const ws = new WebSocket('ws://127.0.0.1:3000/term/logs/data');
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
