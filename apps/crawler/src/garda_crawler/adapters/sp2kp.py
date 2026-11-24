"""SP2KP Kemendag adapter plugin (guarded until official open API credentials become available)."""

from collections.abc import Iterable

from garda_crawler.adapters.base import SkippedAdapter
from garda_crawler.config.source_specs import SP2KP_DATASET


class Sp2kpKemendagAdapter(SkippedAdapter):
    """Official Kemendag SP2KP adapter guarded behind reCAPTCHA/credential requirements."""

    source_code = "SP2KP"

    def __init__(self, api_key: str = "", regions: Iterable[str] | None = None) -> None:
        super().__init__(
            source_code="SP2KP",
            datasets=(SP2KP_DATASET,),
            reason="endpoint publik SP2KP Kemendag mensyaratkan verifikasi reCAPTCHA dan tidak punya API terbuka",
            action="ajukan akses resmi Kemendag bila tersedia",
            regions=regions,
        )
        self.api_key = api_key.strip()
