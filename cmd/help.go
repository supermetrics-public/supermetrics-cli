package cmd

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

const (
	ansiReset      = "\033[0m"
	ansiBold       = "\033[1m"
	ansiDim        = "\033[2m"
	ansiGreen      = "\033[32m"
	ansiYellow     = "\033[33m"
	ansiCyan       = "\033[36m"
	ansiBoldYellow = "\033[1;33m"
)

var (
	sectionHeaderRe = regexp.MustCompile(`(?m)^(\w[\w ]*):\s*$`)
	flagNameRe      = regexp.MustCompile(`(--[\w-]+|-\w\b)`)
	defaultValueRe  = regexp.MustCompile(`(\(default .+?\))`)
	footerHintRe    = regexp.MustCompile(`(?m)^Use ".+".*$`)
)

func shouldColorHelp() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// colorizeHelp applies ANSI colors to rendered Cobra help text.
func colorizeHelp(text string) string {
	var result strings.Builder
	lines := strings.Split(text, "\n")

	section := "" // tracks which section we're in

	for _, line := range lines {
		// Detect section headers like "Usage:", "Available Commands:", "Flags:"
		if sectionHeaderRe.MatchString(line) {
			section = strings.TrimSuffix(strings.TrimSpace(line), ":")
			result.WriteString(ansiBoldYellow + line + ansiReset + "\n")
			continue
		}

		// Footer hint line — dim the entire line, no other coloring
		if footerHintRe.MatchString(line) {
			result.WriteString(ansiDim + line + ansiReset + "\n")
			section = "" // stop coloring after footer
			continue
		}

		switch section {
		case "Available Commands":
			line = colorizeCommandLine(line)
		case "Flags", "Global Flags":
			line = colorizeFlagLine(line)
		}

		result.WriteString(line + "\n")
	}

	// Remove trailing extra newline (we added one per line)
	out := result.String()
	if strings.HasSuffix(out, "\n") && strings.HasSuffix(text, "\n") {
		out = strings.TrimSuffix(out, "\n")
	}

	return out
}

// colorizeCommandLine colors command names in "Available Commands" section.
// Lines look like: "  queries     Execute data queries..."
func colorizeCommandLine(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	if trimmed == "" {
		return line
	}
	indent := line[:len(line)-len(trimmed)]
	parts := strings.SplitN(trimmed, " ", 2)
	if len(parts) == 0 {
		return line
	}
	colored := indent + ansiCyan + parts[0] + ansiReset
	if len(parts) > 1 {
		colored += " " + parts[1]
	}
	return colored
}

// colorizeFlagLine colors flag names and defaults in flag lines.
// Lines look like: "  -o, --output string    Output format (default "json")"
func colorizeFlagLine(line string) string {
	// Color flag names (--foo, -f)
	line = flagNameRe.ReplaceAllString(line, ansiGreen+"$1"+ansiReset)
	// Color default values
	line = defaultValueRe.ReplaceAllString(line, ansiDim+"$1"+ansiReset)
	return line
}

// initHelpColorization sets up colored help output on the root command.
// All subcommands inherit this automatically.
func initHelpColorization(root *cobra.Command) {
	defaultHelp := root.HelpFunc()

	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if !shouldColorHelp() {
			defaultHelp(cmd, args)
			return
		}

		// Capture default help output into a buffer
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		defaultHelp(cmd, args)
		cmd.SetOut(os.Stdout) // restore

		fmt.Print(colorizeHelp(buf.String()))
	})
}
