import json
import threading
import time
import urllib.error
import urllib.request

from garda_ml.pipeline.kontrol import KeadaanKontrolPipeline, buat_server_kontrol


def _call(url: str, method: str = "GET", payload: dict | None = None) -> tuple[int, dict]:
    data = json.dumps(payload).encode("utf-8") if payload is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=3) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as err:
        return err.code, json.loads(err.read().decode("utf-8"))


def test_kontrol_pipeline_progress_dan_anti_double_run():
    def pelaksana(cb):
        cb("EKSTRAKSI_FITUR", 15, "Ekstraksi fitur...", "C20261124T11Z")
        time.sleep(0.12)
        cb("SIMPAN_DATASAFE", 95, "Menyimpan datasafe...", "C20261124T11Z")
        return {"cycle_id": "C20261124T11Z", "status": "BERHASIL"}

    keadaan = KeadaanKontrolPipeline(pelaksana=pelaksana)
    srv = buat_server_kontrol(keadaan, host="127.0.0.1", port=0)
    port = srv.server_address[1]
    t = threading.Thread(target=srv.serve_forever, daemon=True)
    t.start()
    try:
        c, h = _call(f"http://127.0.0.1:{port}/health")
        assert c == 200 and h["ok"] is True

        c, body = _call(f"http://127.0.0.1:{port}/run", "POST", {"job_id": "ML-1"})
        assert c == 202 and body["job_id"] == "ML-1"

        c2, err = _call(f"http://127.0.0.1:{port}/run", "POST", {})
        assert c2 == 409 and err["error"] == "SEDANG_BERJALAN"

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
        srv.shutdown()
        srv.server_close()
