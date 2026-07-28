package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/danieljustus/symaira-corekit/exitcodes"
)

const usageText = `symroom - room management and coordination tool

Usage:
  symroom <subcommand> [flags] [args]

Available Subcommands:
  init           Initialize a room
  identity       Manage Ed25519 identities
  member         Manage room members
  note           Post a journal note
  decide         Record a room decision
  artifact       Manage room artifacts
  log            Display room journal log
  verify         Verify journal chains and signatures
  index          Rebuild or manage derived SQLite index
  run            Manage room runs
  checkpoint     Manage run checkpoints
  watch          Watch symdesk events stream
  brain-profile  Emit a symbrain profile
  doctor         Run system and environment checks
  version        Print version information
  mcp            Run MCP server mode

Use "symroom <subcommand> --help" for more information about a subcommand.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(int(exitcodes.ExitNoInput))
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "init", "identity", "member", "note", "decide", "artifact",
		"log", "verify", "index", "run", "checkpoint", "watch",
		"brain-profile", "doctor", "version", "mcp":
		fs := flag.NewFlagSet(subcommand, flag.ExitOnError)
		fs.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: symroom %s [flags]\n", subcommand)
			fs.PrintDefaults()
		}
		if err := fs.Parse(os.Args[2:]); err != nil {
			os.Exit(int(exitcodes.ExitNoInput))
		}
		fmt.Printf("symroom %s stub\n", subcommand)
		os.Exit(int(exitcodes.ExitOK))
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, usageText)
		os.Exit(int(exitcodes.ExitOK))
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n%s", subcommand, usageText)
		os.Exit(int(exitcodes.ExitNoInput))
	}
}
