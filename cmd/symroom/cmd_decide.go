package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/danieljustus/symaira-corekit/exitcodes"
	"github.com/danieljustus/symaira-room/internal/members"
	"github.com/danieljustus/symaira-room/internal/room"
)

// runDecide implements the "decide" subcommand.
func runDecide(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("decide", flag.ExitOnError)
	idFlag := fs.String("identity", "", "Author identity name")
	refsFlag := fs.String("refs", "", "Comma-separated reference IDs")
	jsonFlag := fs.Bool("json", false, "Output event as JSON")
	if err := fs.Parse(args[2:]); err != nil {
		return int(exitcodes.ExitNoInput)
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stdout, "Usage: symroom decide <decision> [--refs ref1,ref2] [--identity <name>] [--json]")
		return int(exitcodes.ExitOK)
	}
	msg := fs.Arg(0)
	id := resolveIdentity(*idFlag)
	var refs []string
	if *refsFlag != "" {
		refs = strings.Split(*refsFlag, ",")
	}
	ev, err := room.RecordDecision(roomDir(), msg, refs, id)
	if err != nil {
		if errors.Is(err, members.ErrObserverForbidden) {
			fmt.Fprintln(stderr, "Error: observer role has read-only access")
			return int(exitcodes.ExitGeneric)
		}
		fmt.Fprintf(stderr, "Error recording decision: %v\n", err)
		return int(exitcodes.ExitGeneric)
	}
	if *jsonFlag {
		data, _ := ev.MarshalJSONLine()
		fmt.Fprint(stdout, string(data))
	} else {
		fmt.Fprintln(stdout, ev.ID)
	}
	return int(exitcodes.ExitOK)
}
