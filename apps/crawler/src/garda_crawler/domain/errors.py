"""Domain exception hierarchy with zero third-party transport coupling."""


class CrawlerError(Exception):
    """Base exception for crawler operations."""


class ConfigError(CrawlerError):
    """Raised when required configuration or credentials are missing or malformed."""


class HttpFetchError(CrawlerError):
    """Raised when an upstream HTTP request fails permanently or violates policy."""


class TransientFetchError(HttpFetchError):
    """Raised for transient network or 5xx/429 failures eligible for retries."""


class RobotsDisallowedError(HttpFetchError):
    """Raised when robots.txt explicitly disallows crawling the target URL path."""

    def __init__(self, target_url: str) -> None:
        self.target_url = target_url
        super().__init__(f"robots.txt melarang jalur {target_url}")


class SchemaValidationError(CrawlerError):
    """Raised when an upstream response body violates the expected payload shape."""
