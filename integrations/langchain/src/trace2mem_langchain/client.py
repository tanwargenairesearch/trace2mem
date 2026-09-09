"""Backward-compatible import for the shared capture client."""
from trace2mem.client import MemoryClient, NoRedirect

__all__ = ["MemoryClient", "NoRedirect"]
