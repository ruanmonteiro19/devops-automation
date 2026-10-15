#!/usr/bin/env python3
"""Isolated real-server SNMP-exporter HTTP integration and topology smoke.
Usage: python3 scripts/smoke-integrations.py --binary /absolute/path/to/observa
No real cloud/device credentials or external endpoints are used.
"""
import argparse
import http.server
import json
import os
import secrets
import socket
import subprocess
import tempfile
import threading
import time
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True)
    args = parser.parse_args()
    binary = os.path.abspath(args.binary)
    admin, agent = secrets.token_hex(24), secrets.token_hex(24)

    class Exporter(http.server.BaseHTTPRequestHandler):
        fail = False
        calls = 0

        def log_message(self, *args):
            pass

        def do_GET(self):
            if self.fail:
                self.send_response(403)
                self.end_headers()
                self.wfile.write(b'no credentials must be echoed')
                return
            type(self).calls += 1
            self.send_response(200)
            self.end_headers()
            self.wfile.write(('sysName{sysName="core-lab"} 1\n'
                              'ifOperStatus{ifIndex="1",ifName="Gi1"} 1\n'
                              'ifHighSpeed{ifIndex="1"} 1000\n'
                              'ifHCInOctets{ifIndex="1"} %d\n' % (1000 * self.calls)).encode())

    exporter = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Exporter)
    threading.Thread(target=exporter.serve_forever, daemon=True).start()
    with socket.socket() as reservation:
        reservation.bind(('127.0.0.1', 0))
        port = reservation.getsockname()[1]
    base = f'http://127.0.0.1:{port}'
    env = dict(os.environ, OBSERVA_ADMIN_TOKEN=admin, OBSERVA_AGENT_TOKEN=agent,
               OBSERVA_ALLOW_PRIVATE_TARGETS='true')
    env.pop('OBSERVA_TLS_CERT_FILE', None)
    env.pop('OBSERVA_TLS_KEY_FILE', None)
    process = None
    with tempfile.TemporaryDirectory(prefix='observa-integration-smoke-') as tmp:
        env['OBSERVA_DB'] = os.path.join(tmp, 'state.json')

        def request(path, method='GET', data=None):
            raw = None if data is None else json.dumps(data).encode()
            req = urllib.request.Request(base + path, data=raw, method=method,
                                         headers={'X-API-Key': admin, 'Content-Type': 'application/json'})
            with urllib.request.urlopen(req, timeout=10) as response:
                return json.load(response)

        def start():
            nonlocal process
            process = subprocess.Popen([binary, 'server', '--listen', f'127.0.0.1:{port}'],
                                       env=env, cwd=tmp, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                try:
                    if request('/healthz')['ok']:
                        return
                except OSError:
                    time.sleep(.05)
            raise AssertionError('server did not become ready')

        def stop():
            if process is not None:
                process.terminate()
                process.wait(timeout=10)

        def collect(previous=None):
            request('/api/v1/integrations/switch/collect', 'POST', {})
            deadline = time.monotonic() + 5
            while time.monotonic() < deadline:
                result = request('/api/v1/integrations')['results'].get('switch')
                if result and result['at'] != previous:
                    return result
                time.sleep(.05)
            raise AssertionError('async collection did not finish')

        try:
            start()
            profile = dict(id='switch', name='Core Lab', kind='snmp_exporter', enabled=True,
                           endpoint=f'http://127.0.0.1:{exporter.server_port}', target='192.0.2.10',
                           module='if_mib', auth='lab')
            request('/api/v1/integrations', 'POST', profile)
            first = collect()
            assert first['status'] == 'ok' and first['resources'][0]['name'] == 'core-lab'
            time.sleep(1.1)
            second = collect(first['at'])
            metrics = second['resources'][0]['interfaces'][0]['metrics']
            assert metrics['observa_in_bits_per_second'] > 0
            topology = request('/api/v1/topology?view=devices')
            assert len(topology['nodes']) == 3 and len(topology['edges']) == 2
            assert all(edge['kind'] == 'contains' for edge in topology['edges'])
            Exporter.fail = True
            failed = collect(second['at'])
            assert failed['status'] == 'error' and failed['resources'] == second['resources']
            assert failed['success_at'] == second['success_at']
            assert 'no credentials' not in failed['error']
            assert all(node['status'] == 'stale' for node in request('/api/v1/topology?view=devices')['nodes'])
            stop()
            start()
            restored = request('/api/v1/integrations')
            assert restored['results']['switch']['resources'][0]['name'] == 'core-lab'
            request('/api/v1/integrations/switch', 'DELETE')
            assert request('/api/v1/topology?view=devices')['nodes'] == []
            print('PASS: exporter HTTP → profile → async inventory → rates → topology → stale retention → restart → delete')
        finally:
            stop()
            exporter.shutdown()


if __name__ == '__main__':
    main()
