"""Adapter sumber data."""

from collections.abc import Collection

from garda_crawler.konfigurasi import Konfigurasi
from garda_crawler.sumber.bmkg import AdapterBMKG
from garda_crawler.sumber.bps import AdapterBPS
from garda_crawler.sumber.dasar import Adapter
from garda_crawler.sumber.open_meteo import AdapterOpenMeteo
from garda_crawler.sumber.pendukung import AdapterBapanas, AdapterSP2KP
from garda_crawler.sumber.pihps import AdapterPIHPS

LAPANGAN = "LAPANGAN"
SUMBER_CRAWL = ("PIHPS", "BMKG", "OPEN_METEO", "BPS", "SP2KP", "BAPANAS")
SEMUA_SUMBER = (*SUMBER_CRAWL, LAPANGAN)


def buat_adapter(sumber: str, cfg: Konfigurasi, wilayah: Collection[str] | None = None) -> Adapter:
    if sumber == "PIHPS":
        return AdapterPIHPS(wilayah)
    if sumber == "BMKG":
        return AdapterBMKG(wilayah)
    if sumber == "OPEN_METEO":
        return AdapterOpenMeteo(wilayah)
    if sumber == "BPS":
        return AdapterBPS(cfg.bps_api_key, cfg.bps_variabel, wilayah)
    if sumber == "SP2KP":
        return AdapterSP2KP()
    if sumber == "BAPANAS":
        return AdapterBapanas()
    raise ValueError(f"sumber tidak dikenal: {sumber}")
