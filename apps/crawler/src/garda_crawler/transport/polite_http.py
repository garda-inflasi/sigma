"""Polite HTTP client with per-host rate limiting, robots.txt cache, retries, and secret redaction."""

import asyncio
import random
import re
import time
from collections.abc import Callable, Iterable
from dataclasses import dataclass
from typing import Any
from urllib.parse import urljoin, urlsplit

import certifi
import httpx
from tenacity import retry, retry_if_exception_type, stop_after_attempt

from garda_crawler.domain.errors import (
    HttpFetchError,
    RobotsDisallowedError,
    SchemaValidationError,
    TransientFetchError,
)

# BMKG enforces a documented ceiling (<= 60 req/min per IP); stay safely below it.
MIN_HOST_DELAY_MS: dict[str, int] = {
    "api.bmkg.go.id": 1100,
}

_SENSITIVE_QUERY_RE = re.compile(r"((?:key|api_key|token)=)[^&\s]+", re.IGNORECASE)
_SENSITIVE_PATH_RE = re.compile(r"(/key/)[^/\s]+", re.IGNORECASE)


def redact_secrets(text: str, secrets: Iterable[str] = ()) -> str:
    """Scrub registered credentials and common query/path API keys from strings."""
    cleaned = _SENSITIVE_QUERY_RE.sub(r"\1***", text)
    cleaned = _SENSITIVE_PATH_RE.sub(r"\1***", cleaned)
    for secret in secrets:
        if secret:
            cleaned = cleaned.replace(secret, "***")
    return cleaned


@dataclass(frozen=True)
class RobotsPolicy:
    """Evaluates Allow/Disallow directives matched by User-Agent using longest prefix precedence."""

    rules: tuple[tuple[bool, str], ...]

    def is_allowed(self, path_and_query: str) -> bool:
        """Return False if any matching Disallow rule outranks Allow rules."""
        best_length = -1
        allowed = True
        for is_allow, pattern in self.rules:
            if not pattern:
                continue
            if path_and_query.startswith(pattern) and len(pattern) > best_length:
                best_length = len(pattern)
                allowed = is_allow
        return allowed


def _agent_tokens(user_agent: str) -> set[str]:
    base = user_agent.strip().lower()
    product = base.split("/", 1)[0].split(" ", 1)[0]
    return {t for t in (base, product) if t}


def parse_robots(content: str, user_agent: str) -> RobotsPolicy:
    """Parse robots.txt content selecting the most specific matching agent group."""
    groups: list[tuple[list[str], list[tuple[bool, str]]]] = []
    agents: list[str] = []
    rules: list[tuple[bool, str]] = []
    saw_rule = False

    def flush_group() -> None:
        nonlocal agents, rules, saw_rule
        if agents:
            groups.append((agents, rules))
        agents, rules, saw_rule = [], [], False

    for raw_line in content.splitlines():
        line = raw_line.split("#", 1)[0].strip()
        if not line or ":" not in line:
            continue
        directive, value = (part.strip() for part in line.split(":", 1))
        lowered = directive.lower()
        if lowered == "user-agent":
            if saw_rule:
                flush_group()
            agents.append(value.lower())
        elif lowered in {"allow", "disallow"} and agents:
            saw_rule = True
            rules.append((lowered == "allow", value))
    flush_group()

    tokens = _agent_tokens(user_agent)
    specific = [rule for group_agents, group_rules in groups for rule in group_rules if any(a in tokens for a in group_agents)]
    if specific:
        return RobotsPolicy(tuple(specific))
    wildcard = [rule for group_agents, group_rules in groups for rule in group_rules if "*" in group_agents]
    return RobotsPolicy(tuple(wildcard))


def _retry_delay_seconds(exc: BaseException, attempt: int) -> float:
    if isinstance(exc, httpx.HTTPStatusError) and exc.response is not None:
        retry_after = exc.response.headers.get("retry-after", "").strip()
        if retry_after.isdigit():
            return min(60.0, float(retry_after))
    base = min(30.0, float(2 ** max(0, attempt - 1)))
    return base + random.uniform(0.05, 0.35)


class PoliteHttpClient:
    """Synchronous HTTP client enforcing robots.txt, per-host throttling, retries, and TLS verification."""

    def __init__(
        self,
        user_agent: str,
        *,
        delay_ms: int = 800,
        max_attempts: int = 3,
        timeout_seconds: float = 25.0,
        secrets: Iterable[str] = (),
        transport: httpx.BaseTransport | None = None,
        sleep_fn: Callable[[float], None] = time.sleep,
    ) -> None:
        self._user_agent = user_agent
        self._delay_ms = max(0, delay_ms)
        self._max_attempts = max(1, max_attempts)
        self._secrets = tuple(s for s in secrets if s)
        self._sleep = sleep_fn
        self._last_request_ts: dict[str, float] = {}
        self._robots_cache: dict[str, RobotsPolicy] = {}

        client_kwargs: dict[str, Any] = {
            "timeout": timeout_seconds,
            "headers": {"User-Agent": user_agent, "Accept": "application/json,text/plain"},
            "follow_redirects": False,
        }
        if transport is not None:
            client_kwargs["transport"] = transport
        else:
            client_kwargs["verify"] = certifi.where()
        self._client = httpx.Client(**client_kwargs)

    def __enter__(self) -> "PoliteHttpClient":
        return self

    def __exit__(self, *args: object) -> None:
        self.close()

    def close(self) -> None:
        """Close the underlying connection pool."""
        self._client.close()

    def redact_secrets(self, text: str) -> str:
        """Mask secrets registered on this client."""
        return redact_secrets(text, self._secrets)

    def _throttle_host(self, host: str) -> None:
        floor_ms = max(self._delay_ms, MIN_HOST_DELAY_MS.get(host, 0))
        if floor_ms <= 0:
            return
        last = self._last_request_ts.get(host)
        if last is not None:
            remaining = (floor_ms / 1000.0) - (time.monotonic() - last)
            if remaining > 0:
                self._sleep(remaining)
        self._last_request_ts[host] = time.monotonic()

    def _get_robots_policy(self, base_url: str) -> RobotsPolicy:
        parsed = urlsplit(base_url)
        origin = f"{parsed.scheme}://{parsed.netloc}"
        cached = self._robots_cache.get(origin)
        if cached is not None:
            return cached

        robots_url = urljoin(origin, "/robots.txt")
        self._throttle_host(parsed.netloc)
        try:
            response = self._client.get(robots_url)
        except httpx.HTTPError as exc:
            raise TransientFetchError(
                f"gagal mengambil robots.txt {origin}: {self.redact_secrets(str(exc))}"
            ) from exc

        if response.status_code >= 500 or response.status_code == 429:
            raise TransientFetchError(f"robots.txt {origin} mengembalikan HTTP {response.status_code}")

        policy = parse_robots(response.text, self._user_agent) if response.status_code == 200 else RobotsPolicy(())
        self._robots_cache[origin] = policy
        return policy

    def _perform_raw_get(self, url: str, params: dict[str, Any] | None = None) -> httpx.Response:
        parsed = urlsplit(url)

        @retry(
            reraise=True,
            stop=stop_after_attempt(self._max_attempts),
            retry=retry_if_exception_type((TransientFetchError, httpx.TransportError)),
            wait=lambda state: _retry_delay_seconds(
                state.outcome.exception() if state.outcome else RuntimeError(),
                state.attempt_number,
            ),
            sleep=self._sleep,
        )
        def _send_with_retry() -> httpx.Response:
            self._throttle_host(parsed.netloc)
            response = self._client.get(url, params=params)
            if response.status_code >= 500 or response.status_code == 429:
                raise TransientFetchError(
                    f"HTTP {response.status_code} dari {self.redact_secrets(str(response.request.url))}"
                )
            return response

        return _send_with_retry()

    def get_json(self, url: str, params: dict[str, Any] | None = None) -> Any:
        """Fetch JSON over HTTPS after verifying robots.txt and rate limits."""
        policy = self._get_robots_policy(url)
        request = self._client.build_request("GET", url, params=params)
        target_path = request.url.raw_path.decode("ascii", errors="ignore")
        if not policy.is_allowed(target_path):
            raise RobotsDisallowedError(self.redact_secrets(str(request.url)))

        try:
            response = self._perform_raw_get(url, params=params)
        except TransientFetchError:
            raise
        except httpx.HTTPError as exc:
            raise HttpFetchError(
                f"gagal HTTP ke {self.redact_secrets(url)}: {self.redact_secrets(str(exc))}"
            ) from exc

        if response.is_redirect or response.status_code >= 400:
            raise HttpFetchError(
                f"HTTP {response.status_code} dari {self.redact_secrets(str(response.request.url))}"
            )

        try:
            return response.json()
        except ValueError as exc:
            raise SchemaValidationError(
                f"respons bukan JSON sah dari {self.redact_secrets(url)}"
            ) from exc

    def get_text(self, url: str, params: dict[str, Any] | None = None) -> str:
        """Fetch raw UTF-8 response text over HTTPS after checking robots.txt and rate limits."""
        policy = self._get_robots_policy(url)
        request = self._client.build_request("GET", url, params=params)
        target_path = request.url.raw_path.decode("ascii", errors="ignore")
        if not policy.is_allowed(target_path):
            raise RobotsDisallowedError(self.redact_secrets(str(request.url)))

        try:
            response = self._perform_raw_get(url, params=params)
        except TransientFetchError:
            raise
        except httpx.HTTPError as exc:
            raise HttpFetchError(
                f"gagal HTTP ke {self.redact_secrets(url)}: {self.redact_secrets(str(exc))}"
            ) from exc

        if response.is_redirect or response.status_code >= 400:
            raise HttpFetchError(
                f"HTTP {response.status_code} dari {self.redact_secrets(str(response.request.url))}"
            )
        return response.text


class AsyncPoliteHttpClient:
    """Async httpx + tenacity XHR/JSON client with per-host throttling and secret redaction."""

    def __init__(
        self,
        user_agent: str,
        *,
        delay_ms: int = 800,
        max_attempts: int = 3,
        timeout_seconds: float = 25.0,
        secrets: Iterable[str] = (),
        transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        self._user_agent = user_agent
        self._delay_ms = max(0, delay_ms)
        self._max_attempts = max(1, max_attempts)
        self._secrets = tuple(s for s in secrets if s)
        self._last_request_ts: dict[str, float] = {}
        self._robots_cache: dict[str, RobotsPolicy] = {}

        client_kwargs: dict[str, Any] = {
            "timeout": timeout_seconds,
            "headers": {"User-Agent": user_agent, "Accept": "application/json,text/plain"},
            "follow_redirects": False,
        }
        if transport is not None:
            client_kwargs["transport"] = transport
        else:
            client_kwargs["verify"] = certifi.where()
        self._client = httpx.AsyncClient(**client_kwargs)

    async def __aenter__(self) -> "AsyncPoliteHttpClient":
        return self

    async def __aexit__(self, *args: object) -> None:
        await self.aclose()

    async def aclose(self) -> None:
        await self._client.aclose()

    def redact_secrets(self, text: str) -> str:
        return redact_secrets(text, self._secrets)

    async def _throttle_host(self, host: str) -> None:
        floor_ms = max(self._delay_ms, MIN_HOST_DELAY_MS.get(host, 0))
        if floor_ms <= 0:
            return
        last = self._last_request_ts.get(host)
        if last is not None:
            remaining = (floor_ms / 1000.0) - (time.monotonic() - last)
            if remaining > 0:
                await asyncio.sleep(remaining)
        self._last_request_ts[host] = time.monotonic()

    async def get_json(self, url: str, params: dict[str, Any] | None = None) -> Any:
        """Execute an async throttled, retried XHR/JSON GET returning parsed JSON."""
        parsed = urlsplit(url)
        origin = f"{parsed.scheme}://{parsed.netloc}"
        if origin not in self._robots_cache:
            await self._throttle_host(parsed.netloc)
            try:
                rob = await self._client.get(urljoin(origin, "/robots.txt"))
                policy = parse_robots(rob.text, self._user_agent) if rob.status_code == 200 else RobotsPolicy(())
            except httpx.HTTPError:
                policy = RobotsPolicy(())
            self._robots_cache[origin] = policy

        request = self._client.build_request("GET", url, params=params)
        target_path = request.url.raw_path.decode("ascii", errors="ignore")
        if not self._robots_cache[origin].is_allowed(target_path):
            raise RobotsDisallowedError(self.redact_secrets(str(request.url)))

        attempt = 0
        while True:
            attempt += 1
            await self._throttle_host(parsed.netloc)
            try:
                resp = await self._client.get(url, params=params)
                if resp.status_code >= 500 or resp.status_code == 429:
                    raise TransientFetchError(f"HTTP {resp.status_code} dari {self.redact_secrets(str(resp.request.url))}")
                break
            except (TransientFetchError, httpx.TransportError) as exc:
                if attempt >= self._max_attempts:
                    if isinstance(exc, TransientFetchError):
                        raise
                    raise HttpFetchError(f"gagal HTTP async ke {self.redact_secrets(url)}: {exc}") from exc
                await asyncio.sleep(_retry_delay_seconds(exc, attempt))

        if resp.is_redirect or resp.status_code >= 400:
            raise HttpFetchError(f"HTTP {resp.status_code} dari {self.redact_secrets(str(resp.request.url))}")
        try:
            return resp.json()
        except ValueError as exc:
            raise SchemaValidationError(f"respons bukan JSON sah dari {self.redact_secrets(url)}") from exc
