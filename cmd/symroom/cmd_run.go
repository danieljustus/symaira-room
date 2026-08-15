package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/danieljustus/symaira-corekit/exitcodes"
	"github.com/danieljustus/symaira-room/internal/approval"
	"github.com/danieljustus/symaira-room/internal/run"
)

// runRun implements the "run" subcommand.
func runRun(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		_, _ = fmt.Fprintln(stdout, "Usage: symroom run <request|list|show|start|cancel> [flags] [args]")
		return int(exitcodes.ExitOK)
	}
	sub := args[2]
	switch sub {
	case "request":
		fs := flag.NewFlagSet("run request", flag.ExitOnError)
		titleFlag := fs.String("title", "", "Run title")
		planFlag := fs.String("plan-file", "", "Plan file path")
		adapterFlag := fs.String("adapter", "", "Adapter name")
		idFlag := fs.String("identity", "", "Author identity name")
		if err := fs.Parse(args[3:]); err != nil {
			return int(exitcodes.ExitNoInput)
		}
		if *titleFlag == "" {
			fmt.Fprintln(stderr, "Error: --title is required")
			return int(exitcodes.ExitNoInput)
		}
		id := resolveIdentity(*idFlag)
		ev, err := run.Request(roomDir(), *titleFlag, *planFlag, *adapterFlag, id)
		if err != nil {
			fmt.Fprintf(stderr, "Error requesting run: %v\n", err)
			return int(exitcodes.ExitGeneric)
		}
		fmt.Fprintln(stdout, ev.ID)
		return int(exitcodes.ExitOK)

	case "list":
		fs := flag.NewFlagSet("run list", flag.ExitOnError)
		pendingFlag := fs.Bool("pending", false, "Show pending runs only")
		jsonFlag := fs.Bool("json", false, "Output as JSON")
		if err := fs.Parse(args[3:]); err != nil {
			return int(exitcodes.ExitNoInput)
		}
		runs, err := run.List(roomDir(), *pendingFlag)
		if err != nil {
			fmt.Fprintf(stderr, "Error listing runs: %v\n", err)
			return int(exitcodes.ExitGeneric)
		}
		if *jsonFlag {
			data, _ := json.MarshalIndent(runs, "", "  ")
			fmt.Fprintln(stdout, string(data))
		} else {
			for _, r := range runs {
				fmt.Fprintf(stdout, "%s\t[%s]\t%s\t%s\n", r.ID, r.State, r.Author, r.Title)
			}
		}
		return int(exitcodes.ExitOK)

	case "show":
		fs := flag.NewFlagSet("run show", flag.ExitOnError)
		jsonFlag := fs.Bool("json", false, "Output as JSON")
		if err := fs.Parse(args[3:]); err != nil {
			return int(exitcodes.ExitNoInput)
		}
		if fs.NArg() < 1 {
			fmt.Fprintln(stderr, "Usage: symroom run show <run_id> [--json]")
			return int(exitcodes.ExitNoInput)
		}
		r, err := run.Get(roomDir(), fs.Arg(0))
		if err != nil {
			if errors.Is(err, run.ErrRunNotFound) {
				fmt.Fprintf(stderr, "Error: run %s not found\n", fs.Arg(0))
				return int(exitcodes.ExitNotFound)
			}
			fmt.Fprintf(stderr, "Error showing run: %v\n", err)
			return int(exitcodes.ExitGeneric)
		}
		if *jsonFlag {
			data, _ := json.MarshalIndent(r, "", "  ")
			fmt.Fprintln(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "Run ID:     %s\n", r.ID)
			fmt.Fprintf(stdout, "Title:      %s\n", r.Title)
			fmt.Fprintf(stdout, "State:      %s\n", r.State)
			fmt.Fprintf(stdout, "Author:     %s\n", r.Author)
			fmt.Fprintf(stdout, "Created At: %s\n", r.CreatedAt)
			if r.Summary != "" {
				fmt.Fprintf(stdout, "Summary:    %s\n", r.Summary)
			}
			if r.Error != "" {
				fmt.Fprintf(stdout, "Error:      %s\n", r.Error)
			}
		}
		return int(exitcodes.ExitOK)

	case "start":
		fs := flag.NewFlagSet("run start", flag.ExitOnError)
		idFlag := fs.String("identity", "", "Author identity name")
		if err := fs.Parse(args[3:]); err != nil {
			return int(exitcodes.ExitNoInput)
		}
		if fs.NArg() < 1 {
			fmt.Fprintln(stderr, "Usage: symroom run start <run_id> [--identity <name>]")
			return int(exitcodes.ExitNoInput)
		}
		id := resolveIdentity(*idFlag)
		ev, err := run.Start(roomDir(), fs.Arg(0), id)
		if err != nil {
			if errors.Is(err, run.ErrInvalidTransition) {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return int(exitcodes.ExitNoInput)
			}
			fmt.Fprintf(stderr, "Error starting run: %v\n", err)
			return int(exitcodes.ExitGeneric)
		}
		fmt.Fprintln(stdout, ev.ID)
		return int(exitcodes.ExitOK)

	case "cancel":
		fs := flag.NewFlagSet("run cancel", flag.ExitOnError)
		reasonFlag := fs.String("reason", "", "Reason for cancellation")
		idFlag := fs.String("identity", "", "Author identity name")
		if err := fs.Parse(args[3:]); err != nil {
			return int(exitcodes.ExitNoInput)
		}
		if fs.NArg() < 1 {
			fmt.Fprintln(stderr, "Usage: symroom run cancel <run_id> [--reason ...] [--identity <name>]")
			return int(exitcodes.ExitNoInput)
		}
		id := resolveIdentity(*idFlag)
		ev, err := run.Cancel(roomDir(), fs.Arg(0), *reasonFlag, id)
		if err != nil {
			if errors.Is(err, run.ErrInvalidTransition) {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return int(exitcodes.ExitNoInput)
			}
			fmt.Fprintf(stderr, "Error cancelling run: %v\n", err)
			return int(exitcodes.ExitGeneric)
		}
		fmt.Fprintln(stdout, ev.ID)
		return int(exitcodes.ExitOK)

	case "wait":
		fs := flag.NewFlagSet("run wait", flag.ExitOnError)
		timeoutFlag := fs.Duration("timeout", 15*time.Minute, "Timeout duration")
		jsonFlag := fs.Bool("json", false, "Output as JSON")
		if err := fs.Parse(args[3:]); err != nil {
			return int(exitcodes.ExitNoInput)
		}
		if fs.NArg() < 1 {
			fmt.Fprintln(stderr, "Usage: symroom run wait <run_id> [--timeout 15m] [--json]")
			return int(exitcodes.ExitNoInput)
		}
		runID := fs.Arg(0)
		ctx, cancel := context.WithTimeout(ctx, *timeoutFlag)
		defer cancel()

		r, err := run.Wait(ctx, roomDir(), runID, 500*time.Millisecond)
		if err != nil {
			if errors.Is(err, run.ErrWaitTimeout) {
				fmt.Fprintf(stderr, "Error: wait timed out for run %s\n", runID)
				return int(exitcodes.ExitInterrupted)
			}
			if errors.Is(err, run.ErrRunDenied) {
				fmt.Fprintf(stderr, "Error: run %s was denied\n", runID)
				return int(exitcodes.ExitForbidden)
			}
			fmt.Fprintf(stderr, "Error waiting for run %s: %v\n", runID, err)
			return int(exitcodes.ExitGeneric)
		}

		if *jsonFlag {
			data, _ := json.MarshalIndent(r, "", "  ")
			fmt.Fprintln(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "Run %s approved [%s]\n", r.ID, r.Scope)
		}
		return int(exitcodes.ExitOK)

	case "approve":
		fs := flag.NewFlagSet("run approve", flag.ExitOnError)
		scopeFlag := fs.String("scope", "all", "Approval scope")
		ttlFlag := fs.Duration("ttl", 30*time.Minute, "Approval TTL duration")
		idFlag := fs.String("identity", "", "Author identity name")
		if err := fs.Parse(args[3:]); err != nil {
			return int(exitcodes.ExitNoInput)
		}
		if fs.NArg() < 1 {
			fmt.Fprintln(stderr, "Usage: symroom run approve <run_id> [--scope ...] [--ttl 30m] [--identity <name>]")
			return int(exitcodes.ExitNoInput)
		}
		id := resolveIdentity(*idFlag)
		ev, err := approval.Approve(roomDir(), fs.Arg(0), *scopeFlag, *ttlFlag, id)
		if err != nil {
			if errors.Is(err, approval.ErrAgentApprovalForbidden) {
				fmt.Fprintln(stderr, "Error: agent identity is forbidden from approving runs")
				return int(exitcodes.ExitNoInput)
			}
			fmt.Fprintf(stderr, "Error approving run: %v\n", err)
			return int(exitcodes.ExitGeneric)
		}
		fmt.Fprintln(stdout, ev.ID)
		return int(exitcodes.ExitOK)

	case "deny":
		fs := flag.NewFlagSet("run deny", flag.ExitOnError)
		reasonFlag := fs.String("reason", "", "Reason for denial")
		idFlag := fs.String("identity", "", "Author identity name")
		if err := fs.Parse(args[3:]); err != nil {
			return int(exitcodes.ExitNoInput)
		}
		if fs.NArg() < 1 || *reasonFlag == "" {
			fmt.Fprintln(stderr, "Usage: symroom run deny <run_id> --reason ... [--identity <name>]")
			return int(exitcodes.ExitNoInput)
		}
		id := resolveIdentity(*idFlag)
		ev, err := approval.Deny(roomDir(), fs.Arg(0), *reasonFlag, id)
		if err != nil {
			fmt.Fprintf(stderr, "Error denying run: %v\n", err)
			return int(exitcodes.ExitGeneric)
		}
		fmt.Fprintln(stdout, ev.ID)
		return int(exitcodes.ExitOK)

	default:
		fmt.Fprintf(stderr, "Unknown run action: %s\n", sub)
		return int(exitcodes.ExitNoInput)
	}
}
