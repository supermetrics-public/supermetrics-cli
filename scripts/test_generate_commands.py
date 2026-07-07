#!/usr/bin/env python3
"""Tests for generate_commands.py."""

import unittest

from generate_commands import MAPPING_PATH
from generate_commands import SPEC_PATH
from generate_commands import active_resources
from generate_commands import extract_params
from generate_commands import find_operation
from generate_commands import generate_register_files
from generate_commands import generate_resource_file
from generate_commands import go_flag_func
from generate_commands import go_flag_type
from generate_commands import go_string_escape
from generate_commands import go_var_type
from generate_commands import go_zero_value
from generate_commands import load_yaml
from generate_commands import parse_server_url
from generate_commands import parse_timeout
from generate_commands import resolve_operation_server
from generate_commands import resolve_ref
from generate_commands import snake_to_camel
from generate_commands import snake_to_kebab


class TestSnakeToCamel(unittest.TestCase):
    def test_simple(self):
        self.assertEqual(snake_to_camel("login_links"), "LoginLinks")

    def test_kebab(self):
        self.assertEqual(snake_to_camel("get-latest"), "GetLatest")

    def test_single_word(self):
        self.assertEqual(snake_to_camel("accounts"), "Accounts")

    def test_mixed(self):
        self.assertEqual(snake_to_camel("ds_id"), "DsId")

    def test_multiple_segments(self):
        self.assertEqual(snake_to_camel("list-incomplete"), "ListIncomplete")


class TestSnakeToKebab(unittest.TestCase):
    def test_simple(self):
        self.assertEqual(snake_to_kebab("ds_id"), "ds-id")

    def test_no_change(self):
        self.assertEqual(snake_to_kebab("name"), "name")

    def test_multiple(self):
        self.assertEqual(snake_to_kebab("cache_minutes"), "cache-minutes")


class TestParseTimeout(unittest.TestCase):
    def test_minutes(self):
        self.assertEqual(parse_timeout("60m"), "60 * time.Minute")

    def test_seconds(self):
        self.assertEqual(parse_timeout("30s"), "30 * time.Second")

    def test_empty(self):
        self.assertEqual(parse_timeout(""), "httpclient.DefaultTimeout")

    def test_none(self):
        self.assertEqual(parse_timeout(None), "httpclient.DefaultTimeout")

    def test_unknown_suffix(self):
        self.assertEqual(parse_timeout("10h"), "httpclient.DefaultTimeout")


class TestParseServerUrl(unittest.TestCase):
    def test_api_with_path(self):
        sub, path = parse_server_url("https://api.supermetrics.com/v2")
        self.assertEqual(sub, "api")
        self.assertEqual(path, "/v2")

    def test_dts_api(self):
        sub, path = parse_server_url("https://dts-api.supermetrics.com/v1")
        self.assertEqual(sub, "dts-api")
        self.assertEqual(path, "/v1")

    def test_no_path(self):
        sub, path = parse_server_url("https://api.supermetrics.com")
        self.assertEqual(sub, "api")
        self.assertEqual(path, "")

    def test_trailing_slash(self):
        sub, path = parse_server_url("https://api.supermetrics.com/v2/")
        self.assertEqual(sub, "api")
        self.assertEqual(path, "/v2")


class TestGoFlagType(unittest.TestCase):
    def test_string(self):
        self.assertEqual(go_flag_type({"type": "string"}), "string")

    def test_integer(self):
        self.assertEqual(go_flag_type({"type": "integer"}), "int")

    def test_integer_int64(self):
        self.assertEqual(go_flag_type({"type": "integer", "format": "int64"}), "int64")

    def test_number(self):
        self.assertEqual(go_flag_type({"type": "number"}), "float64")

    def test_boolean(self):
        self.assertEqual(go_flag_type({"type": "boolean"}), "bool")

    def test_array(self):
        self.assertEqual(go_flag_type({"type": "array"}), "stringSlice")

    def test_unknown(self):
        self.assertEqual(go_flag_type({"type": "object"}), "string")


class TestGoFlagFunc(unittest.TestCase):
    def test_string(self):
        self.assertEqual(go_flag_func({"type": "string", "required": False}), "StringVar")

    def test_int(self):
        self.assertEqual(go_flag_func({"type": "integer", "required": True}), "IntVar")

    def test_int64(self):
        self.assertEqual(go_flag_func({"type": "integer", "format": "int64", "required": False}), "Int64Var")

    def test_bool(self):
        self.assertEqual(go_flag_func({"type": "boolean", "required": False}), "BoolVar")

    def test_slice(self):
        self.assertEqual(go_flag_func({"type": "array", "required": False}), "StringSliceVar")


class TestGoZeroValue(unittest.TestCase):
    def test_string(self):
        self.assertEqual(go_zero_value({"type": "string"}), '""')

    def test_int(self):
        self.assertEqual(go_zero_value({"type": "integer"}), "0")

    def test_bool(self):
        self.assertEqual(go_zero_value({"type": "boolean"}), "false")

    def test_slice(self):
        self.assertEqual(go_zero_value({"type": "array"}), "nil")


class TestGoVarType(unittest.TestCase):
    def test_string(self):
        self.assertEqual(go_var_type({"type": "string"}), "string")

    def test_int64(self):
        self.assertEqual(go_var_type({"type": "integer", "format": "int64"}), "int64")

    def test_slice(self):
        self.assertEqual(go_var_type({"type": "array"}), "[]string")


class TestFindOperation(unittest.TestCase):
    def setUp(self):
        self.spec = {
            "paths": {
                "/accounts": {
                    "get": {"operationId": "getAccounts", "summary": "List accounts"},
                },
                "/login/link": {
                    "post": {"operationId": "createLoginLink", "summary": "Create link"},
                },
            }
        }

    def test_found(self):
        path, method, op = find_operation(self.spec, "getAccounts")
        self.assertEqual(path, "/accounts")
        self.assertEqual(method, "get")
        self.assertEqual(op["summary"], "List accounts")

    def test_post(self):
        _path, method, _op = find_operation(self.spec, "createLoginLink")
        self.assertEqual(method, "post")

    def test_not_found(self):
        path, method, op = find_operation(self.spec, "nonexistent")
        self.assertIsNone(path)
        self.assertIsNone(method)
        self.assertIsNone(op)


class TestResolveRef(unittest.TestCase):
    def test_simple(self):
        spec = {
            "components": {
                "schemas": {
                    "Account": {"type": "object", "properties": {"id": {"type": "string"}}}
                }
            }
        }
        result = resolve_ref(spec, "#/components/schemas/Account")
        self.assertEqual(result["type"], "object")

    def test_empty_ref(self):
        self.assertEqual(resolve_ref({}, ""), {})

    def test_none_ref(self):
        self.assertEqual(resolve_ref({}, None), {})


class TestExtractParams(unittest.TestCase):
    def test_query_param(self):
        spec = {}
        operation = {
            "parameters": [
                {
                    "name": "ds_id",
                    "in": "query",
                    "required": True,
                    "description": "Data source ID",
                    "schema": {"type": "string"},
                }
            ]
        }
        params = extract_params(spec, operation)
        self.assertEqual(len(params), 1)
        self.assertEqual(params[0]["name"], "ds_id")
        self.assertEqual(params[0]["cli_flag"], "ds-id")
        self.assertTrue(params[0]["required"])

    def test_skips_authorization(self):
        spec = {}
        operation = {
            "parameters": [
                {"name": "Authorization", "in": "header", "schema": {"type": "string"}},
                {"name": "ds_id", "in": "query", "schema": {"type": "string"}},
            ]
        }
        params = extract_params(spec, operation)
        names = [p["name"] for p in params]
        self.assertNotIn("Authorization", names)
        self.assertIn("ds_id", names)

    def test_body_params(self):
        spec = {}
        operation = {
            "requestBody": {
                "content": {
                    "application/json": {
                        "schema": {
                            "type": "object",
                            "properties": {
                                "range_start": {"type": "string", "description": "Start date"},
                                "range_end": {"type": "string", "description": "End date"},
                            },
                            "required": ["range_start", "range_end"],
                        }
                    }
                }
            }
        }
        params = extract_params(spec, operation)
        self.assertEqual(len(params), 2)
        names = {p["name"] for p in params}
        self.assertEqual(names, {"range_start", "range_end"})
        for p in params:
            self.assertTrue(p["required"])
            self.assertEqual(p["in"], "body")


class TestExtractParamsMultipart(unittest.TestCase):
    def test_multipart_file_param(self):
        spec = {}
        operation = {
            "requestBody": {
                "content": {
                    "multipart/form-data": {
                        "schema": {
                            "type": "object",
                            "properties": {
                                "logo": {"type": "string", "format": "binary", "description": "Logo file"}
                            },
                            "required": ["logo"],
                        }
                    }
                }
            }
        }
        params = extract_params(spec, operation)
        self.assertEqual(len(params), 1)
        self.assertEqual(params[0]["name"], "logo")
        self.assertEqual(params[0]["in"], "file")
        self.assertEqual(params[0]["cli_flag"], "file")
        self.assertEqual(params[0]["description"], "")
        self.assertTrue(params[0]["required"])
        self.assertEqual(params[0]["format"], "binary")

    def test_multipart_form_field_param(self):
        spec = {}
        operation = {
            "requestBody": {
                "content": {
                    "multipart/form-data": {
                        "schema": {
                            "type": "object",
                            "properties": {
                                "description": {"type": "string", "description": "File description"}
                            },
                        }
                    }
                }
            }
        }
        params = extract_params(spec, operation)
        self.assertEqual(len(params), 1)
        self.assertEqual(params[0]["name"], "description")
        self.assertEqual(params[0]["in"], "form_field")
        self.assertFalse(params[0]["required"])

    def test_multipart_mixed_params(self):
        spec = {}
        operation = {
            "parameters": [
                {"name": "team_id", "in": "path", "required": True, "schema": {"type": "integer"}},
            ],
            "requestBody": {
                "content": {
                    "multipart/form-data": {
                        "schema": {
                            "type": "object",
                            "properties": {
                                "file": {"type": "string", "format": "binary", "description": "Upload file"},
                                "title": {"type": "string", "description": "File title"},
                            },
                            "required": ["file"],
                        }
                    }
                }
            },
        }
        params = extract_params(spec, operation)
        self.assertEqual(len(params), 3)
        by_name = {p["name"]: p for p in params}
        self.assertEqual(by_name["team_id"]["in"], "path")
        self.assertEqual(by_name["file"]["in"], "file")
        self.assertEqual(by_name["title"]["in"], "form_field")


class TestGenerateRegisterFiles(unittest.TestCase):
    def _all_content(self, resources):
        """Join all generated file contents for assertion convenience."""
        return "\n".join(generate_register_files(resources).values())

    def test_contains_all_resources(self):
        resources = {"login-links": {}, "accounts": {}, "backfills": {}}
        files = generate_register_files(resources)
        content = files["register.go"]

        self.assertIn("LoginLinksCmd", content)
        self.assertIn("AccountsCmd", content)
        self.assertIn("BackfillsCmd", content)

    def test_contains_helpers(self):
        content = self._all_content({"accounts": {}})

        self.assertIn("isTerminal", content)
        self.assertIn("shouldUseColor", content)
        self.assertIn("NO_COLOR", content)

    def test_is_generated_code(self):
        files = generate_register_files({})
        for content in files.values():
            self.assertTrue(content.startswith("// Code generated"))

    def test_split_into_expected_files(self):
        files = generate_register_files({"queries": {}})
        self.assertEqual(set(files.keys()), {"register.go", "auth.go", "request.go", "polling.go", "prompt.go"})

    def test_contains_pagination_helpers(self):
        files = generate_register_files({"queries": {}})
        content = files["polling.go"]

        self.assertIn("executeAsyncQueryWithMeta", content)
        self.assertIn("executeAsyncQueryPaginated", content)
        self.assertIn("executeAsyncQuery", content)

    def test_pagination_helper_returns_meta(self):
        files = generate_register_files({"queries": {}})
        content = files["polling.go"]

        # executeAsyncQueryWithMeta returns (any, map[string]any, error)
        self.assertIn(
            "func executeAsyncQueryWithMeta(cmd *cobra.Command, baseURL, initialURL string, "
            "queryParams map[string]any, apiKey string, timeout time.Duration, spinnerText string) "
            "(any, map[string]any, error)",
            content,
        )

    def test_pagination_helper_follows_next(self):
        files = generate_register_files({"queries": {}})
        content = files["polling.go"]

        # executeAsyncQueryPaginated should extract paginate.next
        self.assertIn('paginate["next"]', content)
        self.assertIn("fetchAll", content)

    def test_resolve_timeout_helper_emitted(self):
        files = generate_register_files({"queries": {}})
        content = files["request.go"]

        self.assertIn("func resolveTimeout(cmd *cobra.Command, defaultTimeout time.Duration) time.Duration", content)
        self.assertIn("time.ParseDuration", content)

    def test_generated_commands_use_resolve_timeout(self):
        spec = load_yaml(SPEC_PATH)
        mapping = load_yaml(MAPPING_PATH)
        servers = spec.get("servers", [])
        # Pick any resource that has commands
        for name, cfg in mapping.get("resources", {}).items():
            content = generate_resource_file(name, cfg, spec, servers)
            if "resolveTimeout" in content:
                break
        self.assertIn("resolveTimeout(cmd,", content)

    def test_contains_multipart_helpers(self):
        files = generate_register_files({"queries": {}})
        content = files["request.go"]
        self.assertIn("buildMultipartStream", content)
        self.assertIn("executeMultipartRequest", content)
        self.assertIn("resolveFileInput", content)

    def test_execute_request_signatures(self):
        files = generate_register_files({"queries": {}})
        content = files["request.go"]
        # doRequest has contentType (no getBody)
        self.assertIn("func doRequest(cmd *cobra.Command, method, url string, body io.Reader, contentType, apiKey string", content)
        # executeRequest and executeRequestNoContent keep original signatures (no contentType)
        self.assertIn("func executeRequest(cmd *cobra.Command, method, url string, body io.Reader, apiKey string", content)
        self.assertIn("func executeRequestNoContent(cmd *cobra.Command, method, url string, body io.Reader, apiKey string", content)
        # executeMultipartRequest has contentType (no getBody)
        self.assertIn("func executeMultipartRequest(cmd *cobra.Command, method, url string, body io.Reader, contentType, apiKey string", content)
        # executeMultipartRequestNoContent has contentType (no getBody)
        self.assertIn("func executeMultipartRequestNoContent(cmd *cobra.Command, method, url string, body io.Reader, contentType, apiKey string", content)


class TestMultipartResourceGeneration(unittest.TestCase):
    """Test that generate_resource_file produces correct ordering for multipart commands."""

    def _generate_multipart_resource(self, *, dry_run=False, confirm=""):
        spec = {
            "servers": [{"url": "https://api.supermetrics.com/v2"}],
            "paths": {
                "/teams/{team_id}/logos": {
                    "post": {
                        "operationId": "uploadLogo",
                        "parameters": [
                            {"name": "team_id", "in": "path", "required": True, "schema": {"type": "integer"}},
                        ],
                        "requestBody": {
                            "content": {
                                "multipart/form-data": {
                                    "schema": {
                                        "type": "object",
                                        "properties": {
                                            "logo": {"type": "string", "format": "binary"},
                                            "title": {"type": "string", "description": "Logo title"},
                                        },
                                        "required": ["logo"],
                                    }
                                }
                            }
                        },
                    }
                }
            },
        }
        config = {
            "description": "Logo management",
            "commands": {
                "upload": {
                    "operation_id": "uploadLogo",
                    "description": "Upload a logo",
                    "dry_run": dry_run,
                    "confirm": confirm,
                },
            },
        }
        servers = spec["servers"]
        return generate_resource_file("logos", config, spec, servers)

    def test_multipart_stream_after_dry_run(self):
        content = self._generate_multipart_resource(dry_run=True)
        dry_run_pos = content.index("dry-run")
        stream_pos = content.index("buildMultipartStream")
        self.assertGreater(stream_pos, dry_run_pos, "buildMultipartStream must appear after dry-run check")

    def test_multipart_stream_after_confirm(self):
        content = self._generate_multipart_resource(confirm="Upload logo for team {team_id}?")
        confirm_pos = content.index("confirmAction")
        stream_pos = content.index("buildMultipartStream")
        self.assertGreater(stream_pos, confirm_pos, "buildMultipartStream must appear after confirmAction")

    def test_resolve_file_input_before_dry_run(self):
        content = self._generate_multipart_resource(dry_run=True)
        resolve_pos = content.index("resolveFileInput")
        dry_run_pos = content.index("dry-run")
        self.assertLess(resolve_pos, dry_run_pos, "resolveFileInput must appear before dry-run check")

    def test_multipart_uses_execute_multipart_request(self):
        content = self._generate_multipart_resource()
        self.assertIn("executeMultipartRequest", content)
        self.assertNotIn("executeRequest(cmd", content)

    def test_multipart_has_file_flag(self):
        content = self._generate_multipart_resource()
        self.assertIn('"file"', content)
        self.assertIn("resolveFileInput", content)

    def test_multipart_form_field_conditional(self):
        content = self._generate_multipart_resource()
        self.assertIn('formFields["title"]', content)


class TestObjectBodyParams(unittest.TestCase):
    """Test that the generator correctly handles type: object body parameters."""

    def _make_spec_and_config(self):
        spec = {
            "servers": [{"url": "https://api.supermetrics.com/v2"}],
            "paths": {
                "/widgets/{widget_id}": {
                    "put": {
                        "operationId": "updateWidget",
                        "parameters": [
                            {"name": "widget_id", "in": "path", "required": True, "schema": {"type": "string"}},
                        ],
                        "requestBody": {
                            "content": {
                                "application/json": {
                                    "schema": {
                                        "type": "object",
                                        "properties": {
                                            "name": {"type": "string", "description": "Widget name"},
                                            "connector": {"type": "object", "description": "Connector settings"},
                                        },
                                        "required": ["name", "connector"],
                                    }
                                }
                            }
                        },
                    }
                }
            },
        }
        config = {
            "description": "Widget management",
            "commands": {
                "update": {
                    "operation_id": "updateWidget",
                    "description": "Update a widget",
                },
            },
        }
        return spec, config

    def test_object_body_param_generates_json_unmarshal(self):
        spec, config = self._make_spec_and_config()
        content = generate_resource_file("widgets", config, spec, spec["servers"])

        # Object param must use json.Unmarshal, not direct assignment
        self.assertIn("json.Unmarshal", content)
        self.assertNotIn('"connector": flagWidgetsUpdateConnector,', content)

        # Regular string param uses separate assignment (not map literal) when object params exist
        self.assertIn('body["name"] = flagWidgetsUpdateName', content)

    def test_object_body_param_error_message_uses_flag_name(self):
        spec, config = self._make_spec_and_config()
        content = generate_resource_file("widgets", config, spec, spec["servers"])

        # Error message must reference the CLI flag name
        self.assertIn("--connector must be a JSON object", content)

    def test_no_object_params_unchanged(self):
        spec = {
            "servers": [{"url": "https://api.supermetrics.com/v2"}],
            "paths": {
                "/items": {
                    "post": {
                        "operationId": "createItem",
                        "requestBody": {
                            "content": {
                                "application/json": {
                                    "schema": {
                                        "type": "object",
                                        "properties": {
                                            "title": {"type": "string", "description": "Item title"},
                                            "count": {"type": "integer", "description": "Item count"},
                                        },
                                        "required": ["title", "count"],
                                    }
                                }
                            }
                        },
                    }
                }
            },
        }
        config = {
            "description": "Item management",
            "commands": {
                "create": {
                    "operation_id": "createItem",
                    "description": "Create an item",
                },
            },
        }
        content = generate_resource_file("items", config, spec, spec["servers"])

        # No object params: must use map literal syntax
        self.assertIn("body := map[string]any{", content)
        self.assertNotIn("json.Unmarshal", content)


class TestBooleanQueryParams(unittest.TestCase):
    """Test that boolean query params generate the correct conditional set code."""

    def _make_spec_and_config(self):
        spec = {
            "servers": [{"url": "https://api.supermetrics.com/v2"}],
            "paths": {
                "/reports": {
                    "get": {
                        "operationId": "listReports",
                        "parameters": [
                            {
                                "name": "include_archived",
                                "in": "query",
                                "required": False,
                                "description": "Include archived reports",
                                "schema": {"type": "boolean"},
                            },
                        ],
                    }
                }
            },
        }
        config = {
            "description": "Report management",
            "commands": {
                "list": {
                    "operation_id": "listReports",
                    "description": "List reports",
                },
            },
        }
        return spec, config

    def test_bool_query_param_generates_conditional_set(self):
        spec, config = self._make_spec_and_config()
        content = generate_resource_file("reports", config, spec, spec["servers"])

        # Bool param must emit q.Set with "true" string value
        self.assertIn('q.Set("include_archived", "true")', content)

    def test_bool_query_param_not_set_when_false(self):
        spec, config = self._make_spec_and_config()
        content = generate_resource_file("reports", config, spec, spec["servers"])

        # The set must be wrapped in an if-condition on the flag variable (not unconditional)
        var_name = "flagReportsListIncludeArchived"
        self.assertIn(f"if {var_name} {{", content)

        # Must NOT set "false" unconditionally — only "true" inside the condition
        self.assertNotIn('q.Set("include_archived", "false")', content)


class TestSnakeToCamelSanitizes(unittest.TestCase):
    """Non-alphanumeric characters must produce valid, unique Go identifiers."""

    def test_bracketed_param(self):
        self.assertEqual(snake_to_camel("filter[category]"), "FilterCategory")

    def test_bracketed_snake_param(self):
        self.assertEqual(snake_to_camel("filter[is_new]"), "FilterIsNew")

    def test_distinct_brackets_are_unique(self):
        self.assertNotEqual(
            snake_to_camel("filter[category]"),
            snake_to_camel("filter[status]"),
        )


class TestGoStringEscape(unittest.TestCase):
    """Descriptions embedded in Go string literals must be escaped."""

    def test_newline(self):
        self.assertEqual(go_string_escape("line1\nline2"), "line1\\nline2")

    def test_quote(self):
        self.assertEqual(go_string_escape('say "hi"'), 'say \\"hi\\"')

    def test_backslash_and_tab(self):
        self.assertEqual(go_string_escape("a\\b\tc"), "a\\\\b\\tc")

    def test_none(self):
        self.assertEqual(go_string_escape(None), "")


class TestResolveOperationServer(unittest.TestCase):
    """Server resolution must prefer per-path/op servers, then fall back to the global index."""

    def test_path_level_server_wins(self):
        spec = {
            "servers": [{"url": "https://api.supermetrics.com/v2"}],
            "paths": {
                "/teams/{team_id}/backfills": {
                    "servers": [{"url": "https://dts-api.supermetrics.com/v1"}],
                },
            },
        }
        subdomain, prefix = resolve_operation_server(
            spec, "/teams/{team_id}/backfills", {}, spec["servers"], 0,
        )
        self.assertEqual((subdomain, prefix), ("dts-api", "/v1"))

    def test_operation_level_server_wins_over_path(self):
        spec = {
            "servers": [],
            "paths": {"/x": {"servers": [{"url": "https://api.supermetrics.com"}]}},
        }
        op = {"servers": [{"url": "https://dts-api.supermetrics.com/v1"}]}
        subdomain, prefix = resolve_operation_server(spec, "/x", op, spec["servers"], 0)
        self.assertEqual((subdomain, prefix), ("dts-api", "/v1"))

    def test_falls_back_to_global_server_index(self):
        # Old-style spec: no per-path servers, only a global servers list + resource index.
        spec = {
            "servers": [
                {"url": "https://api.supermetrics.com/v2"},
                {"url": "https://dts-api.supermetrics.com/v1"},
            ],
            "paths": {"/x": {}},
        }
        subdomain, prefix = resolve_operation_server(spec, "/x", {}, spec["servers"], 1)
        self.assertEqual((subdomain, prefix), ("dts-api", "/v1"))

    def test_default_when_no_servers(self):
        spec = {"servers": [], "paths": {"/x": {}}}
        subdomain, prefix = resolve_operation_server(spec, "/x", {}, [], 0)
        self.assertEqual((subdomain, prefix), ("api", ""))


class TestPerPathServerGeneration(unittest.TestCase):
    """Generated URLs must use each operation's own server, not a single global one."""

    def _make_spec(self):
        return {
            "servers": [{"url": "https://api.supermetrics.com/v2"}],
            "paths": {
                "/query/accounts": {
                    "servers": [{"url": "https://api.supermetrics.com"}],
                    "get": {"operationId": "getAccounts", "parameters": []},
                },
                "/teams/{team_id}/backfills": {
                    "servers": [{"url": "https://dts-api.supermetrics.com/v1"}],
                    "get": {
                        "operationId": "listBackfills",
                        "parameters": [
                            {
                                "name": "team_id",
                                "in": "path",
                                "required": True,
                                "description": "Team",
                                "schema": {"type": "integer"},
                            },
                        ],
                    },
                },
            },
        }

    def test_two_commands_resolve_distinct_hosts(self):
        spec = self._make_spec()
        config = {
            "description": "Mixed",
            "commands": {
                "accounts": {"operation_id": "getAccounts", "description": "Accounts"},
                "backfills": {"operation_id": "listBackfills", "description": "Backfills"},
            },
        }
        content = generate_resource_file("mixed", config, spec, spec["servers"])
        # api host, no /v2 prefix
        self.assertIn('"https://api." + domain + "/query/accounts"', content)
        # dts-api host with /v1 prefix (raw generator output, before gofmt tightens spacing)
        self.assertIn('"https://dts-api." + domain + "/v1/teams/{team_id}/backfills"', content)


class TestSkipMissingOperation(unittest.TestCase):
    """Commands whose operationId is absent from the spec are skipped, not fatal."""

    def test_missing_op_is_skipped(self):
        spec = {
            "servers": [{"url": "https://api.supermetrics.com"}],
            "paths": {
                "/things": {"get": {"operationId": "listThings", "parameters": []}},
            },
        }
        config = {
            "description": "Things",
            "commands": {
                "list": {"operation_id": "listThings", "description": "List"},
                "ghost": {"operation_id": "doesNotExist", "description": "Ghost"},
            },
        }
        content = generate_resource_file("things", config, spec, spec["servers"])
        # Present op generates its command var; missing op does not.
        self.assertIn("ThingsListCmd", content)
        self.assertNotIn("ThingsGhostCmd", content)


class TestActiveResources(unittest.TestCase):
    """Resource groups with no operations present in the spec are excluded entirely."""

    def _spec(self):
        return {
            "servers": [{"url": "https://api.supermetrics.com"}],
            "paths": {"/things": {"get": {"operationId": "listThings", "parameters": []}}},
        }

    def test_present_group_is_kept(self):
        resources = {"things": {"commands": {"list": {"operation_id": "listThings"}}}}
        self.assertIn("things", active_resources(resources, self._spec()))

    def test_fully_missing_group_is_dropped(self):
        resources = {
            "things": {"commands": {"list": {"operation_id": "listThings"}}},
            "ghosts": {"commands": {"list": {"operation_id": "listGhosts"}}},
        }
        result = active_resources(resources, self._spec())
        self.assertIn("things", result)
        self.assertNotIn("ghosts", result)

    def test_partially_present_group_is_kept(self):
        resources = {
            "things": {
                "commands": {
                    "list": {"operation_id": "listThings"},
                    "ghost": {"operation_id": "doesNotExist"},
                },
            },
        }
        self.assertIn("things", active_resources(resources, self._spec()))


class TestAsyncFlatQueryEnvelope(unittest.TestCase):
    """Async commands route flat query params through the ?json= envelope."""

    def _make_spec(self):
        return {
            "servers": [{"url": "https://api.supermetrics.com"}],
            "paths": {
                "/query/data/{context_type}": {
                    "servers": [{"url": "https://api.supermetrics.com"}],
                    "get": {
                        "operationId": "getData",
                        "parameters": [
                            {
                                "name": "context_type",
                                "in": "path",
                                "required": True,
                                "description": "Context",
                                "schema": {"type": "string"},
                            },
                            {
                                "name": "ds_id",
                                "in": "query",
                                "required": True,
                                "description": "Data source",
                                "schema": {"type": "string"},
                            },
                        ],
                    },
                },
            },
        }

    def test_async_uses_json_envelope_for_flat_query(self):
        spec = self._make_spec()
        config = {
            "description": "Queries",
            "commands": {
                "execute": {
                    "operation_id": "getData",
                    "description": "Execute",
                    "async": True,
                },
            },
        }
        content = generate_resource_file("queries", config, spec, spec["servers"])
        # The async helpers require jsonParams + baseURL to be defined.
        self.assertIn("jsonParams := map[string]any{", content)
        self.assertIn("baseURL := requestURL", content)
        self.assertIn('requestURL += "?json=" + url.QueryEscape', content)


if __name__ == "__main__":
    unittest.main()
