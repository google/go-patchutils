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

type interdiffCmd struct {
	oldDiff string
	newDiff string
}

func init() {
	subcommands.Register(&interdiffCmd{}, "")
}

func (*interdiffCmd) Name() string { return "interdiff" }
func (*interdiffCmd) Synopsis() string {
	return "compute difference between " +
		"source patched with oldDiff and same source patched with newDiff."
}
func (*interdiffCmd) Usage() string {
	return "interdiff -olddiff=<oldDiff path> -newdiff=<newDiff path>: " +
		"Compute difference between source patched with oldDiff and same source patched with newDiff.\n"
}

func (c *interdiffCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.oldDiff, "olddiff", "", "path to the old version of diff")
	f.StringVar(&c.newDiff, "newdiff", "", "path to the new version of diff")
}

func (c *interdiffCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if (c.oldDiff == "") || (c.newDiff == "") {
		glog.Error("Error: necessary flags aren't assigned")
		glog.Infof("Usage: %s %s", os.Args[0], c.Usage())
		return subcommands.ExitUsageError
	}

	oldD, err := os.Open(c.oldDiff)
	if err != nil {
		glog.Errorf("Failed to open oldDiffFile: %q\n", c.oldDiff)
		return subcommands.ExitFailure
	}
	defer oldD.Close()

	newD, err := os.Open(c.newDiff)
	if err != nil {
		glog.Errorf("Failed to open newDiffFile %q\n", c.newDiff)
		return subcommands.ExitFailure
	}
	defer newD.Close()

	result, err := patchutils.InterDiff(oldD, newD)
	if err != nil {
		glog.Errorf("Error during computing diff for %q and %q: %v\n", c.oldDiff, c.newDiff, err)
		return subcommands.ExitFailure
	}

	fmt.Println(result)
	return subcommands.ExitSuccess
}
