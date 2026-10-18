// Run with Node: node tests/TerminalRendererTest.cjs
// Reproduce a terminal parser reply arriving while a new session is selected.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const inputs = [];
const acknowledgements = [];
const writes = [];
let output;
let appearance;
let terminal;
const bridge = {
    appearanceChanged: { connect(callback) { appearance = callback; } },
    renderOutput: { connect(callback) { output = callback; } },
    outputRendered(generation) { acknowledgements.push(generation); },
    terminalInput(text, generation) { inputs.push({ text, generation }); },
    terminalSize() {},
    rendererReady() {}
};
class Terminal {
    constructor(options) {
        this.options = options;
        this.cols = 80;
        this.rows = 24;
        this.resets = 0;
        terminal = this;
    }
    loadAddon() {}
    open() {}
    focus() {}
    reset() { ++this.resets; }
    onData(callback) { this.data = callback; }
    write(bytes, callback) { writes.push({ bytes, callback }); }
}
const context = {
    Terminal,
    FitAddon: { FitAddon: class { fit() {} } },
    QWebChannel: class { constructor(transport, callback) { callback({ objects: { terminalBridge: bridge } }); } },
    qt: { webChannelTransport: {} },
    window: {},
    document: {
        hidden: false,
        getElementById() { return {}; },
        addEventListener() {}
    },
    ResizeObserver: class { observe() {} },
    requestAnimationFrame(callback) { callback(); },
    atob(value) { return Buffer.from(value, 'base64').toString('binary'); },
    Uint8Array
};
const source = fs.readFileSync(path.join(__dirname, '../resources/terminal/terminal.js'), 'utf8');
vm.runInNewContext(source, context);
assert.equal(typeof appearance, 'function');
output(Buffer.from('A').toString('base64'), true, 1);
output(Buffer.from('B').toString('base64'), true, 2);
output(Buffer.from('C').toString('base64'), true, 3);
assert.equal(writes.length, 1, 'New generations must wait for the current parser');
assert.equal(terminal.resets, 1, 'Do not reset a terminal while an old chunk is parsing');
terminal.data('old parser reply');
assert.equal(inputs[0].generation, 1, 'Old parser input cannot target the newly selected shell');
writes[0].callback();
assert.equal(writes.length, 2, 'Only the latest pending selected generation is retained');
assert.equal(Buffer.from(writes[1].bytes).toString(), 'C');
terminal.data('current parser reply');
assert.equal(inputs[1].generation, 3);
writes[1].callback();
assert.deepEqual(acknowledgements, [1, 3]);
console.log('Terminal parser generation isolation and bounded pending frame passed.');
