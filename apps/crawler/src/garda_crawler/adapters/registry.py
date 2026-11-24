"""Source plugin registry mapping canonical source codes to adapter factories."""

from collections.abc import Callable, Iterable
from dataclasses import dataclass

from garda_crawler.adapters.bapanas import BapanasPanelAdapter
from garda_crawler.adapters.bmkg import BmkgForecastAdapter
from garda_crawler.adapters.bps import BpsWebApiAdapter
from garda_crawler.adapters.open_meteo import OpenMeteoArchiveAdapter
from garda_crawler.adapters.pihps import PihpsBiAdapter
from garda_crawler.adapters.sp2kp import Sp2kpKemendagAdapter
from garda_crawler.config.settings import CrawlerSettings
from garda_crawler.domain.protocols import SourceAdapter

AdapterFactory = Callable[[CrawlerSettings, Iterable[str] | None], SourceAdapter]

DAILY_ORDER: tuple[str, ...] = ("PIHPS", "BMKG", "OPEN_METEO", "BPS", "SP2KP", "BAPANAS")
DEFAULT_ACTIVE_SOURCES: tuple[str, ...] = ("PIHPS", "BMKG", "OPEN_METEO", "BPS", "SP2KP", "BAPANAS")


@dataclass(frozen=True)
class RegisteredSource:
    """Registry entry holding an adapter's factory and backfill support metadata."""

    source_code: str
    factory: AdapterFactory
    supports_backfill: bool = False


class SourceRegistry:
    """Extensible registry for isolated source adapter plugins."""

    def __init__(self) -> None:
        self._entries: dict[str, RegisteredSource] = {}

    def register(
        self,
        source_code: str,
        factory: AdapterFactory,
        *,
        supports_backfill: bool = False,
    ) -> None:
        """Register or replace an adapter plugin for `source_code`."""
        normalized = source_code.strip().upper()
        self._entries[normalized] = RegisteredSource(
            source_code=normalized,
            factory=factory,
            supports_backfill=supports_backfill,
        )

    def create(
        self,
        source_code: str,
        settings: CrawlerSettings,
        regions: Iterable[str] | None = None,
    ) -> SourceAdapter:
        """Instantiate an adapter plugin by source code."""
        normalized = source_code.strip().upper()
        entry = self._entries.get(normalized)
        if entry is None:
            known = ", ".join(sorted(self._entries))
            raise KeyError(f"sumber tidak dikenal: {source_code!r} (pilihan: {known})")
        return entry.factory(settings, regions)

    def all_source_codes(self) -> tuple[str, ...]:
        """Return registered source codes in canonical execution order."""
        ordered = [code for code in DAILY_ORDER if code in self._entries]
        extras = sorted(code for code in self._entries if code not in DAILY_ORDER)
        return (*ordered, *extras)

    def supports_backfill(self, source_code: str) -> bool:
        """Return whether the registered source claims historical backfill support."""
        entry = self._entries.get(source_code.strip().upper())
        return bool(entry and entry.supports_backfill)


def build_default_registry() -> SourceRegistry:
    """Construct the default source plugin registry."""
    registry = SourceRegistry()
    registry.register(
        "PIHPS",
        lambda _cfg, regions: PihpsBiAdapter(regions=regions),
        supports_backfill=True,
    )
    registry.register(
        "BMKG",
        lambda _cfg, regions: BmkgForecastAdapter(regions=regions),
        supports_backfill=False,
    )
    registry.register(
        "OPEN_METEO",
        lambda _cfg, regions: OpenMeteoArchiveAdapter(regions=regions),
        supports_backfill=True,
    )
    registry.register(
        "BPS",
        lambda cfg, regions: BpsWebApiAdapter(
            api_key=cfg.bps_api_key,
            variables=cfg.bps_variables,
            regions=regions,
        ),
        supports_backfill=True,
    )
    registry.register(
        "BAPANAS",
        lambda cfg, regions: BapanasPanelAdapter(
            api_key=cfg.bapanas_api_key,
            base_url=cfg.bapanas_base_url,
            regions=regions,
        ),
        supports_backfill=False,
    )
    registry.register(
        "SP2KP",
        lambda cfg, regions: Sp2kpKemendagAdapter(api_key=cfg.sp2kp_api_key, regions=regions),
        supports_backfill=False,
    )
    return registry


DEFAULT_REGISTRY = build_default_registry()


def create_adapter(
    source_code: str,
    settings: CrawlerSettings,
    regions: Iterable[str] | None = None,
) -> SourceAdapter:
    """Convenience helper to instantiate a registered adapter from the default registry."""
    return DEFAULT_REGISTRY.create(source_code, settings, regions)
