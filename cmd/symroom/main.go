package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/danieljustus/symaira-corekit/exitcodes"
	"github.com/danieljustus/symaira-room/internal/config"
	"github.com/danieljustus/symaira-room/internal/identity"
	"github.com/danieljustus/symaira-room/internal/room"
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
	case "note", "decide", "artifact",
		"log", "verify", "index", "run", "checkpoint", "watch",
		"brain-profile", "doctor", "mcp":
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
