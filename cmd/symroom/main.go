package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/danieljustus/symaira-corekit/exitcodes"
	"github.com/danieljustus/symaira-room/internal/approval"
	"github.com/danieljustus/symaira-room/internal/artifact"
	"github.com/danieljustus/symaira-room/internal/brainprofile"
	"github.com/danieljustus/symaira-room/internal/config"
	"github.com/danieljustus/symaira-room/internal/desk"
	"github.com/danieljustus/symaira-room/internal/doctor"
	"github.com/danieljustus/symaira-room/internal/identity"
	"github.com/danieljustus/symaira-room/internal/index"
	"github.com/danieljustus/symaira-room/internal/journal"
	"github.com/danieljustus/symaira-room/internal/mcp"
	"github.com/danieljustus/symaira-room/internal/members"
	"github.com/danieljustus/symaira-room/internal/room"
	"github.com/danieljustus/symaira-room/internal/run"
	"github.com/danieljustus/symaira-room/internal/version"
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
	case "version":
		fs := flag.NewFlagSet("version", flag.ExitOnError)
		jsonFlag := fs.Bool("json", false, "Emit version info in JSON format")
		if err := fs.Parse(os.Args[2:]); err != nil {
			os.Exit(int(exitcodes.ExitNoInput))
		}
		info := version.GetInfo()
		if *jsonFlag {
			if err := info.Write(os.Stdout); err != nil {
				os.Exit(int(exitcodes.ExitGeneric))
			}
		} else {
			fmt.Println(info.String())
		}
		os.Exit(int(exitcodes.ExitOK))
	case "identity":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stdout, "Usage: symroom identity <create|list|show|export> [args]")
			os.Exit(int(exitcodes.ExitOK))
		}
		action := os.Args[2]
		switch action {
		case "create":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Usage: symroom identity create <name>")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			name := os.Args[3]
			id, err := identity.Generate(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating identity: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			if err := identity.Save(id); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving identity: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Printf("Created identity %s (%s)\n", id.Name, id.MemberID)
			os.Exit(int(exitcodes.ExitOK))
		case "list":
			names, err := identity.List()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing identities: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			for _, n := range names {
				fmt.Println(n)
			}
			os.Exit(int(exitcodes.ExitOK))
		case "show":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Usage: symroom identity show <name>")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			name := os.Args[3]
			id, err := identity.Load(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading identity: %v\n", err)
				os.Exit(int(exitcodes.ExitNotFound))
			}
			fmt.Printf("Name: %s\nMember ID: %s\nPublic Key: %x\n", id.Name, id.MemberID, id.PublicKey)
			os.Exit(int(exitcodes.ExitOK))
		case "export":
			fs := flag.NewFlagSet("identity export", flag.ExitOnError)
			pubFlag := fs.Bool("public", false, "Export public key only")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if fs.NArg() < 1 {
				fmt.Fprintln(os.Stderr, "Usage: symroom identity export <name> --public")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			name := fs.Arg(0)
			id, err := identity.Load(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading identity: %v\n", err)
				os.Exit(int(exitcodes.ExitNotFound))
			}
			if *pubFlag {
				fmt.Println(hex.EncodeToString(id.PublicKey))
			} else {
				fmt.Fprintf(os.Stderr, "Exporting private key is forbidden for security\n")
				os.Exit(int(exitcodes.ExitForbidden))
			}
			os.Exit(int(exitcodes.ExitOK))
		default:
			fmt.Fprintf(os.Stderr, "Unknown identity action: %s\n", action)
			os.Exit(int(exitcodes.ExitNoInput))
		}
	case "init":
		fs := flag.NewFlagSet("init", flag.ExitOnError)
		nameFlag := fs.String("name", "Default Room", "Room display name")
		idFlag := fs.String("identity", "", "Owner identity name")
		if err := fs.Parse(os.Args[2:]); err != nil {
			os.Exit(int(exitcodes.ExitNoInput))
		}

		targetDir := "."
		if fs.NArg() > 0 {
			targetDir = fs.Arg(0)
		}

		idName := *idFlag
		if idName == "" {
			cfg := config.LoadOrExit()
			idName = cfg.DefaultIdentity
		}
		if idName == "" && fs.NArg() == 0 {
			fmt.Println("Usage: symroom init <dir> --identity <name> [--name <display_name>]")
			os.Exit(int(exitcodes.ExitOK))
		}
		if idName == "" {
			fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
			os.Exit(int(exitcodes.ExitNoInput))
		}

		id, err := identity.Load(idName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
			os.Exit(int(exitcodes.ExitNotFound))
		}

		roomCfg, err := room.Init(targetDir, *nameFlag, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing room: %v\n", err)
			os.Exit(int(exitcodes.ExitGeneric))
		}

		fmt.Printf("Initialized room %s in %s (owner: %s)\n", roomCfg.ID, targetDir, id.Name)
		os.Exit(int(exitcodes.ExitOK))
	case "member":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stdout, "Usage: symroom member <add|list|remove|role> [flags] [args]")
			os.Exit(int(exitcodes.ExitOK))
		}
		sub := os.Args[2]
		switch sub {
		case "add":
			fs := flag.NewFlagSet("member add", flag.ExitOnError)
			pubFlag := fs.String("pubkey", "", "Member public key (hex)")
			nameFlag := fs.String("name", "", "Member display name")
			roleFlag := fs.String("role", "member", "Member role (owner|member|agent|observer)")
			kindFlag := fs.String("kind", "human", "Member kind (human|agent)")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if *pubFlag == "" || *nameFlag == "" {
				fmt.Fprintln(os.Stderr, "Usage: symroom member add --pubkey <hex> --name <name> [--role <role>] [--kind <kind>]")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			pubBytes, err := hex.DecodeString(*pubFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid public key hex: %v\n", err)
				os.Exit(int(exitcodes.ExitNoInput))
			}
			memID := identity.ComputeMemberID(pubBytes)
			fmt.Printf("Member added: %s (%s, role: %s, kind: %s)\n", *nameFlag, memID, *roleFlag, *kindFlag)
			os.Exit(int(exitcodes.ExitOK))
		case "list":
			fmt.Println("Members: (run symroom log or index to view members state)")
			os.Exit(int(exitcodes.ExitOK))
		case "remove":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Usage: symroom member remove <member_id>")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			fmt.Printf("Member removed: %s\n", os.Args[3])
			os.Exit(int(exitcodes.ExitOK))
		case "role":
			if len(os.Args) < 5 {
				fmt.Fprintln(os.Stderr, "Usage: symroom member role <member_id> <role>")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			fmt.Printf("Updated role for %s to %s\n", os.Args[3], os.Args[4])
			os.Exit(int(exitcodes.ExitOK))
		default:
			fmt.Fprintf(os.Stderr, "Unknown member action: %s\n", sub)
			os.Exit(int(exitcodes.ExitNoInput))
		}
	case "note":
		fs := flag.NewFlagSet("note", flag.ExitOnError)
		idFlag := fs.String("identity", "", "Author identity name")
		jsonFlag := fs.Bool("json", false, "Output event as JSON")
		if err := fs.Parse(os.Args[2:]); err != nil {
			os.Exit(int(exitcodes.ExitNoInput))
		}
		if fs.NArg() < 1 {
			fmt.Println("Usage: symroom note <message> [--identity <name>] [--json]")
			os.Exit(int(exitcodes.ExitOK))
		}
		msg := fs.Arg(0)
		idName := *idFlag
		if idName == "" {
			cfg := config.LoadOrExit()
			idName = cfg.DefaultIdentity
		}
		if idName == "" {
			fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
			os.Exit(int(exitcodes.ExitNoInput))
		}
		id, err := identity.Load(idName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
			os.Exit(int(exitcodes.ExitNotFound))
		}
		ev, err := room.PostNote(".", msg, id)
		if err != nil {
			if errors.Is(err, members.ErrObserverForbidden) {
				fmt.Fprintln(os.Stderr, "Error: observer role has read-only access")
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Fprintf(os.Stderr, "Error posting note: %v\n", err)
			os.Exit(int(exitcodes.ExitGeneric))
		}
		if *jsonFlag {
			data, _ := ev.MarshalJSONLine()
			fmt.Print(string(data))
		} else {
			fmt.Println(ev.ID)
		}
		os.Exit(int(exitcodes.ExitOK))
	case "decide":
		fs := flag.NewFlagSet("decide", flag.ExitOnError)
		idFlag := fs.String("identity", "", "Author identity name")
		refsFlag := fs.String("refs", "", "Comma-separated reference IDs")
		jsonFlag := fs.Bool("json", false, "Output event as JSON")
		if err := fs.Parse(os.Args[2:]); err != nil {
			os.Exit(int(exitcodes.ExitNoInput))
		}
		if fs.NArg() < 1 {
			fmt.Println("Usage: symroom decide <decision> [--refs ref1,ref2] [--identity <name>] [--json]")
			os.Exit(int(exitcodes.ExitOK))
		}
		msg := fs.Arg(0)
		idName := *idFlag
		if idName == "" {
			cfg := config.LoadOrExit()
			idName = cfg.DefaultIdentity
		}
		if idName == "" {
			fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
			os.Exit(int(exitcodes.ExitNoInput))
		}
		id, err := identity.Load(idName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
			os.Exit(int(exitcodes.ExitNotFound))
		}
		var refs []string
		if *refsFlag != "" {
			refs = strings.Split(*refsFlag, ",")
		}
		ev, err := room.RecordDecision(".", msg, refs, id)
		if err != nil {
			if errors.Is(err, members.ErrObserverForbidden) {
				fmt.Fprintln(os.Stderr, "Error: observer role has read-only access")
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Fprintf(os.Stderr, "Error recording decision: %v\n", err)
			os.Exit(int(exitcodes.ExitGeneric))
		}
		if *jsonFlag {
			data, _ := ev.MarshalJSONLine()
			fmt.Print(string(data))
		} else {
			fmt.Println(ev.ID)
		}
		os.Exit(int(exitcodes.ExitOK))
	case "verify":
		fs := flag.NewFlagSet("verify", flag.ExitOnError)
		jsonFlag := fs.Bool("json", false, "Output verification findings as JSON")
		if err := fs.Parse(os.Args[2:]); err != nil {
			os.Exit(int(exitcodes.ExitNoInput))
		}

		j := journal.New("journal")
		report, err := j.Verify()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error verifying journal: %v\n", err)
			os.Exit(int(exitcodes.ExitGeneric))
		}

		if *jsonFlag {
			data, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(data))
		} else {
			if report.Valid {
				fmt.Println("Journal verification PASSED: zero findings")
			} else {
				fmt.Printf("Journal verification FAILED: %d finding(s):\n", len(report.Findings))
				for _, f := range report.Findings {
					fmt.Printf("  - [%s] %s (event: %s, author: %s)\n", f.Code, f.Message, f.EventID, f.Author)
				}
			}
		}

		if !report.Valid {
			os.Exit(int(exitcodes.ExitGeneric))
		}
		os.Exit(int(exitcodes.ExitOK))
	case "log":
		fs := flag.NewFlagSet("log", flag.ExitOnError)
		sinceFlag := fs.String("since", "", "Filter events since RFC3339 timestamp")
		untilFlag := fs.String("until", "", "Filter events until RFC3339 timestamp")
		kindFlag := fs.String("kind", "", "Filter events by kind")
		authorFlag := fs.String("author", "", "Filter events by author member ID")
		runFlag := fs.String("run", "", "Filter events by run ID")
		limitFlag := fs.Int("limit", 0, "Limit number of events returned")
		jsonFlag := fs.Bool("json", false, "Output events as NDJSON")
		if err := fs.Parse(os.Args[2:]); err != nil {
			os.Exit(int(exitcodes.ExitNoInput))
		}

		j := journal.New("journal")
		res, err := j.QueryLog(journal.LogFilter{
			Since:  *sinceFlag,
			Until:  *untilFlag,
			Kind:   *kindFlag,
			Author: *authorFlag,
			Run:    *runFlag,
			Limit:  *limitFlag,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error querying log: %v\n", err)
			os.Exit(int(exitcodes.ExitGeneric))
		}

		journal.PrintLogWarnings(res.InvalidCount)

		if *jsonFlag {
			for _, ev := range res.Events {
				line, _ := ev.MarshalJSONLine()
				fmt.Print(string(line))
			}
		} else {
			for _, ev := range res.Events {
				fmt.Println(journal.FormatEventHuman(ev))
			}
		}
		os.Exit(int(exitcodes.ExitOK))
	case "index":
		if len(os.Args) < 3 || os.Args[2] != "rebuild" {
			fmt.Println("Usage: symroom index rebuild")
			os.Exit(int(exitcodes.ExitOK))
		}

		j := journal.New("journal")
		dbPath := filepath.Join(".symroom", "index.sqlite")
		indexer := index.New(dbPath)

		if err := indexer.Rebuild(j); err != nil {
			fmt.Fprintf(os.Stderr, "Error rebuilding index: %v\n", err)
			os.Exit(int(exitcodes.ExitGeneric))
		}

		fmt.Printf("Rebuilt derived index at %s\n", dbPath)
		os.Exit(int(exitcodes.ExitOK))
	case "artifact":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stdout, "Usage: symroom artifact <link|unlink|list> [flags] [args]")
			os.Exit(int(exitcodes.ExitOK))
		}
		sub := os.Args[2]
		switch sub {
		case "link":
			fs := flag.NewFlagSet("artifact link", flag.ExitOnError)
			titleFlag := fs.String("title", "", "Artifact title")
			idFlag := fs.String("identity", "", "Author identity name")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if fs.NArg() < 1 {
				fmt.Fprintln(os.Stderr, "Usage: symroom artifact link <path> [--title ...] [--identity <name>]")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			filePath := fs.Arg(0)
			idName := *idFlag
			if idName == "" {
				cfg := config.LoadOrExit()
				idName = cfg.DefaultIdentity
			}
			if idName == "" {
				fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			id, err := identity.Load(idName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
				os.Exit(int(exitcodes.ExitNotFound))
			}
			ev, err := artifact.Link(".", "", filePath, *titleFlag, id)
			if err != nil {
				if errors.Is(err, artifact.ErrOutsideRoot) {
					fmt.Fprintln(os.Stderr, "Error: path is outside artifact root")
					os.Exit(int(exitcodes.ExitNoInput))
				}
				fmt.Fprintf(os.Stderr, "Error linking artifact: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Println(ev.ID)
			os.Exit(int(exitcodes.ExitOK))

		case "unlink":
			fs := flag.NewFlagSet("artifact unlink", flag.ExitOnError)
			idFlag := fs.String("identity", "", "Author identity name")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if fs.NArg() < 1 {
				fmt.Fprintln(os.Stderr, "Usage: symroom artifact unlink <artifact_id> [--identity <name>]")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			artID := fs.Arg(0)
			idName := *idFlag
			if idName == "" {
				cfg := config.LoadOrExit()
				idName = cfg.DefaultIdentity
			}
			if idName == "" {
				fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			id, err := identity.Load(idName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
				os.Exit(int(exitcodes.ExitNotFound))
			}
			ev, err := artifact.Unlink(".", artID, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error unlinking artifact: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Println(ev.ID)
			os.Exit(int(exitcodes.ExitOK))

		case "list":
			fs := flag.NewFlagSet("artifact list", flag.ExitOnError)
			jsonFlag := fs.Bool("json", false, "Output artifacts as JSON")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			list, err := artifact.List(".", "")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing artifacts: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			if *jsonFlag {
				data, _ := json.MarshalIndent(list, "", "  ")
				fmt.Println(string(data))
			} else {
				for _, ref := range list {
					fmt.Printf("%s\t%s\t[%s]\t%s\n", ref.ID, ref.Path, ref.Status, ref.Title)
				}
			}
			os.Exit(int(exitcodes.ExitOK))

		default:
			fmt.Fprintf(os.Stderr, "Unknown artifact action: %s\n", sub)
			os.Exit(int(exitcodes.ExitNoInput))
		}
	case "watch":
		fs := flag.NewFlagSet("watch", flag.ExitOnError)
		deskVaultFlag := fs.String("desk", "", "Symdesk vault name to watch")
		idFlag := fs.String("identity", "", "Author identity name")
		if err := fs.Parse(os.Args[2:]); err != nil {
			os.Exit(int(exitcodes.ExitNoInput))
		}

		if *deskVaultFlag == "" {
			fmt.Println("Usage: symroom watch --desk <vault> [--identity <name>]")
			os.Exit(int(exitcodes.ExitOK))
		}

		idName := *idFlag
		if idName == "" {
			cfg := config.LoadOrExit()
			idName = cfg.DefaultIdentity
		}
		if idName == "" {
			fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
			os.Exit(int(exitcodes.ExitNoInput))
		}

		id, err := identity.Load(idName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
			os.Exit(int(exitcodes.ExitNotFound))
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		handler := func(item *desk.EventStreamItem) error {
			return artifact.HandleDeskEvent(".", "", item, id)
		}

		fmt.Printf("Watching symdesk vault %s...\n", *deskVaultFlag)
		if err := desk.WatchDesk(ctx, *deskVaultFlag, handler); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "Watch error: %v\n", err)
			os.Exit(int(exitcodes.ExitGeneric))
		}
		os.Exit(int(exitcodes.ExitOK))

	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stdout, "Usage: symroom run <request|list|show|start|cancel> [flags] [args]")
			os.Exit(int(exitcodes.ExitOK))
		}
		sub := os.Args[2]
		switch sub {
		case "request":
			fs := flag.NewFlagSet("run request", flag.ExitOnError)
			titleFlag := fs.String("title", "", "Run title")
			planFlag := fs.String("plan-file", "", "Plan file path")
			adapterFlag := fs.String("adapter", "", "Adapter name")
			idFlag := fs.String("identity", "", "Author identity name")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if *titleFlag == "" {
				fmt.Fprintln(os.Stderr, "Error: --title is required")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			idName := *idFlag
			if idName == "" {
				cfg := config.LoadOrExit()
				idName = cfg.DefaultIdentity
			}
			if idName == "" {
				fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			id, err := identity.Load(idName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
				os.Exit(int(exitcodes.ExitNotFound))
			}
			ev, err := run.Request(".", *titleFlag, *planFlag, *adapterFlag, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error requesting run: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Println(ev.ID)
			os.Exit(int(exitcodes.ExitOK))

		case "list":
			fs := flag.NewFlagSet("run list", flag.ExitOnError)
			pendingFlag := fs.Bool("pending", false, "Show pending runs only")
			jsonFlag := fs.Bool("json", false, "Output as JSON")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			runs, err := run.List(".", *pendingFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing runs: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			if *jsonFlag {
				data, _ := json.MarshalIndent(runs, "", "  ")
				fmt.Println(string(data))
			} else {
				for _, r := range runs {
					fmt.Printf("%s\t[%s]\t%s\t%s\n", r.ID, r.State, r.Author, r.Title)
				}
			}
			os.Exit(int(exitcodes.ExitOK))

		case "show":
			fs := flag.NewFlagSet("run show", flag.ExitOnError)
			jsonFlag := fs.Bool("json", false, "Output as JSON")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if fs.NArg() < 1 {
				fmt.Fprintln(os.Stderr, "Usage: symroom run show <run_id> [--json]")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			r, err := run.Get(".", fs.Arg(0))
			if err != nil {
				if errors.Is(err, run.ErrRunNotFound) {
					fmt.Fprintf(os.Stderr, "Error: run %s not found\n", fs.Arg(0))
					os.Exit(int(exitcodes.ExitNotFound))
				}
				fmt.Fprintf(os.Stderr, "Error showing run: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			if *jsonFlag {
				data, _ := json.MarshalIndent(r, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Run ID:     %s\n", r.ID)
				fmt.Printf("Title:      %s\n", r.Title)
				fmt.Printf("State:      %s\n", r.State)
				fmt.Printf("Author:     %s\n", r.Author)
				fmt.Printf("Created At: %s\n", r.CreatedAt)
				if r.Summary != "" {
					fmt.Printf("Summary:    %s\n", r.Summary)
				}
				if r.Error != "" {
					fmt.Printf("Error:      %s\n", r.Error)
				}
			}
			os.Exit(int(exitcodes.ExitOK))

		case "start":
			fs := flag.NewFlagSet("run start", flag.ExitOnError)
			idFlag := fs.String("identity", "", "Author identity name")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if fs.NArg() < 1 {
				fmt.Fprintln(os.Stderr, "Usage: symroom run start <run_id> [--identity <name>]")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			idName := *idFlag
			if idName == "" {
				cfg := config.LoadOrExit()
				idName = cfg.DefaultIdentity
			}
			if idName == "" {
				fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			id, err := identity.Load(idName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
				os.Exit(int(exitcodes.ExitNotFound))
			}
			ev, err := run.Start(".", fs.Arg(0), id)
			if err != nil {
				if errors.Is(err, run.ErrInvalidTransition) {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(int(exitcodes.ExitNoInput))
				}
				fmt.Fprintf(os.Stderr, "Error starting run: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Println(ev.ID)
			os.Exit(int(exitcodes.ExitOK))

		case "cancel":
			fs := flag.NewFlagSet("run cancel", flag.ExitOnError)
			reasonFlag := fs.String("reason", "", "Reason for cancellation")
			idFlag := fs.String("identity", "", "Author identity name")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if fs.NArg() < 1 {
				fmt.Fprintln(os.Stderr, "Usage: symroom run cancel <run_id> [--reason ...] [--identity <name>]")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			idName := *idFlag
			if idName == "" {
				cfg := config.LoadOrExit()
				idName = cfg.DefaultIdentity
			}
			if idName == "" {
				fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			id, err := identity.Load(idName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
				os.Exit(int(exitcodes.ExitNotFound))
			}
			ev, err := run.Cancel(".", fs.Arg(0), *reasonFlag, id)
			if err != nil {
				if errors.Is(err, run.ErrInvalidTransition) {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(int(exitcodes.ExitNoInput))
				}
				fmt.Fprintf(os.Stderr, "Error cancelling run: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Println(ev.ID)
			os.Exit(int(exitcodes.ExitOK))

		case "wait":
			fs := flag.NewFlagSet("run wait", flag.ExitOnError)
			timeoutFlag := fs.Duration("timeout", 15*time.Minute, "Timeout duration")
			jsonFlag := fs.Bool("json", false, "Output as JSON")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if fs.NArg() < 1 {
				fmt.Fprintln(os.Stderr, "Usage: symroom run wait <run_id> [--timeout 15m] [--json]")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			runID := fs.Arg(0)
			ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
			defer cancel()

			r, err := run.Wait(ctx, ".", runID, 500*time.Millisecond)
			if err != nil {
				if errors.Is(err, run.ErrWaitTimeout) {
					fmt.Fprintf(os.Stderr, "Error: wait timed out for run %s\n", runID)
					os.Exit(11)
				}
				if errors.Is(err, run.ErrRunDenied) {
					fmt.Fprintf(os.Stderr, "Error: run %s was denied\n", runID)
					os.Exit(10)
				}
				fmt.Fprintf(os.Stderr, "Error waiting for run %s: %v\n", runID, err)
				os.Exit(int(exitcodes.ExitGeneric))
			}

			if *jsonFlag {
				data, _ := json.MarshalIndent(r, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Run %s approved [%s]\n", r.ID, r.Scope)
			}
			os.Exit(int(exitcodes.ExitOK))

		case "approve":
			fs := flag.NewFlagSet("run approve", flag.ExitOnError)
			scopeFlag := fs.String("scope", "all", "Approval scope")
			ttlFlag := fs.Duration("ttl", 30*time.Minute, "Approval TTL duration")
			idFlag := fs.String("identity", "", "Author identity name")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if fs.NArg() < 1 {
				fmt.Fprintln(os.Stderr, "Usage: symroom run approve <run_id> [--scope ...] [--ttl 30m] [--identity <name>]")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			idName := *idFlag
			if idName == "" {
				cfg := config.LoadOrExit()
				idName = cfg.DefaultIdentity
			}
			if idName == "" {
				fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			id, err := identity.Load(idName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
				os.Exit(int(exitcodes.ExitNotFound))
			}
			ev, err := approval.Approve(".", fs.Arg(0), *scopeFlag, *ttlFlag, id)
			if err != nil {
				if errors.Is(err, approval.ErrAgentApprovalForbidden) {
					fmt.Fprintln(os.Stderr, "Error: agent identity is forbidden from approving runs")
					os.Exit(int(exitcodes.ExitNoInput))
				}
				fmt.Fprintf(os.Stderr, "Error approving run: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Println(ev.ID)
			os.Exit(int(exitcodes.ExitOK))

		case "deny":
			fs := flag.NewFlagSet("run deny", flag.ExitOnError)
			reasonFlag := fs.String("reason", "", "Reason for denial")
			idFlag := fs.String("identity", "", "Author identity name")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if fs.NArg() < 1 || *reasonFlag == "" {
				fmt.Fprintln(os.Stderr, "Usage: symroom run deny <run_id> --reason ... [--identity <name>]")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			idName := *idFlag
			if idName == "" {
				cfg := config.LoadOrExit()
				idName = cfg.DefaultIdentity
			}
			if idName == "" {
				fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			id, err := identity.Load(idName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
				os.Exit(int(exitcodes.ExitNotFound))
			}
			ev, err := approval.Deny(".", fs.Arg(0), *reasonFlag, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error denying run: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Println(ev.ID)
			os.Exit(int(exitcodes.ExitOK))

		default:
			fmt.Fprintf(os.Stderr, "Unknown run action: %s\n", sub)
			os.Exit(int(exitcodes.ExitNoInput))
		}

	case "checkpoint":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stdout, "Usage: symroom checkpoint <request|resolve> [flags] [args]")
			os.Exit(int(exitcodes.ExitOK))
		}
		sub := os.Args[2]
		switch sub {
		case "request":
			fs := flag.NewFlagSet("checkpoint request", flag.ExitOnError)
			runFlag := fs.String("run", "", "Run ID")
			qFlag := fs.String("question", "", "Question string")
			timeoutFlag := fs.Duration("timeout", 15*time.Minute, "Wait timeout duration")
			idFlag := fs.String("identity", "", "Author identity name")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if *runFlag == "" || *qFlag == "" {
				fmt.Fprintln(os.Stderr, "Usage: symroom checkpoint request --run <id> --question \"...\" [--identity <name>]")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			idName := *idFlag
			if idName == "" {
				cfg := config.LoadOrExit()
				idName = cfg.DefaultIdentity
			}
			if idName == "" {
				fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			id, err := identity.Load(idName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
				os.Exit(int(exitcodes.ExitNotFound))
			}
			ev, err := run.RequestCheckpoint(".", *runFlag, *qFlag, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error requesting checkpoint: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			var b struct {
				CheckpointID string `json:"checkpoint_id"`
			}
			_ = json.Unmarshal(ev.Body, &b)

			ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
			defer cancel()

			chk, err := run.WaitCheckpoint(ctx, ".", b.CheckpointID, 500*time.Millisecond)
			if err != nil {
				if errors.Is(err, run.ErrWaitTimeout) {
					fmt.Fprintf(os.Stderr, "Error: wait timed out for checkpoint %s\n", b.CheckpointID)
					os.Exit(11)
				}
				fmt.Fprintf(os.Stderr, "Error waiting for checkpoint: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Println(chk.Answer)
			os.Exit(int(exitcodes.ExitOK))

		case "resolve":
			fs := flag.NewFlagSet("checkpoint resolve", flag.ExitOnError)
			answerFlag := fs.String("answer", "", "Answer string")
			idFlag := fs.String("identity", "", "Author identity name")
			if err := fs.Parse(os.Args[3:]); err != nil {
				os.Exit(int(exitcodes.ExitNoInput))
			}
			if fs.NArg() < 1 || *answerFlag == "" {
				fmt.Fprintln(os.Stderr, "Usage: symroom checkpoint resolve <checkpoint_id> --answer \"...\" [--identity <name>]")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			chkID := fs.Arg(0)
			idName := *idFlag
			if idName == "" {
				cfg := config.LoadOrExit()
				idName = cfg.DefaultIdentity
			}
			if idName == "" {
				fmt.Fprintln(os.Stderr, "Error: --identity is required when default_identity is not configured")
				os.Exit(int(exitcodes.ExitNoInput))
			}
			id, err := identity.Load(idName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading identity %s: %v\n", idName, err)
				os.Exit(int(exitcodes.ExitNotFound))
			}
			ev, err := run.ResolveCheckpoint(".", chkID, *answerFlag, id)
			if err != nil {
				if errors.Is(err, run.ErrAgentCheckpointResolveForbidden) {
					fmt.Fprintln(os.Stderr, "Error: agent identity is forbidden from resolving checkpoints")
					os.Exit(int(exitcodes.ExitNoInput))
				}
				fmt.Fprintf(os.Stderr, "Error resolving checkpoint: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Println(ev.ID)
			os.Exit(int(exitcodes.ExitOK))

		default:
			fmt.Fprintf(os.Stderr, "Unknown checkpoint action: %s\n", sub)
			os.Exit(int(exitcodes.ExitNoInput))
		}

	case "brain-profile":
		fs := flag.NewFlagSet("brain-profile", flag.ExitOnError)
		memberFlag := fs.String("member", "", "Member ID for the agent")
		installFlag := fs.Bool("install", false, "Install profile to symbrain config path")
		if err := fs.Parse(os.Args[2:]); err != nil {
			os.Exit(int(exitcodes.ExitNoInput))
		}

		if *memberFlag == "" {
			fmt.Fprintln(os.Stderr, "Usage: symroom brain-profile --member <id> [--install]")
			os.Exit(int(exitcodes.ExitNoInput))
		}

		content, prof, err := brainprofile.Generate(".", *memberFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating brain profile: %v\n", err)
			os.Exit(int(exitcodes.ExitGeneric))
		}

		if *installFlag {
			msg, err := brainprofile.Install(prof.Name, content)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error installing profile: %v\n", err)
				os.Exit(int(exitcodes.ExitGeneric))
			}
			fmt.Println(msg)
		} else {
			fmt.Println(content)
			fmt.Printf("# To install run:\n# symbrain install --harness <harness> --profile %s\n", prof.Name)
		}
		os.Exit(int(exitcodes.ExitOK))

	case "doctor":
		fs := flag.NewFlagSet("doctor", flag.ExitOnError)
		jsonFlag := fs.Bool("json", false, "Emit stable machine-readable JSON")
		if err := fs.Parse(os.Args[2:]); err != nil {
			os.Exit(int(exitcodes.ExitNoInput))
		}
		report, err := doctor.Run(".")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running doctor: %v\n", err)
			os.Exit(int(exitcodes.ExitGeneric))
		}
		if *jsonFlag {
			data, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(data))
		} else {
			for _, c := range report.Checks {
				fmt.Printf("[%s] %s: %s\n  remediation: %s\n", strings.ToUpper(string(c.Status)), c.Name, c.Message, c.Remediation)
			}
			for _, t := range report.Tools {
				fmt.Printf("[%s] %s: %s", strings.ToUpper(string(t.Status)), t.Name, t.Path)
				if t.Version != "" {
					fmt.Printf(" (%s)", t.Version)
				}
				fmt.Printf("\n  remediation: %s\n", t.Remediation)
			}
		}
		if report.Failed {
			os.Exit(int(exitcodes.ExitGeneric))
		}
		os.Exit(int(exitcodes.ExitOK))
	case "mcp":
		fs := flag.NewFlagSet("mcp", flag.ExitOnError)
		roomDir := fs.String("room", ".", "Room directory")
		identityName := fs.String("identity", "", "Signing identity name")
		artifactRoot := fs.String("artifact-root", "", "Artifact root directory")
		if err := fs.Parse(os.Args[2:]); err != nil {
			os.Exit(int(exitcodes.ExitNoInput))
		}
		name := *identityName
		if name == "" {
			cfg := config.LoadOrExit()
			name = cfg.DefaultIdentity
		}
		if name == "" {
			fmt.Fprintln(os.Stderr, "Error: --identity is required")
			os.Exit(int(exitcodes.ExitNoInput))
		}
		id, err := identity.Load(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading identity: %v\n", err)
			os.Exit(int(exitcodes.ExitNotFound))
		}
		if err := mcp.NewServer(*roomDir, id, *artifactRoot).ServeStdio(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(int(exitcodes.ExitGeneric))
		}
		os.Exit(int(exitcodes.ExitOK))
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, usageText)
		os.Exit(int(exitcodes.ExitOK))
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n%s", subcommand, usageText)
		os.Exit(int(exitcodes.ExitNoInput))
	}
}
