from __future__ import annotations

import json
import os
import socket
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


REGION = os.environ.get("REGION", "unknown")
PORT = int(os.environ.get("PORT", "8080"))
START_TIME = time.time()
HOSTNAME = socket.gethostname()


def metrics_text() -> str:
    uptime = max(time.time() - START_TIME, 0.0)
    return "\n".join(
        [
            "# HELP dslab_probe_info Static identity information for the probe.",
            "# TYPE dslab_probe_info gauge",
            f'dslab_probe_info{{region="{REGION}",hostname="{HOSTNAME}"}} 1',
            "# HELP dslab_probe_uptime_seconds Process uptime in seconds.",
            "# TYPE dslab_probe_uptime_seconds gauge",
            f"dslab_probe_uptime_seconds {uptime:.3f}",
            "",
        ]
    )


class Handler(BaseHTTPRequestHandler):
    server_version = "dslab-probe/0.1"

    def _send_json(self, payload: dict[str, object], status: int = 200) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:  # noqa: N802
        if self.path in ("/", "/health"):
            self._send_json(
                {
                    "ok": True,
                    "region": REGION,
                    "hostname": HOSTNAME,
                    "uptime_seconds": round(max(time.time() - START_TIME, 0.0), 3),
                }
            )
            return

        if self.path == "/metrics":
            body = metrics_text().encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; version=0.0.4")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return

        self._send_json({"ok": False, "error": "not_found"}, status=404)

    def log_message(self, fmt: str, *args: object) -> None:
        return


def main() -> None:
    server = ThreadingHTTPServer(("0.0.0.0", PORT), Handler)
    server.serve_forever()


if __name__ == "__main__":
    main()

