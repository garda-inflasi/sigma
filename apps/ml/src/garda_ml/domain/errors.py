"""Domain exception hierarchy cleanly separated from infrastructure implementations."""


class InsufficientHistoryError(ValueError):
    """Raised when a series or target lacks enough historical points."""


class CycleLockError(RuntimeError):
    """Raised when a distributed cycle lease is lost or held by another worker."""


class ArtifactLoadError(RuntimeError):
    """Raised when serialized model artifacts are missing or corrupted."""


class InvalidPayloadError(ValueError):
    """Raised on malformed JSON requests to the stateless HTTP inference service."""
