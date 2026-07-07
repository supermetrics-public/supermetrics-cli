#!/usr/bin/env python3
"""Generate Cobra CLI commands from OpenAPI spec + command mapping.

Reads:
  - openapi-spec.yaml: parameter definitions, types, descriptions
  - scripts/command-mapping.yaml: maps operations to CLI resource groups

Outputs:
  - cmd/generated/*.go: one file per resource group with Cobra commands
  - cmd/generated/{register,auth,request,polling,prompt}.go: shared helpers
"""

import sys
from pathlib import Path

from go_types import go_flag_func
from go_types import go_flag_type
from go_types import go_string_escape
from go_types import go_var_type
from go_types import go_zero_value
from go_types import parse_timeout
from naming import parse_server_url
from naming import snake_to_camel
from naming import snake_to_kebab
from register_generator import generate_register_files
from resource_generator import generate_resource_file
from resource_generator import resolve_operation_server
from spec_parser import extract_params
from spec_parser import find_operation
from spec_parser import load_yaml
from spec_parser import resolve_ref

ROOT = Path(__file__).resolve().parent.parent
SPEC_PATH = ROOT / "openapi-spec.yaml"
MAPPING_PATH = ROOT / "scripts" / "command-mapping.yaml"
OUTPUT_DIR = ROOT / "cmd" / "generated"

# Re-export all public symbols for backward compatibility with tests
__all__ = [
    "MAPPING_PATH",
    "OUTPUT_DIR",
    "SPEC_PATH",
    "active_resources",
    "extract_params",
    "find_operation",
    "generate_register_files",
    "generate_resource_file",
    "go_flag_func",
    "go_flag_type",
    "go_string_escape",
    "go_var_type",
    "go_zero_value",
    "load_yaml",
    "parse_server_url",
    "parse_timeout",
    "resolve_operation_server",
    "resolve_ref",
    "snake_to_camel",
    "snake_to_kebab",
]


def active_resources(resources, spec):
    """Return the subset of resource groups that have >=1 command present in the spec.

    Lets command-mapping.yaml stay ahead of the spec — commands for not-yet-present
    operations are dormant — without emitting empty command groups. A group's individual
    commands are additionally skipped by generate_resource_file when their op is missing.
    """
    return {
        name: cfg
        for name, cfg in resources.items()
        if any(find_operation(spec, c["operation_id"])[2] is not None for c in cfg.get("commands", {}).values())
    }


def main():
    spec = load_yaml(SPEC_PATH)
    mapping = load_yaml(MAPPING_PATH)
    servers = spec.get("servers", [])

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    resources = mapping.get("resources", {})

    active = active_resources(resources, spec)
    skipped = [name for name in resources if name not in active]
    if skipped:
        print(
            f"Skipping {len(skipped)} resource group(s) with no operations in the spec: {', '.join(skipped)}",
            file=sys.stderr,
        )

    # Generate all content in memory FIRST. If any resource fails to generate, we abort
    # before touching the filesystem — a crash must never leave cmd/generated/ wiped.
    generated = {}
    for resource_name, resource_config in active.items():
        filename = resource_name.replace("-", "_") + ".go"
        generated[filename] = generate_resource_file(resource_name, resource_config, spec, servers)

    # Generate helper files (register.go, auth.go, request.go, polling.go, prompt.go).
    # Only the active resource groups are registered.
    helper_files = generate_register_files(active)
    generated.update(helper_files)

    # All content produced successfully — now atomically swap: remove old generated
    # files (preserve hand-written test files) and write the new ones.
    for f in OUTPUT_DIR.glob("*.go"):
        if not f.name.endswith("_test.go"):
            f.unlink()

    for filename, content in generated.items():
        out_path = OUTPUT_DIR / filename
        out_path.write_text(content)
        print(f"Generated {out_path}")

    print(f"\nGenerated {len(active)} resource files + {len(helper_files)} helper files")


if __name__ == "__main__":
    main()
