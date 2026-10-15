#!/usr/bin/env python3
"""Verify a real isolated local server, three OTLP signals, analysis and restart.
Usage: python3 scripts/smoke-apm.py --binary /absolute/path/to/observa
Creates temporary lab credentials/data. Never uses an existing deployment.
"""
import argparse
import datetime as dt
import json
import os
import re
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True)
    args = parser.parse_args()
    binary = str(Path(args.binary).resolve())
    admin, agent = secrets.token_hex(24), secrets.token_hex(24)
    trace, root, dbspan = secrets.token_hex(16), secrets.token_hex(8), secrets.token_hex(8)
    with socket.socket() as reservation:
        reservation.bind(('127.0.0.1', 0))
        port = reservation.getsockname()[1]
    base = f'http://127.0.0.1:{port}'
    env = dict(os.environ, OBSERVA_ADMIN_TOKEN=admin, OBSERVA_AGENT_TOKEN=agent)
    env.pop('OBSERVA_TLS_CERT_FILE', None)
    env.pop('OBSERVA_TLS_KEY_FILE', None)
    with tempfile.TemporaryDirectory(prefix='observa-apm-smoke-') as tmp:
        process = None

        def start():
            nonlocal process
            process = subprocess.Popen([binary, 'server', '--listen', f'127.0.0.1:{port}', '--data', tmp+'/state.json'], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            for _ in range(50):
                if process.poll() is not None:
                    raise RuntimeError('Server stopped during startup')
                try:
                    urllib.request.urlopen(base+'/healthz', timeout=1).close()
                    return
                except OSError:
                    time.sleep(.1)
            raise RuntimeError('Server startup timed out')

        def stop():
            if process is not None and process.poll() is None:
                process.terminate()
                process.wait(timeout=10)

        def request(path, method='GET', body=None, token=None):
            payload = json.dumps(body).encode() if body is not None else None
            req = urllib.request.Request(base+path, data=payload, method=method, headers={'Content-Type': 'application/json', 'X-API-Key': token or admin})
            with urllib.request.urlopen(req, timeout=10) as response:
                return json.load(response)

        attrs = lambda **values: [{'key': k, 'value': {'stringValue': v}} for k, v in values.items()]
        resource = {'attributes': attrs(**{'service.name': 'smoke-checkout', 'service.namespace': 'qa', 'service.instance.id': 'smoke-1', 'service.version': '1.0'})}
        service = 'svc:qa:smoke-checkout'
        now = time.time_ns()-3_000_000_000
        try:
            start()
            with urllib.request.urlopen(base+'/', timeout=5) as response:
                html=response.read().decode()
            assets=re.findall(r'(?:src|href)="(/assets/[^" ]+)"',html)
            assert assets, 'Build must include the compiled React UI, not the legacy fallback'
            for asset in assets:
                with urllib.request.urlopen(base+asset,timeout=5) as response:
                    assert response.status==200 and len(response.read())>0
            spans = [
                {'traceId': trace, 'spanId': root, 'name': 'POST /orders', 'kind': 2, 'startTimeUnixNano': str(now), 'endTimeUnixNano': str(now+250_000_000), 'status': {'code': 2}, 'attributes': attrs(**{'http.route': '/orders'})},
                {'traceId': trace, 'spanId': dbspan, 'parentSpanId': root, 'name': 'SELECT orders', 'kind': 3, 'startTimeUnixNano': str(now), 'endTimeUnixNano': str(now+30_000_000), 'attributes': attrs(**{'db.system.name': 'postgresql', 'db.namespace': 'orders', 'db.operation.name': 'SELECT'})},
            ]
            request('/v1/traces', 'POST', {'resourceSpans': [{'resource': resource, 'scopeSpans': [{'spans': spans}]}]}, agent)
            request('/v1/logs', 'POST', {'resourceLogs': [{'resource': resource, 'scopeLogs': [{'logRecords': [{'timeUnixNano': str(now), 'severityNumber': 17, 'severityText': 'ERROR', 'traceId': trace, 'spanId': root, 'body': {'stringValue': 'Smoke test evidence'}}]}]}]}, agent)
            request('/v1/metrics', 'POST', {'resourceMetrics': [{'resource': resource, 'scopeMetrics': [{'metrics': [{'name': 'smoke.queue.depth', 'unit': '{item}', 'gauge': {'dataPoints': [{'timeUnixNano': str(now), 'asInt': '7'}]}}]}]}]}, agent)
            svc = request('/api/v1/apm/services')['services'][0]
            assert (svc['requests'], svc['error_rate'], svc['p95_ms']) == (1, 100, 250), svc
            analysis = request('/api/v1/apm/analysis?service='+service)
            assert len(analysis['database_calls']) == 1 and len(analysis['versions']) == 1
            logs = request('/api/v1/telemetry/logs?trace_id='+trace)['logs']
            assert logs[0]['body'] == 'Smoke test evidence'
            assert request('/api/v1/telemetry/metrics?name=smoke.queue.depth')['metrics'][0]['data']['asInt'] == '7'
            request('/api/v1/slos', 'POST', {'id': 'smoke-availability', 'name': 'Smoke availability', 'service_id': service, 'kind': 'availability', 'target_percent': 99.5})
            assert request('/api/v1/slos')['slos'][0]['status'] == 'breached'
            request('/api/v1/apm/rules', 'POST', {'id': 'smoke-error-rate', 'name': 'Smoke error rate', 'service_id': service, 'metric': 'error_rate', 'threshold': 50, 'min_requests': 1, 'window_minutes': 5, 'severity': 'critical', 'enabled': True})
            request('/api/v1/apm/rules/evaluate', 'POST')
            assert len(request('/api/v1/apm/problems?status=active')['problems']) == 1
            request('/api/v1/dashboards', 'POST', {'id': 'smoke-dashboard', 'title': 'Smoke dashboard', 'panels': [{'id': 'requests', 'title': 'Requests', 'kind': 'apm-stat', 'source_id': 'observa-apm', 'query': json.dumps({'service_id': service, 'metric': 'requests'})}]})
            assert request('/api/v1/snapshot')['dashboards'][0]['id'] == 'smoke-dashboard'
            stop()
            start()
            assert request('/api/v1/apm/services')['services'][0]['requests'] == 1
            assert request('/api/v1/telemetry/logs?trace_id='+trace)['logs'][0]['trace_id'] == trace
            assert len(request('/api/v1/apm/problems?status=active')['problems']) == 1
            assert request('/api/v1/slos')['slos'][0]['status'] == 'breached'
            assert request('/api/v1/snapshot')['dashboards'][0]['id'] == 'smoke-dashboard'
            request('/api/v1/dashboards/smoke-dashboard', 'DELETE')
            request('/api/v1/slos/smoke-availability', 'DELETE')
            request('/api/v1/apm/rules/smoke-error-rate', 'DELETE')
            assert request('/api/v1/apm/problems')['problems'][0]['status'] == 'resolved'
            print('PASS: traces + logs + metrics → analysis → SLO → rule/problem → dashboard → restart → delete/recovery')
        finally:
            stop()


if __name__ == '__main__':
    main()
