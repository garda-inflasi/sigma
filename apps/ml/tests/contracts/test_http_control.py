"""Contract tests for internal HTTP control server (:8092) and worker CLI."""

import json
import threading
import time
import urllib.error
import urllib.request

from garda_ml.orchestration.control_server import PipelineControlState, create_control_server


def _call(url: str, method: str = "GET", payload: dict | None = None) -> tuple[int, dict]:
    data = json.dumps(payload).encode("utf-8") if payload is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=3) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as err:
        return err.code, json.loads(err.read().decode("utf-8"))


def test_control_http_progress_and_anti_double_run():
    def executor(cb):
        cb("EKSTRAKSI_FITUR", 15, "Ekstraksi fitur...", "C20261124T11Z")
        time.sleep(0.12)
        cb("SIMPAN_DATASAFE", 95, "Menyimpan datasafe...", "C20261124T11Z")
        return {"cycle_id": "C20261124T11Z", "status": "BERHASIL"}

    state = PipelineControlState(executor=executor)
    server = create_control_server(state, host="127.0.0.1", port=0)
    port = server.server_address[1]
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        code, health = _call(f"http://127.0.0.1:{port}/health")
        assert code == 200 and health["ok"] is True

        code, accepted = _call(f"http://127.0.0.1:{port}/run", "POST", {"job_id": "ML-1"})
        assert code == 202 and accepted["job_id"] == "ML-1"

        conflict_code, err_body = _call(f"http://127.0.0.1:{port}/run", "POST", {})
        assert conflict_code == 409 and err_body["error"] == "SEDANG_BERJALAN"

        for _ in range(30):
            _, snap = _call(f"http://127.0.0.1:{port}/status")
            if not snap["berjalan"]:
                break
            time.sleep(0.05)

        _, snap = _call(f"http://127.0.0.1:{port}/status")
        assert snap["berjalan"] is False
        assert snap["tahap"] == "SELESAI"
        assert snap["progress_pct"] == 100
        assert snap["cycle_id"] == "C20261124T11Z"
    finally:
        server.shutdown()
        server.server_close()
