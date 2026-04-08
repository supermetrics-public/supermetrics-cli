"""Generate Go files for resource groups (one file per resource with Cobra commands)."""

import sys

from go_types import go_flag_func
from go_types import go_flag_type
from go_types import go_var_type
from go_types import go_zero_value
from go_types import parse_timeout
from naming import parse_server_url
from naming import snake_to_camel
from spec_parser import extract_params
from spec_parser import find_operation

GO_PACKAGE = "generated"


def _generate_flag_declarations(params, var_prefix, is_paginated):
    """Emit Go flag variable declarations for a command's parameters."""
    lines = []
    for param in params:
        var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
        var_type = go_var_type(param)
        lines.append(f"var {var_name} {var_type}")
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


def _generate_request_body(params, fixed_values, method, var_prefix, is_async):
    """Emit Go code that builds the request body or query parameters."""
    lines = []
    method_upper = method.upper()
    body_params = [p for p in params if p["in"] == "body"]
    json_query_params = [p for p in params if p["in"] == "json_query"]
    query_params = [p for p in params if p["in"] == "query"]

    has_body = len(body_params) > 0 or (len(fixed_values) > 0 and method_upper in ("POST", "PUT", "PATCH"))
    has_json_query = len(json_query_params) > 0

    body_var = "nil"

    if has_body:
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
        lines.append('\t\tif encoded := q.Encode(); encoded != "" {')
        lines.append('\t\t\trequestURL += "?" + encoded')
        lines.append("\t\t}")
        lines.append("")

    return lines, body_var


def _generate_execution(cmd_config, var_prefix, subdomain, path_prefix, timeout_expr, body_var):  # noqa: PLR0913
    """Emit Go code for request execution (sync/async/paginated/wait)."""
    lines = []
    is_async = cmd_config.get("async", False)
    is_paginated = cmd_config.get("paginated", False)
    has_wait = cmd_config.get("wait", False)
    spinner_text = cmd_config.get("spinner_text", "Processing...")

    lines.append(f"\t\ttimeout := resolveTimeout(cmd, {timeout_expr})")
    if is_async and is_paginated:
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
    for param in params:
        if param["in"] == "path":
            continue
        var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
        flag_func = go_flag_func(param)
        flag_name = param["cli_flag"]
        desc = param["description"].replace('"', '\\"')
        zero = go_zero_value(param)
        lines.append(f'\t{cmd_var}.Flags().{flag_func}(&{var_name}, "{flag_name}", {zero}, "{desc}")')

    path_params = [p for p in params if p["in"] == "path"]
    for param in path_params:
        var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
        flag_func = go_flag_func(param)
        flag_name = param["cli_flag"]
        desc = param["description"].replace('"', '\\"')
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
        if param["required"]:
            flag_name = param["cli_flag"]
            lines.append(f'\t_ = {cmd_var}.MarkFlagRequired("{flag_name}")')

    return lines


def generate_resource_file(resource_name, resource_config, spec, servers):
    """Generate a Go file for a resource group."""
    camel_name = snake_to_camel(resource_name)
    server_index = resource_config.get("server_index", 0)
    server_url = servers[server_index]["url"]
    subdomain, path_prefix = parse_server_url(server_url)

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
    lines.append('\t"strings"')
    lines.append('\t"time"')
    lines.append("")
    lines.append('\t"github.com/spf13/cobra"')
    lines.append("")
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
            print(f"WARNING: operationId '{op_id}' not found in spec", file=sys.stderr)
            continue

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

        # Required param validation
        required_string_params = [
            p for p in params
            if p["required"] and go_flag_type(p) == "string"
        ]
        for param in required_string_params:
            var_name = f"{var_prefix}{snake_to_camel(param['name'])}"
            flag_name = param["cli_flag"]
            lines.append(f'\t\tif {var_name} == "" {{')
            lines.append(f'\t\t\treturn exitcode.Wrap(fmt.Errorf("--{flag_name} must not be empty"), exitcode.Usage)')
            lines.append("\t\t}")
        if required_string_params:
            lines.append("")

        # URL building
        lines.extend(_generate_url_building(params, fixed_values, var_prefix, subdomain, path_prefix, path))

        # Request body/query params
        method_upper = method.upper()
        body_lines, body_var = _generate_request_body(params, fixed_values, method, var_prefix, is_async)
        lines.extend(body_lines)

        # Dry-run
        if has_dry_run:
            lines.append('\t\tif dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {')
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

        # Execution — pass method_upper through cmd_config for the helper
        cmd_config_with_method = {**cmd_config, "_method_upper": method_upper}
        lines.extend(_generate_execution(cmd_config_with_method, var_prefix, subdomain, path_prefix, timeout_expr, body_var))

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
