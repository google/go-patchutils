package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/google/go-patchutils"
	"github.com/google/subcommands"
)

type mixedCmd struct {
	oldSource string
	oldDiff   string
	newSource string
	newDiff   string
}

func (*mixedCmd) Name() string { return "mixed" }
func (*mixedCmd) Synopsis() string {
	return "compute difference between " +
		"oldSource patched with oldDiff and newSource patched with newDiff."
}
func (*mixedCmd) Usage() string {
	return "mixed -oldsource=<oldSource path> -olddiff=<oldDiff path> -newsource=<newSource path> -newdiff=<newDiff path>: " +
		"Compute difference between oldSource patched with oldDiff and newSource patched with newDiff."
}

func (m *mixedCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&m.oldSource, "oldsource", "", "oldSource")
	f.StringVar(&m.oldDiff, "olddiff", "", "oldDiff")
	f.StringVar(&m.newSource, "newsource", "", "newSource")
	f.StringVar(&m.newDiff, "newdiff", "", "newDiff")
}

func (m *mixedCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if (m.oldSource == "") || (m.oldDiff == "") || (m.newSource == "") || (m.newDiff == "") {
		glog.Errorf("Error: necessary flags aren't assigned")
		glog.Infof("Usage: %s %s", os.Args[0], m.Usage())
		return subcommands.ExitUsageError
	}

	oldD, err := os.Open(m.oldDiff)
	if err != nil {
		glog.Errorf("Failed to open oldDiffFile %q\n", m.oldDiff)
		return subcommands.ExitFailure
	}

	newD, err := os.Open(m.newDiff)
	if err != nil {
		glog.Errorf("Failed to open newDiffFile %q\n", m.newDiff)
		return subcommands.ExitFailure
	}

	result, err := patchutils.MixedModePath(m.oldSource, m.newSource, oldD, newD)
	if err != nil {
		glog.Errorf("Error during computing diff for (%q + %q) and (%q + %q): %v\n",
			m.oldSource, m.oldDiff, m.newSource, m.newDiff, err)
		return subcommands.ExitFailure
	}

	fmt.Println(result)
	return subcommands.ExitSuccess
}
