"""Map OpenAPI types to Go equivalents — flag types, flag functions, zero values, var types, timeouts."""


def go_flag_type(param):
    """Map parameter type to Go flag type."""
    t = param["type"]
    fmt = param.get("format", "")
    if t == "integer":
        if fmt == "int64":
            return "int64"
        return "int"
    if t == "number":
        return "float64"
    if t == "boolean":
        return "bool"
    if t == "array":
        return "stringSlice"
    return "string"


def go_flag_func(param):
    """Return the cobra flag function name for a parameter."""
    ft = go_flag_type(param)
    mapping = {
        "string": "StringVar",
        "int": "IntVar",
        "int64": "Int64Var",
        "float64": "Float64Var",
        "bool": "BoolVar",
        "stringSlice": "StringSliceVar",
    }
    return mapping.get(ft, "StringVar")


def go_zero_value(param):
    """Return Go zero value for a parameter type."""
    ft = go_flag_type(param)
    return {
        "string": '""',
        "int": "0",
        "int64": "0",
        "float64": "0",
        "bool": "false",
        "stringSlice": "nil",
    }.get(ft, '""')


def go_var_type(param):
    """Return Go variable type for a parameter."""
    ft = go_flag_type(param)
    return {
        "string": "string",
        "int": "int",
        "int64": "int64",
        "float64": "float64",
        "bool": "bool",
        "stringSlice": "[]string",
    }.get(ft, "string")


def parse_timeout(timeout_str):
    """Convert a timeout string like '60m' or '30s' to a Go time.Duration expression."""
    if not timeout_str:
        return "httpclient.DefaultTimeout"
    if timeout_str.endswith("m"):
        minutes = int(timeout_str[:-1])
        return f"{minutes} * time.Minute"
    if timeout_str.endswith("s"):
        seconds = int(timeout_str[:-1])
        return f"{seconds} * time.Second"
    return "httpclient.DefaultTimeout"
