#!/usr/bin/env python3
"""Tests for generate_commands.py."""

import unittest

from generate_commands import MAPPING_PATH
from generate_commands import SPEC_PATH
from generate_commands import extract_params
from generate_commands import find_operation
from generate_commands import generate_register_files
from generate_commands import generate_resource_file
from generate_commands import go_flag_func
from generate_commands import go_flag_type
from generate_commands import go_var_type
from generate_commands import go_zero_value
from generate_commands import load_yaml
from generate_commands import parse_server_url
from generate_commands import parse_timeout
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


if __name__ == "__main__":
    unittest.main()
