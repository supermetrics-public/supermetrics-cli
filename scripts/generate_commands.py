#!/usr/bin/env python3
"""Generate Cobra CLI commands from OpenAPI spec + command mapping.

Reads:
  - openapi-spec.yaml: parameter definitions, types, descriptions
  - scripts/command-mapping.yaml: maps operations to CLI resource groups

Outputs:
  - cmd/generated/*.go: one file per resource group with Cobra commands
  - cmd/generated/{register,auth,request,polling,prompt}.go: shared helpers
"""

from pathlib import Path

from go_types import go_flag_func
from go_types import go_flag_type
from go_types import go_var_type
from go_types import go_zero_value
from go_types import parse_timeout
from naming import parse_server_url
from naming import snake_to_camel
from naming import snake_to_kebab
from register_generator import generate_register_files
from resource_generator import generate_resource_file
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
    "extract_params",
    "find_operation",
    "generate_register_files",
    "generate_resource_file",
    "go_flag_func",
    "go_flag_type",
    "go_var_type",
    "go_zero_value",
    "load_yaml",
    "parse_server_url",
    "parse_timeout",
    "resolve_ref",
    "snake_to_camel",
    "snake_to_kebab",
]


def main():
    spec = load_yaml(SPEC_PATH)
    mapping = load_yaml(MAPPING_PATH)
    servers = spec.get("servers", [])

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    # Clean old generated files (preserve test files)
    for f in OUTPUT_DIR.glob("*.go"):
        if not f.name.endswith("_test.go"):
            f.unlink()

    resources = mapping.get("resources", {})

    for resource_name, resource_config in resources.items():
        filename = resource_name.replace("-", "_") + ".go"
        content = generate_resource_file(resource_name, resource_config, spec, servers)
        out_path = OUTPUT_DIR / filename
        out_path.write_text(content)
        print(f"Generated {out_path}")

    # Generate helper files (register.go, auth.go, request.go, polling.go, prompt.go)
    helper_files = generate_register_files(resources)
    for filename, content in helper_files.items():
        out_path = OUTPUT_DIR / filename
        out_path.write_text(content)
        print(f"Generated {out_path}")

    print(f"\nGenerated {len(resources)} resource files + {len(helper_files)} helper files")


if __name__ == "__main__":
    main()
