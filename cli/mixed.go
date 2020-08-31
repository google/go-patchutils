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

func init() {
	subcommands.Register(&mixedCmd{}, "")
}

func (*mixedCmd) Name() string { return "mixed" }
func (*mixedCmd) Synopsis() string {
	return "compute difference between " +
		"oldSource patched with oldDiff and newSource patched with newDiff."
}
func (*mixedCmd) Usage() string {
	return "mixed -oldsource=<oldSource path> -olddiff=<oldDiff path> -newsource=<newSource path> -newdiff=<newDiff path>: " +
		"Compute difference between oldSource patched with oldDiff and newSource patched with newDiff.\n"
}

func (c *mixedCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.oldSource, "oldsource", "", "path to the old version of source")
	f.StringVar(&c.oldDiff, "olddiff", "", "path to the old version of diff")
	f.StringVar(&c.newSource, "newsource", "", "path to the new version of source")
	f.StringVar(&c.newDiff, "newdiff", "", "path to the new version of diff")
}

func (c *mixedCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if (c.oldSource == "") || (c.oldDiff == "") || (c.newSource == "") || (c.newDiff == "") {
		glog.Errorf("Error: necessary flags aren't assigned")
		glog.Infof("Usage: %s %s", os.Args[0], c.Usage())
		return subcommands.ExitUsageError
	}

	oldD, err := os.Open(c.oldDiff)
	if err != nil {
		glog.Errorf("Failed to open oldDiffFile %q\n", c.oldDiff)
		return subcommands.ExitFailure
	}
	defer oldD.Close()

	newD, err := os.Open(c.newDiff)
	if err != nil {
		glog.Errorf("Failed to open newDiffFile %q\n", c.newDiff)
		return subcommands.ExitFailure
	}
	defer newD.Close()

	result, err := patchutils.MixedModePath(c.oldSource, c.newSource, oldD, newD)
	if err != nil {
		glog.Errorf("Error during computing diff for (%q + %q) and (%q + %q): %v\n",
			c.oldSource, c.oldDiff, c.newSource, c.newDiff, err)
		return subcommands.ExitFailure
	}

	fmt.Println(result)
	return subcommands.ExitSuccess
}
