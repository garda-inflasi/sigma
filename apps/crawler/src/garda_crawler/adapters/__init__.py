"""Isolated source adapter plugins and registry."""

from garda_crawler.adapters.bapanas import BapanasPanelAdapter, parse_bapanas_panel
from garda_crawler.adapters.base import BaseAdapter, SkippedAdapter, format_skip_message
from garda_crawler.adapters.bmkg import BmkgForecastAdapter, parse_bmkg_forecast
from garda_crawler.adapters.bps import BpsWebApiAdapter, lookup_var_id, parse_bps_payload, strip_bps_key
from garda_crawler.adapters.field_entries import sync_field_entries, write_field_carried_forward
from garda_crawler.adapters.open_meteo import OpenMeteoArchiveAdapter, parse_open_meteo_archive
from garda_crawler.adapters.pihps import PihpsBiAdapter, parse_pihps_grid
from garda_crawler.adapters.registry import (
    DAILY_ORDER,
    DEFAULT_ACTIVE_SOURCES,
    DEFAULT_REGISTRY,
    SourceRegistry,
    build_default_registry,
    create_adapter,
)
from garda_crawler.adapters.sp2kp import Sp2kpKemendagAdapter

__all__ = [
    "BapanasPanelAdapter",
    "BaseAdapter",
    "BmkgForecastAdapter",
    "BpsWebApiAdapter",
    "DAILY_ORDER",
    "DEFAULT_ACTIVE_SOURCES",
    "DEFAULT_REGISTRY",
    "OpenMeteoArchiveAdapter",
    "PihpsBiAdapter",
    "SkippedAdapter",
    "SourceRegistry",
    "Sp2kpKemendagAdapter",
    "build_default_registry",
    "create_adapter",
    "format_skip_message",
    "lookup_var_id",
    "parse_bapanas_panel",
    "parse_bmkg_forecast",
    "parse_bps_payload",
    "parse_open_meteo_archive",
    "parse_pihps_grid",
    "strip_bps_key",
    "sync_field_entries",
    "write_field_carried_forward",
]
