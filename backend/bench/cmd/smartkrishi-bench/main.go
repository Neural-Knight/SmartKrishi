// Command smartkrishi-bench is the benchmark orchestrator for the SmartKrishi
// backend. It has three subcommands:
//
//	run      execute a benchmark suite (mode e2e | controlled | e2e_no_gemini)
//	         and write a results JSON file.
//	compare  diff a baseline suite against a current suite (same mode only).
//	report   render a human-readable table from a results JSON file.
//
// The tool never fabricates numbers: every metric comes from a real run, and an
// unmeasured field stays zero. Modes are labeled explicitly in the output and
// are never mixed within one results file.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/smartkrishi/backend/bench/internal/compare"
	"github.com/smartkrishi/backend/bench/internal/report"
	"github.com/smartkrishi/backend/bench/internal/schema"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "run":
		err = runCmd(ctx, os.Args[2:])
	case "compare":
		err = compareCmd(os.Args[2:])
	case "report":
		err = reportCmd(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `smartkrishi-bench — SmartKrishi backend benchmark harness

Usage:
  smartkrishi-bench run     --mode <e2e|controlled|e2e_no_gemini> [flags]
  smartkrishi-bench compare --baseline <file> --current <file> [--output <file>]
  smartkrishi-bench report  --input <file> [--output <file>]

Run 'smartkrishi-bench <command> -h' for command-specific flags.

Modes:
  controlled     in-process server, scripted LLM + stubbed tool HTTP, real Postgres.
                 Deterministic; compare controlled-to-controlled only.
  e2e            live server with real Gemini/Postgres/tool APIs. High variance;
                 indicative only.
  e2e_no_gemini  live server, non-agent scenarios only (health, chat CRUD).
`)
}

// compareCmd diffs two result suites of the SAME mode and prints/writes a table.
func compareCmd(args []string) error {
	fs := newFlagSet("compare")
	baseline := fs.String("baseline", "", "baseline results JSON file")
	current := fs.String("current", "", "current results JSON file")
	output := fs.String("output", "", "optional output file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *baseline == "" || *current == "" {
		return fmt.Errorf("compare requires --baseline and --current")
	}

	base, err := schema.ReadSuite(*baseline)
	if err != nil {
		return err
	}
	cur, err := schema.ReadSuite(*current)
	if err != nil {
		return err
	}
	if base.Mode != cur.Mode {
		return fmt.Errorf("refusing to compare across modes: baseline=%q current=%q", base.Mode, cur.Mode)
	}

	table, err := compare.Compare(base, cur)
	if err != nil {
		return err
	}
	if *output != "" {
		if err := os.WriteFile(*output, []byte(table), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote comparison to %s\n", *output)
		return nil
	}
	fmt.Print(table)
	return nil
}

// reportCmd renders a human-readable table from a results suite.
func reportCmd(args []string) error {
	fs := newFlagSet("report")
	input := fs.String("input", "", "results JSON file")
	output := fs.String("output", "", "optional output file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" {
		return fmt.Errorf("report requires --input")
	}
	suite, err := schema.ReadSuite(*input)
	if err != nil {
		return err
	}
	text := report.Render(suite)
	if *output != "" {
		if err := os.WriteFile(*output, []byte(text), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote report to %s\n", *output)
		return nil
	}
	fmt.Print(text)
	return nil
}
