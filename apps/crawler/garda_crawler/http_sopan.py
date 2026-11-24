"""Klien HTTP sopan: jeda per host, retry dengan backoff, timeout, dan kepatuhan robots.txt.

Tidak ada opsi untuk melewati robots.txt atau mematikan verifikasi TLS. Pesan galat tidak
pernah memuat kunci API.
"""

import logging
import re
import ssl
import time
from collections.abc import Callable, Iterable
from dataclasses import dataclass, field
from typing import Any
from urllib.parse import urlencode, urlsplit

import certifi
import httpx
from tenacity import RetryCallState, Retrying, retry_if_exception_type, stop_after_attempt, wait_exponential_jitter

log = logging.getLogger(__name__)

# BMKG membatasi 60 permintaan per menit per IP.
JEDA_MIN_PER_HOST_MS = {"api.bmkg.go.id": 1100}
BATAS_BADAN = 32 * 1024 * 1024
TUNGGU_MAKS_DETIK = 120.0


class GalatAmbil(Exception):
    """Pengambilan gagal; pesannya aman dicatat."""


class GalatSementara(GalatAmbil):
    """Layak diulang: 429, 5xx, atau gangguan jaringan."""

    def __init__(self, pesan: str, tunggu: float | None = None) -> None:
        super().__init__(pesan)
        self.tunggu = tunggu


class GalatRobots(GalatAmbil):
    """robots.txt host melarang jalur ini."""


def _cocok(pola: str, jalur: str) -> bool:
    if "*" not in pola and not pola.endswith("$"):
        return jalur.startswith(pola)
    jangkar = pola.endswith("$")
    inti = pola[:-1] if jangkar else pola
    rx = "".join(".*" if c == "*" else re.escape(c) for c in inti)
    return re.match(rx + ("$" if jangkar else ""), jalur) is not None


@dataclass
class AturanRobots:
    """Aturan satu grup: aturan terpanjang menang, Allow menang bila seri."""

    izin: list[str] = field(default_factory=list)
    larang: list[str] = field(default_factory=list)

    def boleh(self, jalur: str) -> bool:
        jalur = jalur or "/"
        terpanjang_izin = max((len(p) for p in self.izin if p and _cocok(p, jalur)), default=-1)
        terpanjang_larang = max((len(p) for p in self.larang if p and _cocok(p, jalur)), default=-1)
        return terpanjang_larang < 0 or terpanjang_izin >= terpanjang_larang


def parse_robots(isi: str, agen: str) -> AturanRobots:
    grup: dict[str, AturanRobots] = {}
    aktif: list[AturanRobots] = []
    baru_mulai = True
    for baris in isi.splitlines():
        baris = baris.split("#", 1)[0]
        if ":" not in baris:
            continue
        kunci, nilai = (b.strip() for b in baris.split(":", 1))
        kunci = kunci.lower()
        if kunci == "user-agent":
            if baru_mulai:
                aktif = []
            baru_mulai = False
            aktif.append(grup.setdefault(nilai.lower(), AturanRobots()))
        elif kunci in ("allow", "disallow"):
            baru_mulai = True
            for g in aktif:
                (g.izin if kunci == "allow" else g.larang).append(nilai)
    token = agen.split("/", 1)[0].strip().lower()
    return grup.get(token) or grup.get("*") or AturanRobots()


def konteks_tls() -> ssl.SSLContext:
    """Selalu memverifikasi sertifikat memakai bundel certifi.

    SSL_CERT_FILE di macOS sering menunjuk bundel lama tanpa akar GlobalSign R46 yang dipakai bi.go.id.
    """
    return ssl.create_default_context(cafile=certifi.where())


class KlienSopan:
    def __init__(
        self,
        user_agent: str,
        jeda_ms: int = 1500,
        timeout_detik: float = 45.0,
        percobaan: int = 4,
        rahasia: Iterable[str] = (),
        transport: httpx.BaseTransport | None = None,
        tidur: Callable[[float], None] = time.sleep,
    ) -> None:
        self.user_agent = user_agent
        self.jeda_ms = jeda_ms
        self.percobaan = max(1, percobaan)
        self._rahasia = [r for r in rahasia if r]
        self._tidur = tidur
        self._terakhir: dict[str, float] = {}
        self._robots: dict[str, AturanRobots] = {}
        argumen: dict[str, Any] = {"transport": transport} if transport else {"verify": konteks_tls()}
        self._http = httpx.Client(
            timeout=timeout_detik,
            headers={"User-Agent": user_agent},
            follow_redirects=False,  # Pengalihan bisa membawa kunci API ke host lain.
            trust_env=False,
            **argumen,
        )

    def __enter__(self) -> "KlienSopan":
        return self

    def __exit__(self, *_: object) -> None:
        self.tutup()

    def tutup(self) -> None:
        self._http.close()

    def samarkan(self, teks: str) -> str:
        for r in self._rahasia:
            teks = teks.replace(r, "***")
        return teks

    def _lokasi(self, url: str) -> str:
        bagian = urlsplit(url)
        return self.samarkan(f"{bagian.scheme}://{bagian.netloc}{bagian.path}")

    def _tunggu_giliran(self, host: str) -> None:
        jeda = max(self.jeda_ms, JEDA_MIN_PER_HOST_MS.get(host, 0) if self.jeda_ms else 0) / 1000
        if jeda <= 0:
            return
        terakhir = self._terakhir.get(host)
        if terakhir is not None:
            sisa = terakhir + jeda - time.monotonic()
            if sisa > 0:
                self._tidur(sisa)
        self._terakhir[host] = time.monotonic()

    def _kirim(self, url: str, header: dict[str, str] | None = None) -> httpx.Response:
        lokasi = self._lokasi(url)
        self._tunggu_giliran(urlsplit(url).netloc)
        try:
            res = self._http.get(url, headers=header)
        except httpx.HTTPError as exc:
            # Pesan httpx bisa memuat URL lengkap, jadi yang disimpan hanya jenis galatnya.
            raise GalatSementara(f"GET {lokasi}: {type(exc).__name__}") from None
        if res.status_code == 429 or res.status_code >= 500:
            raise GalatSementara(f"GET {lokasi}: status {res.status_code}", _retry_after(res))
        return res

    def _dengan_retry(self, fungsi: Callable[[], httpx.Response]) -> httpx.Response:
        dasar = wait_exponential_jitter(multiplier=2, max=60)

        def tunggu(keadaan: RetryCallState) -> float:
            nilai = dasar(keadaan)
            galat = keadaan.outcome.exception() if keadaan.outcome else None
            if isinstance(galat, GalatSementara) and galat.tunggu:
                nilai = max(nilai, min(galat.tunggu, TUNGGU_MAKS_DETIK))
            return nilai

        pengulang = Retrying(
            stop=stop_after_attempt(self.percobaan),
            wait=tunggu,
            retry=retry_if_exception_type(GalatSementara),
            reraise=True,
            sleep=self._tidur,
        )
        return pengulang(fungsi)

    def _aturan(self, skema: str, host: str) -> AturanRobots:
        kunci = f"{skema}://{host}"
        if kunci in self._robots:
            return self._robots[kunci]
        res = self._dengan_retry(lambda: self._kirim(f"{kunci}/robots.txt"))
        if res.status_code == 200 and "html" not in res.headers.get("content-type", "").lower():
            aturan = parse_robots(res.text, self.user_agent)
        elif res.status_code == 200 or 400 <= res.status_code < 500:
            # Peladen SPA menjawab robots.txt dengan HTML; 4xx berarti tidak ada pembatasan (RFC 9309).
            aturan = AturanRobots()
        else:
            raise GalatAmbil(f"robots.txt {host}: status {res.status_code}")
        self._robots[kunci] = aturan
        return aturan

    def boleh(self, url: str) -> bool:
        bagian = urlsplit(url)
        jalur = bagian.path + (f"?{bagian.query}" if bagian.query else "")
        return self._aturan(bagian.scheme, bagian.netloc).boleh(jalur)

    def get_json(self, url: str, params: dict[str, Any] | None = None) -> Any:
        lengkap = f"{url}?{urlencode(params)}" if params else url
        if not self.boleh(lengkap):
            raise GalatRobots(f"{self._lokasi(url)} dilarang robots.txt")
        res = self._dengan_retry(lambda: self._kirim(lengkap, {"Accept": "application/json"}))
        lokasi = self._lokasi(url)
        if not 200 <= res.status_code < 300:
            raise GalatAmbil(f"GET {lokasi}: status {res.status_code}")
        if len(res.content) > BATAS_BADAN:
            raise GalatAmbil(f"GET {lokasi}: respons melebihi batas ukuran")
        try:
            return res.json()
        except ValueError:
            raise GalatAmbil(f"GET {lokasi}: respons bukan JSON") from None


def _retry_after(res: httpx.Response) -> float | None:
    try:
        return float(res.headers.get("retry-after", ""))
    except ValueError:
        return None
