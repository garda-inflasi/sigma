"""Unit tests for PoliteHttpClient rate limiting, robots.txt enforcement, retries, and secret redaction."""

import httpx
import pytest

from garda_crawler.domain.errors import HttpFetchError, RobotsDisallowedError, TransientFetchError
from garda_crawler.transport.polite_http import parse_robots
from tests.conftest import make_http_client, without_robots


def test_robots_open_meteo_disallows_all():
    policy = parse_robots("User-agent: *\nDisallow: /", "garda-crawler/1.0")
    assert not policy.is_allowed("/v1/archive?latitude=5.55")


def test_robots_bi_longest_match_wins():
    policy = parse_robots("User-agent: *\nAllow: /\nDisallow:/_layouts\n", "garda-crawler/1.0")
    assert policy.is_allowed("/hargapangan/WebSite/TabelHarga/GetGridDataDaerah")
    assert not policy.is_allowed("/_layouts/x")


def test_robots_bmkg_specific_group_isolation():
    content = "User-agent: *\nDisallow: /api\nDisallow: /alerts\n\nUser-agent: Go-http-client\nUser-agent: dotbot\nDisallow: /\n"
    policy = parse_robots(content, "garda-crawler/1.0")
    assert not policy.is_allowed("/alerts/nowcast/id")
    assert policy.is_allowed("/cuaca")
    assert not parse_robots(content, "Go-http-client/2.0").is_allowed("/cuaca")


def test_disallowed_path_never_hits_endpoint():
    calls: list[str] = []

    def handler(req: httpx.Request) -> httpx.Response:
        calls.append(req.url.path)
        if req.url.path == "/robots.txt":
            return httpx.Response(200, text="User-agent: *\nDisallow: /", headers={"content-type": "text/plain"})
        return httpx.Response(200, json={})

    with make_http_client(handler) as client, pytest.raises(RobotsDisallowedError):
        client.get_json("https://contoh.test/data")
    assert calls == ["/robots.txt"]


def test_robots_5xx_fails_closed():
    calls: list[str] = []

    def handler(req: httpx.Request) -> httpx.Response:
        calls.append(req.url.path)
        return httpx.Response(503)

    with make_http_client(handler) as client, pytest.raises(TransientFetchError):
        client.get_json("https://contoh.test/data")
    assert "/data" not in calls


def test_retry_succeeds_with_generic_user_agent():
    user_agents: list[str] = []

    def handler(req: httpx.Request) -> httpx.Response:
        user_agents.append(req.headers["user-agent"])
        if len(user_agents) < 3:
            return httpx.Response(503)
        return httpx.Response(200, json={"ok": True})

    with make_http_client(without_robots(handler), max_attempts=4) as client:
        assert client.get_json("https://contoh.test/data") == {"ok": True}
    assert user_agents == ["garda-crawler/test"] * 3


def test_4xx_does_not_retry():
    calls: list[int] = []

    def handler(_req: httpx.Request) -> httpx.Response:
        calls.append(1)
        return httpx.Response(401, json={"message": "unauthorized"})

    with make_http_client(without_robots(handler), max_attempts=4) as client, pytest.raises(HttpFetchError):
        client.get_json("https://contoh.test/data")
    assert len(calls) == 1


def test_secrets_are_redacted_from_errors():
    secret = "KUNCIRAHASIA123"

    def handler(req: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError(f"failed reaching {req.url}")

    with make_http_client(without_robots(handler), secrets=[secret]) as client:
        with pytest.raises(HttpFetchError) as exc_info:
            client._robots_cache["https://contoh.test"] = parse_robots("", "x")
            client.get_json(f"https://contoh.test/data/key/{secret}")
    assert secret not in str(exc_info.value)
    assert "***" in str(exc_info.value)


def test_redirects_are_not_followed():
    hosts: list[str] = []

    def handler(req: httpx.Request) -> httpx.Response:
        hosts.append(req.url.host)
        return httpx.Response(302, headers={"location": "https://lain.test/x"})

    with make_http_client(without_robots(handler)) as client, pytest.raises(HttpFetchError):
        client.get_json("https://contoh.test/data/key/SECRET")
    assert hosts == ["contoh.test"]
