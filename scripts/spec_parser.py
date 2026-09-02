"""OpenAPI spec parsing — load YAML, find operations, resolve refs, extract params."""

from pathlib import Path

import yaml
from naming import snake_to_kebab


def load_yaml(path):
    with Path(path).open() as f:
        return yaml.safe_load(f)


def find_operation(spec, operation_id):
    """Find an operation in the spec by operationId. Returns (path, method, operation)."""
    for path, path_item in spec.get("paths", {}).items():
        for method in ("get", "post", "put", "patch", "delete"):
            op = path_item.get(method, {})
            if op.get("operationId") == operation_id:
                return path, method, op
    return None, None, None


def resolve_ref(spec, ref):
    """Resolve a $ref pointer in the spec."""
    if not ref or not ref.startswith("#/"):
        return {}
    parts = ref.lstrip("#/").split("/")
    node = spec
    for part in parts:
        node = node.get(part, {})
    return node


def extract_params(spec, operation):
    """Extract CLI-relevant parameters from an operation."""
    params = []

    # Path and query parameters
    for param in operation.get("parameters", []):
        if "$ref" in param:
            param = resolve_ref(spec, param["$ref"])  # noqa: PLW2901

        schema = param.get("schema", {})
        if "$ref" in schema:
            schema = resolve_ref(spec, schema["$ref"])

        p = {
            "name": param["name"],
            "cli_flag": snake_to_kebab(param["name"].lower().replace("-", "_").replace(" ", "_")),
            "in": param.get("in", "query"),
            "required": param.get("required", False),
            "description": param.get("description", ""),
            "type": schema.get("type", "string"),
            "format": schema.get("format", ""),
        }
        # Skip auth headers — handled globally
        if p["name"].lower() == "authorization":
            continue
        params.append(p)

    # Request body parameters (for POST/PUT/PATCH)
    body = operation.get("requestBody", {})
    if "$ref" in body:
        body = resolve_ref(spec, body["$ref"])

    body_content = body.get("content", {})

    # JSON body parameters
    json_content = body_content.get("application/json", {})
    body_schema = json_content.get("schema", {})
    if "$ref" in body_schema:
        body_schema = resolve_ref(spec, body_schema["$ref"])

    for prop_name, prop_schema in body_schema.get("properties", {}).items():
        if "$ref" in prop_schema:
            prop_schema = resolve_ref(spec, prop_schema["$ref"])  # noqa: PLW2901

        required_list = body_schema.get("required", [])
        p = {
            "name": prop_name,
            "cli_flag": snake_to_kebab(prop_name),
            "in": "body",
            "required": prop_name in required_list,
            "description": prop_schema.get("description", ""),
            "type": prop_schema.get("type", "string"),
            "format": prop_schema.get("format", ""),
        }
        params.append(p)

    # Multipart form-data parameters (file uploads)
    multipart_content = body_content.get("multipart/form-data", {})
    multipart_schema = multipart_content.get("schema", {})
    if "$ref" in multipart_schema:
        multipart_schema = resolve_ref(spec, multipart_schema["$ref"])

    for prop_name, prop_schema in multipart_schema.get("properties", {}).items():
        if "$ref" in prop_schema:
            prop_schema = resolve_ref(spec, prop_schema["$ref"])  # noqa: PLW2901

        required_list = multipart_schema.get("required", [])
        is_file = prop_schema.get("format") == "binary"
        p = {
            "name": prop_name,
            "cli_flag": "file" if is_file else snake_to_kebab(prop_name),
            "in": "file" if is_file else "form_field",
            "required": prop_name in required_list,
            "description": "" if is_file else prop_schema.get("description", ""),
            "type": prop_schema.get("type", "string"),
            "format": prop_schema.get("format", ""),
        }
        params.append(p)

    # Form-urlencoded body parameters (application/x-www-form-urlencoded)
    form_content = body_content.get("application/x-www-form-urlencoded", {})
    form_schema = form_content.get("schema", {})
    if "$ref" in form_schema:
        form_schema = resolve_ref(spec, form_schema["$ref"])

    for prop_name, prop_schema in form_schema.get("properties", {}).items():
        if "$ref" in prop_schema:
            prop_schema = resolve_ref(spec, prop_schema["$ref"])  # noqa: PLW2901

        required_list = form_schema.get("required", [])
        p = {
            "name": prop_name,
            "cli_flag": snake_to_kebab(prop_name),
            "in": "form_urlencoded",
            "required": prop_name in required_list,
            "description": prop_schema.get("description", ""),
            "type": prop_schema.get("type", "string"),
            "format": prop_schema.get("format", ""),
        }
        params.append(p)

    # For GET endpoints with json query param, extract from the schema
    for param in operation.get("parameters", []):
        if "$ref" in param:
            param = resolve_ref(spec, param["$ref"])  # noqa: PLW2901
        if param.get("name") == "json" and param.get("in") == "query":
            json_schema = param.get("schema", {})
            if "$ref" in json_schema:
                json_schema = resolve_ref(spec, json_schema["$ref"])
            for prop_name, prop_schema in json_schema.get("properties", {}).items():
                if "$ref" in prop_schema:
                    prop_schema = resolve_ref(spec, prop_schema["$ref"])  # noqa: PLW2901
                # Skip api_key — handled via auth
                if prop_name == "api_key":
                    continue
                required_list = json_schema.get("required", [])

                # Handle union types (oneOf with string and array)
                actual_type = prop_schema.get("type", "string")
                if "oneOf" in prop_schema:
                    # Check if it's a string-or-array-of-strings pattern
                    types = [s.get("type") for s in prop_schema["oneOf"]]
                    if "array" in types:
                        actual_type = "array"
                    elif "string" in types:
                        actual_type = "string"

                p = {
                    "name": prop_name,
                    "cli_flag": snake_to_kebab(prop_name),
                    "in": "json_query",
                    "required": prop_name in required_list,
                    "description": prop_schema.get("description", ""),
                    "type": actual_type,
                    "format": prop_schema.get("format", ""),
                }
                params.append(p)
            # Remove the raw "json" param since we've expanded it
            params = [p for p in params if not (p["name"] == "json" and p["in"] == "query")]

    return params
