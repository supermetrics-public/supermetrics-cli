"""Generate Go files for resource groups (one file per resource with Cobra commands)."""

import sys

from go_types import go_flag_func
from go_types import go_flag_type
from go_types import go_string_escape
from go_types import go_var_type
from go_types import go_zero_value
from go_types import parse_timeout
from naming import parse_server_url
from naming import snake_to_camel
from spec_parser import extract_params
from spec_parser import find_operation

GO_PACKAGE = "generated"


def _file_var_name(var_prefix):
    """Return the Go variable name for the --file flag."""
    return f"{var_prefix}File"


def _generate_flag_declarations(params, var_prefix, is_paginated):
    """Emit Go flag variable declarations for a command's parameters."""
    lines = []
    has_file = False
    for param in params:
        if param["in"] == "file":
            has_file = True
            continue
        var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
        var_type = go_var_type(param)
        lines.append(f"var {var_name} {var_type}")
        if param["in"] == "body" and param["type"] == "object":
            lines.append(f"var {var_name}File string")
    if has_file:
        lines.append(f"var {_file_var_name(var_prefix)} string")
    if is_paginated:
        lines.append(f"var {var_prefix}All bool")
        lines.append(f"var {var_prefix}Limit int")
    if params or is_paginated:
        lines.append("")
    return lines


def _generate_url_building(params, fixed_values, var_prefix, subdomain, path_prefix, path):  # noqa: PLR0913
    """Emit Go code that builds the request URL with path parameter substitution."""
    url_expr = f'"https://{subdomain}." + domain + "{path_prefix}{path}"'
    for param in params:
        if param["in"] == "path":
            var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
            if go_flag_type(param) in ("int", "int64"):
                url_expr = f'strings.Replace({url_expr}, "{{{param["name"]}}}", fmt.Sprintf("%d", {var_name}), 1)'
            else:
                url_expr = f'strings.Replace({url_expr}, "{{{param["name"]}}}", url.PathEscape({var_name}), 1)'
    for fixed_name, fixed_val in fixed_values.items():
        url_expr = f'strings.Replace({url_expr}, "{{{fixed_name}}}", "{fixed_val}", 1)'
    return [f"\t\trequestURL := {url_expr}", ""]


def _has_file_params(params):
    """Return True if any parameter is a file upload."""
    return any(p["in"] == "file" for p in params)


def _generate_request_body(params, fixed_values, method, var_prefix, is_async):
    """Emit Go code that builds the request body or query parameters.

    Returns (lines, body_var, content_type_var, multipart_info) where multipart_info
    is a dict like {"field_name": "logo", "file_var": "filePath"} for multipart commands,
    or None for non-multipart commands.
    """
    lines = []
    method_upper = method.upper()
    body_params = [p for p in params if p["in"] == "body"]
    json_query_params = [p for p in params if p["in"] == "json_query"]
    query_params = [p for p in params if p["in"] == "query"]
    file_params = [p for p in params if p["in"] == "file"]
    form_field_params = [p for p in params if p["in"] == "form_field"]
    form_url_params = [p for p in params if p["in"] == "form_urlencoded"]

    # Async commands send their query params through the Supermetrics `?json={...}` envelope
    # so the adaptive polling loop can re-issue the same request. When the spec models the
    # query fields as flat query params (rather than a single `json` object param), route
    # them through the json-envelope path so the async helpers get their jsonParams/baseURL.
    if is_async and not json_query_params and query_params:
        json_query_params = query_params
        query_params = []

    has_body = len(body_params) > 0 or (len(fixed_values) > 0 and method_upper in ("POST", "PUT", "PATCH"))
    has_json_query = len(json_query_params) > 0
    has_multipart = len(file_params) > 0
    has_form_url = len(form_url_params) > 0

    body_var = "nil"
    content_type_var = ""
    multipart_info = None
    is_form = False

    if has_multipart:
        if len(file_params) > 1:
            names = ", ".join(p["name"] for p in file_params)
            print(f"ERROR: multiple file params ({names}) — only one file param per command is supported", file=sys.stderr)
            sys.exit(1)
        file_param = file_params[0]
        file_var = _file_var_name(var_prefix)

        # Resolve file input (--file flag or stdin)
        lines.append(f"\t\tfilePath, cleanup, err := resolveFileInput({file_var})")
        lines.append("\t\tif err != nil {")
        lines.append("\t\t\treturn err")
        lines.append("\t\t}")
        lines.append("\t\tdefer cleanup()")
        lines.append("")

        # Build form fields map
        if form_field_params:
            lines.append("\t\tformFields := map[string]string{}")
            for param in form_field_params:
                var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
                ft = go_flag_type(param)
                if ft in ("int", "int64"):
                    value_expr = f'fmt.Sprintf("%d", {var_name})'
                elif ft == "float64":
                    value_expr = f'fmt.Sprintf("%g", {var_name})'
                elif ft == "bool":
                    value_expr = f'fmt.Sprintf("%t", {var_name})'
                elif ft == "string":
                    value_expr = var_name
                else:
                    value_expr = f'fmt.Sprintf("%v", {var_name})'
                if not param["required"] and ft == "string":
                    lines.append(f'\t\tif {var_name} != "" {{')
                    lines.append(f'\t\t\tformFields["{param["name"]}"] = {value_expr}')
                    lines.append("\t\t}")
                else:
                    lines.append(f'\t\tformFields["{param["name"]}"] = {value_expr}')
        else:
            lines.append("\t\tformFields := map[string]string{}")
        lines.append("")

        body_var = "multipartBody"
        content_type_var = "contentType"
        multipart_info = {"field_name": file_param["name"], "file_var": "filePath"}
    elif has_body:
        object_params = [p for p in body_params if p["type"] == "object"]
        simple_params = [p for p in body_params if p["type"] != "object"]
        if object_params:
            lines.append("\t\tbody := map[string]any{}")
            for param in simple_params:
                var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
                lines.append(f'\t\tbody["{param["name"]}"] = {var_name}')
            # KNOWN LIMITATION: object params are unmarshaled unconditionally, so
            # omitting an OPTIONAL object flag makes the JSON parse fail on empty
            # input and surface a usage error instead of skipping the field.
            # Guarding the unmarshal on a non-empty value (for non-required
            # params) would allow omission. Flagged in PR #62 review; deferred as
            # a separate generator-robustness item since it touches this shared
            # path beyond that PR's scope.
            for param in object_params:
                var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
                flag_name = param["cli_flag"]
                lines.append(f"\t\tvar {var_name}Parsed map[string]any")
                lines.append(f"\t\tif err := json.Unmarshal([]byte({var_name}), &{var_name}Parsed); err != nil {{")
                lines.append(f'\t\t\treturn fmt.Errorf("--{flag_name} must be a JSON object: %w", err)')
                lines.append("\t\t}")
                lines.append(f'\t\tbody["{param["name"]}"] = {var_name}Parsed')
            for fixed_name, fixed_val in fixed_values.items():
                lines.append(f'\t\tbody["{fixed_name}"] = "{fixed_val}"')
        else:
            lines.append("\t\tbody := map[string]any{")
            for param in body_params:
                var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
                lines.append(f'\t\t\t"{param["name"]}": {var_name},')
            for fixed_name, fixed_val in fixed_values.items():
                lines.append(f'\t\t\t"{fixed_name}": "{fixed_val}",')
            lines.append("\t\t}")
        lines.append("\t\tbodyJSON, err := json.Marshal(body)")
        lines.append("\t\tif err != nil {")
        lines.append('\t\t\treturn fmt.Errorf("failed to encode request body: %w", err)')
        lines.append("\t\t}")
        lines.append("")
        body_var = "strings.NewReader(string(bodyJSON))"
    elif has_form_url:
        lines.append("\t\tform := url.Values{}")
        for param in form_url_params:
            var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
            ft = go_flag_type(param)
            if ft in ("int", "int64"):
                value_expr = f'fmt.Sprintf("%d", {var_name})'
            elif ft == "float64":
                value_expr = f'fmt.Sprintf("%g", {var_name})'
            elif ft == "bool":
                value_expr = f'fmt.Sprintf("%t", {var_name})'
            elif ft == "string":
                value_expr = var_name
            else:
                value_expr = f'fmt.Sprintf("%v", {var_name})'
            if not param["required"] and ft == "string":
                lines.append(f'\t\tif {var_name} != "" {{')
                lines.append(f'\t\t\tform.Set("{param["name"]}", {value_expr})')
                lines.append("\t\t}")
            else:
                lines.append(f'\t\tform.Set("{param["name"]}", {value_expr})')
        for fixed_name, fixed_val in fixed_values.items():
            lines.append(f'\t\tform.Set("{fixed_name}", "{fixed_val}")')
        lines.append("")
        body_var = "strings.NewReader(form.Encode())"
        is_form = True
    elif has_json_query:
        lines.append("\t\tjsonParams := map[string]any{")
        for param in json_query_params:
            var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
            ft = go_flag_type(param)
            if not param["required"] and ft in {"string", "stringSlice"}:
                lines.append(f'\t\t\t// {param["name"]} included if non-empty')
            lines.append(f'\t\t\t"{param["name"]}": {var_name},')
        lines.append("\t\t}")
        lines.append("")
        lines.append("\t\tcleanZeroValues(jsonParams)")
        if is_async:
            lines.append('\t\tjsonParams["sync_timeout"] = 0')
        lines.append("")
        if is_async:
            lines.append("\t\tbaseURL := requestURL")
        lines.append("\t\tjsonBytes, err := json.Marshal(jsonParams)")
        lines.append("\t\tif err != nil {")
        lines.append('\t\t\treturn fmt.Errorf("failed to encode query params: %w", err)')
        lines.append("\t\t}")
        lines.append('\t\trequestURL += "?json=" + url.QueryEscape(string(jsonBytes))')
        lines.append("")
    elif query_params:
        lines.append("\t\tq := url.Values{}")
        for param in query_params:
            var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
            ft = go_flag_type(param)
            if ft == "string":
                lines.append(f'\t\tif {var_name} != "" {{')
                lines.append(f'\t\t\tq.Set("{param["name"]}", {var_name})')
                lines.append("\t\t}")
            elif ft in ("int", "int64"):
                lines.append(f"\t\tif {var_name} != 0 {{")
                lines.append(f'\t\t\tq.Set("{param["name"]}", fmt.Sprintf("%d", {var_name}))')
                lines.append("\t\t}")
            elif ft == "bool":
                lines.append(f"\t\tif {var_name} {{")
                lines.append(f'\t\t\tq.Set("{param["name"]}", "true")')
                lines.append("\t\t}")
        lines.append('\t\tif encoded := q.Encode(); encoded != "" {')
        lines.append('\t\t\trequestURL += "?" + encoded')
        lines.append("\t\t}")
        lines.append("")

    return lines, body_var, content_type_var, multipart_info, is_form


def _generate_execution(cmd_config, var_prefix, subdomain, path_prefix, timeout_expr, body_var, content_type_var="", *, is_form=False):  # noqa: PLR0913
    """Emit Go code for request execution (sync/async/paginated/wait/no_content)."""
    lines = []
    is_async = cmd_config.get("async", False)
    is_paginated = cmd_config.get("paginated", False)
    has_wait = cmd_config.get("wait", False)
    is_no_content = cmd_config.get("no_content", False)
    spinner_text = cmd_config.get("spinner_text", "Processing...")
    is_multipart = bool(content_type_var)

    if is_no_content and is_multipart:
        method = cmd_config.get("_method_upper", "PUT")
        lines.append(f"\t\ttimeout := resolveTimeout(cmd, {timeout_expr})")
        lines.append(f'\t\tif err := executeMultipartRequestNoContent(cmd, "{method}", requestURL, {body_var}, {content_type_var}, apiKey, timeout, "{spinner_text}"); err != nil {{')
        lines.append("\t\t\treturn err")
        lines.append("\t\t}")
        lines.append("\t\treturn nil")
        return lines

    if is_no_content:
        method = cmd_config.get("_method_upper", "GET")
        done_message = cmd_config.get("done_message", "Done.")
        lines.append(f"\t\ttimeout := resolveTimeout(cmd, {timeout_expr})")
        lines.append(f'\t\tif err := executeRequestNoContent(cmd, "{method}", requestURL, {body_var}, apiKey, timeout, "{spinner_text}"); err != nil {{')
        lines.append("\t\t\treturn err")
        lines.append("\t\t}")
        lines.append(f'\t\tfmt.Fprintln(cli.InfoWriterErr(cmd), "{done_message}")')
        lines.append("\t\treturn nil")
        return lines

    lines.append(f"\t\ttimeout := resolveTimeout(cmd, {timeout_expr})")
    if is_multipart:
        method = cmd_config.get("_method_upper", "POST")
        lines.append(f'\t\tresult, err := executeMultipartRequest(cmd, "{method}", requestURL, {body_var}, {content_type_var}, apiKey, timeout, "{spinner_text}")')
    elif is_form:
        method = cmd_config.get("_method_upper", "POST")
        lines.append(f'\t\tresult, err := executeFormRequest(cmd, "{method}", requestURL, {body_var}, apiKey, timeout, "{spinner_text}")')
    elif is_async and is_paginated:
        lines.append("\t\tvar result any")
        lines.append(f"\t\tif {var_prefix}All || {var_prefix}Limit > 0 {{")
        lines.append(f'\t\t\tresult, err = executeAsyncQueryPaginated(cmd, baseURL, requestURL, jsonParams, apiKey, timeout, "{spinner_text}", {var_prefix}All, {var_prefix}Limit)')
        lines.append("\t\t} else {")
        lines.append(f'\t\t\tresult, err = executeAsyncQuery(cmd, baseURL, requestURL, jsonParams, apiKey, timeout, "{spinner_text}")')
        lines.append("\t\t}")
    elif is_async:
        lines.append(f'\t\tresult, err := executeAsyncQuery(cmd, baseURL, requestURL, jsonParams, apiKey, timeout, "{spinner_text}")')
    else:
        method = cmd_config.get("_method_upper", "GET")
        lines.append(f'\t\tresult, err := executeRequest(cmd, "{method}", requestURL, {body_var}, apiKey, timeout, "{spinner_text}")')
    lines.append("\t\tif err != nil {")
    lines.append("\t\t\treturn err")
    lines.append("\t\t}")

    if has_wait:
        lines.append("\t\tif err := printResult(cmd, result); err != nil {")
        lines.append("\t\t\treturn err")
        lines.append("\t\t}")
        lines.append("")
        lines.append('\t\twaitFlag, _ := cmd.Flags().GetBool("wait")')
        lines.append("\t\tif waitFlag {")
        lines.append("\t\t\tdata, ok := result.(map[string]any)")
        lines.append("\t\t\tif !ok {")
        lines.append('\t\t\t\treturn fmt.Errorf("unexpected response format")')
        lines.append("\t\t\t}")
        lines.append('\t\t\tbackfillID, _ := data["transfer_backfill_id"].(float64)')
        lines.append("\t\t\tif backfillID == 0 {")
        lines.append('\t\t\t\treturn fmt.Errorf("could not extract backfill ID from response")')
        lines.append("\t\t\t}")
        team_id_var = f"{var_prefix}{snake_to_camel('team_id')}"
        lines.append(f'\t\t\tgetURL := fmt.Sprintf("https://{subdomain}.%s{path_prefix}/teams/%d/backfills/%d", domain, {team_id_var}, int64(backfillID))')
        lines.append("\t\t\twaitResult, waitErr := waitForBackfill(cmd, getURL, apiKey, timeout)")
        lines.append("\t\t\tif waitResult != nil {")
        lines.append("\t\t\t\t_ = printResult(cmd, waitResult)")
        lines.append("\t\t\t}")
        lines.append("\t\t\treturn waitErr")
        lines.append("\t\t}")
        lines.append("\t\treturn nil")
    else:
        lines.append("\t\treturn printResult(cmd, result)")

    return lines


def _generate_init_flags(params, cmd_config, cmd_var, var_prefix):
    """Emit Go code for flag registration in init()."""
    lines = []
    secure_params = set(cmd_config.get("secure_input", []))
    has_file = False
    for param in params:
        if param["in"] == "path":
            continue
        if param["in"] == "file":
            has_file = True
            continue
        var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
        flag_func = go_flag_func(param)
        flag_name = param["cli_flag"]
        desc = go_string_escape(param["description"])
        if param["name"] in secure_params:
            desc += " (leave empty for secure prompt)"
        zero = go_zero_value(param)
        lines.append(f'\t{cmd_var}.Flags().{flag_func}(&{var_name}, "{flag_name}", {zero}, "{desc}")')
        if param["in"] == "body" and param["type"] == "object":
            lines.append(f'\t{cmd_var}.Flags().StringVar(&{var_name}File, "{flag_name}-file", "", "Path to JSON file for --{flag_name}")')

    if has_file:
        file_var = _file_var_name(var_prefix)
        lines.append(f'\t{cmd_var}.Flags().StringVar(&{file_var}, "file", "", "Path to file (reads from stdin if not provided)")')

    path_params = [p for p in params if p["in"] == "path"]
    for param in path_params:
        var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
        flag_func = go_flag_func(param)
        flag_name = param["cli_flag"]
        desc = go_string_escape(param["description"])
        if param["name"] in secure_params:
            desc += " (leave empty for secure prompt)"
        zero = go_zero_value(param)
        lines.append(f'\t{cmd_var}.Flags().{flag_func}(&{var_name}, "{flag_name}", {zero}, "{desc}")')

    if cmd_config.get("confirm", ""):
        lines.append(f'\t{cmd_var}.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")')
    if cmd_config.get("wait", False):
        lines.append(f'\t{cmd_var}.Flags().Bool("wait", false, "Wait for completion and show progress")')
    if cmd_config.get("dry_run", False):
        lines.append(f'\t{cmd_var}.Flags().Bool("dry-run", false, "Print request details without executing")')
    if cmd_config.get("paginated", False):
        lines.append(f'\t{cmd_var}.Flags().BoolVar(&{var_prefix}All, "all", false, "Fetch all pages of results")')
        lines.append(f'\t{cmd_var}.Flags().IntVar(&{var_prefix}Limit, "limit", 0, "Maximum number of data rows to return")')

    for param in params:
        has_file_companion = param["in"] == "body" and param["type"] == "object"
        if param["required"] and param["name"] not in secure_params and param["in"] != "file" and not has_file_companion:
            flag_name = param["cli_flag"]
            lines.append(f'\t_ = {cmd_var}.MarkFlagRequired("{flag_name}")')

    return lines


def resolve_operation_server(spec, path, operation, servers, resource_server_index):
    """Resolve (subdomain, path_prefix) for a single operation.

    The combined spec attaches a `servers` override to each path (and may attach one
    per operation), so the base URL is resolved per operation rather than from a single
    global index. Resolution priority (most specific first):

    1. operation-level `servers[0].url`
    2. path-level `servers[0].url`
    3. the resource's `server_index` into the spec's global `servers` (back-compat with
       older specs that only declared global servers)
    4. the first global server, else a hardcoded default
    """
    op_servers = (operation or {}).get("servers")
    path_servers = spec.get("paths", {}).get(path, {}).get("servers")

    if op_servers:
        server_url = op_servers[0]["url"]
    elif path_servers:
        server_url = path_servers[0]["url"]
    elif servers and 0 <= resource_server_index < len(servers):
        server_url = servers[resource_server_index]["url"]
    elif servers:
        server_url = servers[0]["url"]
    else:
        server_url = "https://api.supermetrics.com"

    return parse_server_url(server_url)


def generate_resource_file(resource_name, resource_config, spec, servers):
    """Generate a Go file for a resource group."""
    camel_name = snake_to_camel(resource_name)
    resource_server_index = resource_config.get("server_index", 0)

    lines = []
    lines.append("// Code generated by generate_commands.py. DO NOT EDIT.")
    lines.append(f"package {GO_PACKAGE}")
    lines.append("")
    # Imports are managed by goimports — emit all potentially needed ones.
    # goimports will remove unused ones automatically.
    lines.append("import (")
    lines.append('\t"encoding/json"')
    lines.append('\t"fmt"')
    lines.append('\t"net/url"')
    lines.append('\t"os"')
    lines.append('\t"strings"')
    lines.append('\t"time"')
    lines.append("")
    lines.append('\t"github.com/spf13/cobra"')
    lines.append('\t"golang.org/x/term"')
    lines.append("")
    lines.append('\t"github.com/supermetrics-public/supermetrics-cli/internal/cli"')
    lines.append('\t"github.com/supermetrics-public/supermetrics-cli/internal/exitcode"')
    lines.append('\t"github.com/supermetrics-public/supermetrics-cli/internal/httpclient"')
    lines.append(")")
    lines.append("")

    # Resource group command
    lines.append(f"var {camel_name}Cmd = &cobra.Command{{")
    lines.append(f'\tUse:   "{resource_name}",')
    lines.append(f'\tShort: "{resource_config["description"]}",')
    lines.append("}")
    lines.append("")

    # Flag variables and subcommands
    for cmd_name, cmd_config in resource_config.get("commands", {}).items():
        op_id = cmd_config["operation_id"]
        path, method, operation = find_operation(spec, op_id)
        if not operation:
            print(f"WARNING: operationId '{op_id}' not found in spec — skipping command '{resource_name} {cmd_name}'", file=sys.stderr)
            continue

        # Resolve the base URL for this specific operation (per-path server override).
        subdomain, path_prefix = resolve_operation_server(spec, path, operation, servers, resource_server_index)

        description = cmd_config.get("description", operation.get("summary", ""))
        timeout_str = cmd_config.get("timeout", "")
        timeout_expr = parse_timeout(timeout_str)
        confirm_msg = cmd_config.get("confirm", "")
        has_dry_run = cmd_config.get("dry_run", False)
        is_async = cmd_config.get("async", False)
        is_paginated = cmd_config.get("paginated", False)
        exclude_params = set(cmd_config.get("exclude_params", []))
        params = extract_params(spec, operation)
        fixed_values = cmd_config.get("fixed_values", {})
        params = [p for p in params if p["name"] not in fixed_values and p["name"] not in exclude_params]

        cmd_camel = snake_to_camel(cmd_name)
        var_prefix = f"flag{camel_name}{cmd_camel}"

        # Flag variable declarations
        lines.extend(_generate_flag_declarations(params, var_prefix, is_paginated))

        # Command definition
        lines.append(f"var {camel_name}{cmd_camel}Cmd = &cobra.Command{{")
        lines.append(f'\tUse:   "{cmd_name}",')
        lines.append(f'\tShort: "{description}",')
        lines.append("\tRunE: func(cmd *cobra.Command, args []string) error {")

        # Auth resolution
        lines.append("\t\tdomain, apiKey, err := resolveAuth(cmd)")
        lines.append("\t\tif err != nil {")
        lines.append("\t\t\treturn err")
        lines.append("\t\t}")
        lines.append("")

        # File input for object body params (--param-file reads JSON from file)
        object_body_params = [p for p in params if p["in"] == "body" and p["type"] == "object"]
        for param in object_body_params:
            var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
            flag_name = param["cli_flag"]
            lines.append(f'\t\tif {var_name} == "" && {var_name}File != "" {{')
            lines.append(f"\t\t\tfileData, err := os.ReadFile({var_name}File)")
            lines.append("\t\t\tif err != nil {")
            lines.append(f'\t\t\t\treturn fmt.Errorf("failed to read --{flag_name}-file: %w", err)')
            lines.append("\t\t\t}")
            lines.append(f"\t\t\t{var_name} = string(fileData)")
            lines.append("\t\t}")
        if object_body_params:
            lines.append("")

        # Required param validation
        secure_params = set(cmd_config.get("secure_input", []))

        # Guard: secure_input and file upload cannot coexist (both read stdin)
        if secure_params and _has_file_params(params):
            print(
                f"ERROR: command '{cmd_name}' combines secure_input and file upload — "
                f"both read stdin, which causes conflicts",
                file=sys.stderr,
            )
            sys.exit(1)

        if is_async and _has_file_params(params):
            print(f"WARNING: command '{cmd_name}' combines async and file upload — async behavior will be ignored for multipart", file=sys.stderr)

        if cmd_config.get("wait", False) and _has_file_params(params):
            print(f"WARNING: command '{cmd_name}' combines wait and file upload — wait behavior will be ignored for multipart", file=sys.stderr)
        required_string_params = [
            p for p in params
            if p["required"] and go_flag_type(p) == "string" and p["name"] not in secure_params and p["in"] != "file"
        ]
        for param in required_string_params:
            var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
            flag_name = param["cli_flag"]
            lines.append(f'\t\tif {var_name} == "" {{')
            lines.append(f'\t\t\treturn exitcode.Wrap(fmt.Errorf("--{flag_name} must not be empty"), exitcode.Usage)')
            lines.append("\t\t}")
        if required_string_params:
            lines.append("")

        # Secure input prompting (before URL building, after param validation)
        for param in params:
            if param["name"] in secure_params:
                var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
                flag_name = param["cli_flag"]
                lines.append(f'\t\tif {var_name} == "" {{')
                lines.append(f'\t\t\tval, err := readSecureInput(cmd, "{flag_name}")')
                lines.append("\t\t\tif err != nil {")
                lines.append("\t\t\t\treturn err")
                lines.append("\t\t\t}")
                lines.append('\t\t\tif val == "" {')
                lines.append(f'\t\t\t\treturn exitcode.Wrap(fmt.Errorf("--{flag_name} is required (provide via flag or stdin)"), exitcode.Usage)')
                lines.append("\t\t\t}")
                lines.append(f"\t\t\t{var_name} = val")
                lines.append("\t\t}")
                lines.append("")

        # URL building
        lines.extend(_generate_url_building(params, fixed_values, var_prefix, subdomain, path_prefix, path))

        # Request body/query params
        method_upper = method.upper()
        is_multipart = _has_file_params(params)
        body_lines, body_var, content_type_var, multipart_info, is_form = _generate_request_body(params, fixed_values, method, var_prefix, is_async)
        lines.extend(body_lines)

        # Dry-run
        if has_dry_run:
            lines.append('\t\tif dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {')
            if is_multipart:
                lines.append(f'\t\t\tfmt.Fprintf(cmd.ErrOrStderr(), "%s %s\\nFile: %s\\n", "{method_upper}", requestURL, filePath)')
            else:
                lines.append(f'\t\t\tdryRunRequest(cmd, "{method_upper}", requestURL, {body_var})')
            lines.append("\t\t\treturn nil")
            lines.append("\t\t}")
            lines.append("")

        # Confirmation
        if confirm_msg:
            msg_expr = f'"{confirm_msg}"'
            for param in params:
                placeholder = "{" + param["name"] + "}"
                if placeholder in confirm_msg:
                    var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
                    ft = go_flag_type(param)
                    if ft in ("int", "int64"):
                        msg_expr = f'strings.Replace({msg_expr}, "{placeholder}", fmt.Sprintf("%d", {var_name}), 1)'
                    else:
                        msg_expr = f'strings.Replace({msg_expr}, "{placeholder}", {var_name}, 1)'
            lines.append(f"\t\tif err := confirmAction(cmd, {msg_expr}); err != nil {{")
            lines.append("\t\t\treturn err")
            lines.append("\t\t}")
            lines.append("")

        if multipart_info:
            field_name = multipart_info["field_name"]
            lines.append(f'\t\tmultipartBody, contentType, err := buildMultipartStream(filePath, "{field_name}", formFields)')
            lines.append("\t\tif err != nil {")
            lines.append('\t\t\treturn fmt.Errorf("failed to build multipart stream: %w", err)')
            lines.append("\t\t}")
            lines.append("")

        # Execution — pass method_upper through cmd_config for the helper
        cmd_config_with_method = {**cmd_config, "_method_upper": method_upper}
        lines.extend(_generate_execution(cmd_config_with_method, var_prefix, subdomain, path_prefix, timeout_expr, body_var, content_type_var, is_form=is_form))

        lines.append("\t},")
        lines.append("}")
        lines.append("")

    # init function
    lines.append("func init() {")
    for cmd_name, cmd_config in resource_config.get("commands", {}).items():
        op_id = cmd_config["operation_id"]
        path, method, operation = find_operation(spec, op_id)
        if not operation:
            continue
        params = extract_params(spec, operation)
        fixed_values = cmd_config.get("fixed_values", {})
        exclude_params = set(cmd_config.get("exclude_params", []))
        params = [p for p in params if p["name"] not in fixed_values and p["name"] not in exclude_params]
        cmd_camel = snake_to_camel(cmd_name)
        var_prefix = f"flag{camel_name}{cmd_camel}"
        cmd_var = f"{camel_name}{cmd_camel}Cmd"

        lines.extend(_generate_init_flags(params, cmd_config, cmd_var, var_prefix))
        lines.append(f"\t{camel_name}Cmd.AddCommand({cmd_var})")
        lines.append("")
    lines.append("}")

    return "\n".join(lines) + "\n"
