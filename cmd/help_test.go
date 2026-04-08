package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColorizeHelp_SectionHeaders(t *testing.T) {
	input := "Usage:\n  supermetrics [command]\n"
	output := colorizeHelp(input)

	assert.Contains(t, output, ansiBoldYellow+"Usage:"+ansiReset, "section header should be bold yellow")
}

func TestColorizeHelp_CommandNames(t *testing.T) {
	input := "Available Commands:\n  queries     Execute data queries\n  backfills   Manage backfills\n"
	output := colorizeHelp(input)

	assert.Contains(t, output, ansiCyan+"queries"+ansiReset, "command name should be cyan")
	assert.Contains(t, output, ansiCyan+"backfills"+ansiReset, "command name should be cyan")
	// Description should not be colored
	assert.NotContains(t, output, ansiCyan+"Execute", "command description should not be cyan")
}

func TestColorizeHelp_FlagNames(t *testing.T) {
	input := "Flags:\n  -o, --output string    Output format\n      --verbose           Enable verbose\n"
	output := colorizeHelp(input)

	assert.Contains(t, output, ansiGreen+"--output"+ansiReset, "long flag should be green")
	assert.Contains(t, output, ansiGreen+"-o"+ansiReset, "short flag should be green")
	assert.Contains(t, output, ansiGreen+"--verbose"+ansiReset, "flag without short form should be green")
}

func TestColorizeHelp_DefaultValues(t *testing.T) {
	input := "Flags:\n  --output string    Output format (default \"json\")\n"
	output := colorizeHelp(input)

	assert.Contains(t, output, ansiDim+`(default "json")`+ansiReset, "default value should be dim")
}

func TestColorizeHelp_FooterHint(t *testing.T) {
	input := "Flags:\n  -h, --help   help\n\nUse \"supermetrics [command] --help\" for more information about a command.\n"
	output := colorizeHelp(input)

	// Footer should be dim
	assert.Contains(t, output, ansiDim+`Use "supermetrics [command] --help" for more information about a command.`+ansiReset, "footer hint should be fully dim")
	// --help in footer should NOT be individually green
	assert.NotContains(t, output, ansiGreen+"--help"+ansiReset+`" for more`, "flag names inside footer should not be individually colored")
}

func TestColorizeHelp_GlobalFlags(t *testing.T) {
	input := "Global Flags:\n      --verbose          Enable verbose output\n"
	output := colorizeHelp(input)

	assert.Contains(t, output, ansiBoldYellow+"Global Flags:"+ansiReset, "Global Flags header should be bold yellow")
	assert.Contains(t, output, ansiGreen+"--verbose"+ansiReset, "global flag should be green")
}

func TestColorizeHelp_PlainTextUnchanged(t *testing.T) {
	input := "Execute a data query\n\nUsage:\n  supermetrics queries execute [flags]\n"
	output := colorizeHelp(input)

	// Description line should have no color codes
	descLine := strings.Split(output, "\n")[0]
	assert.NotContains(t, descLine, "\033[", "plain description should not contain ANSI codes")
}
