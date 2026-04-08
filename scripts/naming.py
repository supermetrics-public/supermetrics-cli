"""Name and URL transforms — snake_to_camel, snake_to_kebab, parse_server_url."""

import re
from urllib.parse import urlparse


def snake_to_camel(name: str) -> str:
    """Convert snake_case or kebab-case to CamelCase."""
    return "".join(word.capitalize() for word in re.split(r"[_\-]", name))


def snake_to_kebab(name: str) -> str:
    """Convert snake_case to kebab-case."""
    return name.replace("_", "-")


def parse_server_url(server_url):
    """Parse a server URL into (subdomain, path_prefix).

    E.g., "https://api.supermetrics.com/v2" → ("api", "/v2")
          "https://dts-api.supermetrics.com/v1" → ("dts-api", "/v1")
          "https://api.supermetrics.com" → ("api", "")
    """
    parsed = urlparse(server_url)
    # Extract subdomain: first part before the domain suffix
    # e.g., "api.supermetrics.com" → "api"
    #        "dts-api.supermetrics.com" → "dts-api"
    host_parts = parsed.hostname.split(".")
    subdomain = host_parts[0]  # "api" or "dts-api"
    path_prefix = parsed.path.rstrip("/")  # "/v2" or ""
    return subdomain, path_prefix
