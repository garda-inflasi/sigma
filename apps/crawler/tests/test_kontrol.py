import json
import time
import urllib.error
import urllib.request

from garda_crawler.kontrol import KeadaanKontrolCrawler, buat_server_kontrol


def _call(url: str, method: str = "GET", payload: dict | None = None) -> tuple[int, dict]:
    data = json.dumps(payload).encode("utf-8") if payload is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=3) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as err:
        return err.code, json.loads(err.read().decode("utf-8"))


def test_kontrol_status_run_dan_tolak_ganda():
    dipanggil: list[list[str]] = []

    def pelaksana(sumber: list[str]) -> list[dict]:
        dipanggil.append(list(sumber))
        time.sleep(0.15)
        return [{"sumber": s, "dataset": "test", "status": "FRESH", "jumlah_baru": 3} for s in sumber]

    keadaan = KeadaanKontrolCrawler(db=None, pemilik="uji-1", pelaksana=pelaksana)
    srv = buat_server_kontrol(keadaan, host="127.0.0.1", port=0)
    port = srv.server_address[1]
    import threading

    t = threading.Thread(target=srv.serve_forever, daemon=True)
    t.start()
    try:
        code, health = _call(f"http://127.0.0.1:{port}/health")
        assert code == 200 and health["ok"] is True

        code, body = _call(f"http://127.0.0.1:{port}/run", "POST", {"sumber": ["BPS"], "job_id": "JOB-1"})
        assert code == 202
        assert body["job_id"] == "JOB-1"
        assert body["sumber"] == ["BPS"]

        # Panggilan kedua selagi berjalan harus ditolak 409 anti double-run.
        code2, body2 = _call(f"http://127.0.0.1:{port}/run", "POST", {"sumber": ["BI"]})
        assert code2 == 409
        assert body2["error"] == "SEDANG_BERJALAN"

        # Tunggu selesai.
        for _ in range(30):
            _, snap = _call(f"http://127.0.0.1:{port}/status")
            if not snap["berjalan"]:
                break
            time.sleep(0.05)

        _, snap = _call(f"http://127.0.0.1:{port}/status")
        assert snap["berjalan"] is False
        assert snap["tahap"] == "SELESAI"
        assert snap["progress_pct"] == 100
        assert dipanggil == [["BPS"]]
    finally:
        srv.shutdown()
        srv.server_close()
