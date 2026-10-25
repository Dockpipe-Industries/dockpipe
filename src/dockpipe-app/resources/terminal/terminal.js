'use strict';

const terminal = new Terminal({
    cursorBlink: false,
    allowTransparency: true,
    scrollback: 2000,
    fontSize: 13,
    fontFamily: 'monospace',
    allowProposedApi: false,
    theme: { background: '#171c24', foreground: '#e4e9ef', cursor: '#a4e3d2' }
});
const fit = new FitAddon.FitAddon();
terminal.loadAddon(fit);
terminal.open(document.getElementById('terminal'));
window.dockpipeTerminal = terminal;

new QWebChannel(qt.webChannelTransport, channel => {
    const bridge = channel.objects.terminalBridge;
    let resizePending = false;
    let activeGeneration = -1;
    function resize() {
        if (document.hidden || resizePending)
            return;
        resizePending = true;
        requestAnimationFrame(() => {
            resizePending = false;
            fit.fit();
            bridge.terminalSize(terminal.cols, terminal.rows, activeGeneration);
        });
    }
    bridge.appearanceChanged.connect(settings => {
        const color = settings.theme.background;
        const red = parseInt(color.slice(1, 3), 16);
        const green = parseInt(color.slice(3, 5), 16);
        const blue = parseInt(color.slice(5, 7), 16);
        document.getElementById('terminal').style.backgroundColor = `rgba(${red}, ${green}, ${blue}, ${settings.opacity / 100})`;
        const glowColor = settings.theme.foreground;
        const glowRed = parseInt(glowColor.slice(1, 3), 16);
        const glowGreen = parseInt(glowColor.slice(3, 5), 16);
        const glowBlue = parseInt(glowColor.slice(5, 7), 16);
        document.body.style.setProperty('--terminal-glow',
            `rgba(${glowRed}, ${glowGreen}, ${glowBlue}, ${settings.glowStrength / 100})`);
        document.body.style.setProperty('--terminal-glow-spread', `${settings.glowSpread}px`);
        document.body.classList.toggle('scanlines', settings.scanlines);
        document.body.classList.toggle('glow', settings.glow && settings.glowStrength > 0 && settings.glowSpread > 0);
        document.documentElement.style.colorScheme = settings.theme.background === '#f4efe3' ? 'light' : 'dark';
        terminal.options.theme = { ...settings.theme, background: '#00000000' };
        terminal.options.fontFamily = settings.fontFamily;
        terminal.options.fontSize = settings.fontSize;
        terminal.options.cursorStyle = settings.cursorStyle;
        terminal.options.cursorBlink = settings.cursorBlink;
        terminal.options.scrollback = settings.scrollback;
        resize();
    });
    let writing = false;
    let pendingFrame = null;
    function renderFrame(frame) {
        if (writing) {
            // Native code admits one chunk per selected generation. If the
            // selection changes repeatedly, only its latest frame matters.
            pendingFrame = frame;
            return;
        }
        writing = true;
        activeGeneration = frame.generation;
        if (frame.reset) {
            terminal.reset();
            resize();
        }
        const bytes = Uint8Array.from(atob(frame.base64), character => character.charCodeAt(0));
        terminal.write(bytes, () => {
            writing = false;
            bridge.outputRendered(frame.generation);
            if (pendingFrame) {
                const next = pendingFrame;
                pendingFrame = null;
                renderFrame(next);
            }
        });
    }
    bridge.renderOutput.connect((base64, reset, generation) => {
        // Keep parser-generated replies bound to the generation being parsed,
        // even when a different session is selected during an asynchronous write.
        renderFrame({ base64, reset, generation });
    });
    terminal.onData(data => bridge.terminalInput(data, activeGeneration));
    new ResizeObserver(resize).observe(document.getElementById('terminal'));
    document.addEventListener('visibilitychange', resize);
    bridge.rendererReady();
    resize();
    terminal.focus();
});
