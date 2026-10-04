#!/usr/bin/env python3
"""Request-counting forwarding proxy for xo performance checks.

Forwards every request verbatim to the upstream (the xo-api-sim) and appends
one line — `METHOD /path` — per request to ./reqs.log, so a script can count
the HTTP requests a single `xo` invocation made. It is used to verify the
constant-cost (anti-N+1) promise of the `list`/`get` views: see the
"Performance" section of docs/development.md.

Usage:
    python3 ci/perf/proxy.py [listen_port] [upstream_host:port]

Defaults: listen on 127.0.0.1:3002, forward to 127.0.0.1:3001 (the
simulator's default port).
"""
import http.client
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

LISTEN = ("127.0.0.1", int(sys.argv[1]) if len(sys.argv) > 1 else 3002)
UP = tuple(sys.argv[2].split(":")) if len(sys.argv) > 2 else ("127.0.0.1", "3001")
LOG = open("reqs.log", "a", buffering=1)


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def do(self):
        LOG.write(f"{self.command} {self.path}\n")
        length = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(length) if length else None
        conn = http.client.HTTPConnection(*UP, timeout=10)
        headers = {k: v for k, v in self.headers.items() if k.lower() != "host"}
        conn.request(self.command, self.path, body, headers)
        resp = conn.getresponse()
        data = resp.read()
        self.send_response(resp.status)
        for k, v in resp.getheaders():
            if k.lower() in ("transfer-encoding", "connection"):
                continue
            self.send_header(k, v)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)
        conn.close()

    do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = do

    def log_message(self, *a):
        pass


ThreadingHTTPServer(LISTEN, Handler).serve_forever()
