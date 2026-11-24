// Controlled transport fixture only; it deliberately provides no language engine.
const mode = process.env.DOCKPIPE_LSP_FIXTURE_MODE;
let input = Buffer.alloc(0);
let document;
let exiting = false;

function send(message) {
    const body = Buffer.from(JSON.stringify(message));
    const header = Buffer.from(`Content-Length: ${body.length}\r\n\r\n`);
    // Independent writes exercise frames crossing pipe read boundaries.
    process.stdout.write(header.subarray(0, 8));
    process.stdout.write(header.subarray(8));
    process.stdout.write(body);
}

function handle(message) {
    if (message.method === 'initialize') {
        if (mode === 'malformed') {
            process.stdout.write('Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}');
            return;
        }
        send({ jsonrpc: '2.0', id: message.id, result: {
            capabilities: { positionEncoding: 'utf-16', hoverProvider: true }
        } });
    } else if (message.method === 'textDocument/didOpen' || message.method === 'textDocument/didChange') {
        document = message.params.textDocument;
        const diagnostic = (version, text) => send({ jsonrpc: '2.0', method: 'textDocument/publishDiagnostics', params: {
            uri: document.uri, version, diagnostics: [{ message: text, severity: 1,
                range: { start: { line: 0, character: 0 }, end: { line: 0, character: 1 } } }]
        } });
        diagnostic(document.version - 1, 'stale');
        diagnostic(document.version, 'current');
    } else if (message.method === 'textDocument/hover') {
        if (mode === 'death') {
            process.exit(7);
        }
        setTimeout(() => {
            if (!exiting)
                send({ jsonrpc: '2.0', id: message.id, result: { contents: { kind: 'plaintext', value: 'controlled hover' } } });
        }, 200);
    } else if (message.method === 'shutdown') {
        send({ jsonrpc: '2.0', id: message.id, result: null });
    } else if (message.method === 'exit') {
        exiting = true;
        process.exit(0);
    }
}

process.stdin.on('data', chunk => {
    input = Buffer.concat([input, chunk]);
    while (true) {
        const headerEnd = input.indexOf('\r\n\r\n');
        if (headerEnd < 0)
            return;
        const length = Number(input.subarray(0, headerEnd).toString().split(':')[1]);
        if (input.length < headerEnd + 4 + length)
            return;
        const message = JSON.parse(input.subarray(headerEnd + 4, headerEnd + 4 + length));
        input = input.subarray(headerEnd + 4 + length);
        handle(message);
    }
});
