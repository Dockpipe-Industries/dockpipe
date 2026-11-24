#!/usr/bin/env python3
"""Generated HTTP/DOM/download fixture; run inside the owned Flatpak network namespace."""
import argparse
import hashlib
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
from pathlib import Path
import re
import time

SCHEMA = 'dockpipe.browser-fixture/v1'
MAX_EVENTS = 256


def payload(nonce):
    return ('Dockpipe generated browser download\nfixture=' + nonce + '\n').encode()


def route(path, nonce):
    if not re.fullmatch(r'[0-9a-f]{32}', nonce):
        raise ValueError('Invalid generated fixture identity')
    prefix = '/' + nonce + '/'
    if not path.startswith(prefix):
        return None
    selected = path[len(prefix):]
    return selected if selected in ['a', 'b', 'download.txt', 'event'] else None


def page(nonce, selected):
    title = 'Dockpipe fixture ' + selected.upper()
    return ('''<!doctype html><html><head><meta charset="utf-8"><title>''' + title + '''</title>
<style>body{font:24px sans-serif;padding:40px;background:#eff9f6;color:#123c30}button,a{font:22px sans-serif;padding:16px;margin:18px;display:inline-block}#status{padding:18px;border:2px solid #23604f}</style></head>
<body><h1>''' + title + '''</h1><p>Generated local fixture ''' + nonce[:8] + '''</p>
<p id="storage">Checking generated browser storage</p><button id="confirm">Confirm generated DOM</button>
<p id="status">Waiting for generated DOM input</p><a href="/''' + nonce + '''/download.txt">Download generated fixture</a>
<script nonce="''' + nonce + '''">
const fixture = "''' + nonce + '''";
const selected = "''' + selected + '''";
const previous = localStorage.getItem('dockpipe-generated-fixture');
const stored = previous === fixture;
document.getElementById('storage').textContent = stored ? 'Generated storage retained' : 'Generated storage empty';
async function record(event, present) {
 const response = await fetch('/' + fixture + '/event', {method:'POST',headers:{'Content-Type':'application/json'},
  body:JSON.stringify({event:event,page:selected,present:present})});
 if (!response.ok) throw new Error('Fixture acknowledgement failed');
}
record('storage', stored);
document.getElementById('confirm').addEventListener('click', async () => {
 localStorage.setItem('dockpipe-generated-fixture', fixture);
 await record('dom', true);
 document.getElementById('status').textContent = 'Generated DOM accepted ' + fixture.slice(0,8);
});
</script></body></html>''').encode()


def stop_requested(output, nonce, launch):
    path = output / ('browser-stop-' + str(launch) + '.json')
    if not path.exists():
        return False
    if path.is_symlink() or path.stat().st_size > 4096:
        raise ValueError('Invalid bounded fixture stop request')
    expected = {'schema': SCHEMA, 'nonce': nonce, 'launch': launch, 'command': 'stop'}
    if json.loads(path.read_text()) != expected:
        raise ValueError('Fixture stop request belongs to another invocation')
    return True


def serve(output, launch):
    configuration = json.loads((output / 'browser-fixture.json').read_text())
    nonce = configuration['nonce']
    if configuration.get('schema') != SCHEMA or not re.fullmatch(r'[0-9a-f]{32}', nonce):
        raise ValueError('Invalid generated browser fixture')
    port = 0
    if launch == 2:
        previous = json.loads((output / 'browser-server-1.json').read_text())
        port = previous['port']
        if not isinstance(port, int) or not 1 <= port <= 65535 or previous['nonce'] != nonce:
            raise ValueError('Invalid original private loopback endpoint')
    log_path = output / ('browser-server-' + str(launch) + '.jsonl')
    with log_path.open('x') as log:
        log_path.chmod(0o600)
        count = 0

        def event(value):
            nonlocal count
            count += 1
            if count > MAX_EVENTS:
                raise RuntimeError('Generated fixture event bound exceeded')
            log.write(json.dumps({'sequence': count, 'time_ns': time.time_ns(), **value}) + '\n')
            log.flush()

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, _format, *_arguments):
                pass

            def respond(self, status, content, content_type='text/plain', download=False):
                self.send_response(status)
                self.send_header('Content-Type', content_type)
                self.send_header('Content-Length', str(len(content)))
                self.send_header('Cache-Control', 'no-store')
                self.send_header('Content-Security-Policy', "default-src 'none'; script-src 'nonce-" + nonce + "'; style-src 'unsafe-inline'; connect-src 'self'")
                if download:
                    self.send_header('Content-Disposition', 'attachment; filename="dockpipe-browser-' + nonce[:8] + '.txt"')
                self.end_headers()
                self.wfile.write(content)

            def do_GET(self):
                selected = route(self.path, nonce)
                event({'method': 'GET', 'route': selected or 'refused'})
                if selected in ['a', 'b']:
                    self.respond(200, page(nonce, selected), 'text/html; charset=utf-8')
                elif selected == 'download.txt':
                    self.respond(200, payload(nonce), 'application/octet-stream', True)
                else:
                    self.respond(404, b'No generated fixture route')

            def do_POST(self):
                length = self.headers.get('Content-Length', '')
                if (route(self.path, nonce) != 'event' or not length.isdigit() or not 1 <= int(length) <= 1024
                        or self.headers.get('Origin') != 'http://127.0.0.1:' + str(self.server.server_port)):
                    self.respond(400, b'Invalid bounded fixture event')
                    return
                try:
                    value = json.loads(self.rfile.read(int(length)))
                except (ValueError, UnicodeError):
                    self.respond(400, b'Invalid fixture JSON')
                    return
                if (not isinstance(value, dict) or set(value) != {'event', 'page', 'present'}
                        or value['event'] not in ['storage', 'dom'] or value['page'] not in ['a', 'b']
                        or not isinstance(value['present'], bool)):
                    self.respond(400, b'Invalid generated DOM observation')
                    return
                event({'method': 'POST', **value})
                self.respond(200, b'ack')

        with HTTPServer(('127.0.0.1', port), Handler) as server:
            server.timeout = 1
            metadata = {'schema': SCHEMA, 'nonce': nonce, 'launch': launch, 'port': server.server_port,
                        'namespace_pid': os.getpid(), 'payload_sha256': hashlib.sha256(payload(nonce)).hexdigest()}
            ready = output / ('browser-server-' + str(launch) + '.json')
            temporary = output / ('browser-server-' + str(launch) + '.next')
            with temporary.open('x') as destination:
                json.dump(metadata, destination, indent=2)
                destination.write('\n')
            temporary.chmod(0o600)
            temporary.replace(ready)
            deadline = time.monotonic() + 1000
            while time.monotonic() < deadline and count < MAX_EVENTS and not stop_requested(output, nonce, launch):
                server.handle_request()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--launch', type=int, choices=[1, 2], required=True)
    arguments = parser.parse_args()
    if not arguments.output.is_absolute() or arguments.output.is_symlink() or not arguments.output.is_dir():
        parser.error('Use the owning private fixture output')
    serve(arguments.output, arguments.launch)


if __name__ == '__main__':
    main()
