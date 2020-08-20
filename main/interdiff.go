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
		"Compute difference between source patched with oldDiff and same source patched with newDiff."
}

func (i *interdiffCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&i.oldDiff, "olddiff", "", "oldDiff")
	f.StringVar(&i.newDiff, "newdiff", "", "newDiff")
}

func (i *interdiffCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if (i.oldDiff == "") || (i.newDiff == "") {
		glog.Error("Error: necessary flags aren't assigned")
		glog.Infof("Usage: %s %s", os.Args[0], i.Usage())
		return subcommands.ExitUsageError
	}

	oldD, err := os.Open(i.oldDiff)
	if err != nil {
		glog.Errorf("Failed to open oldDiffFile: %q\n", i.oldDiff)
		return subcommands.ExitFailure
	}

	newD, err := os.Open(i.newDiff)
	if err != nil {
		glog.Errorf("Failed to open newDiffFile %q\n", i.newDiff)
		return subcommands.ExitFailure
	}

	result, err := patchutils.InterDiff(oldD, newD)
	if err != nil {
		glog.Errorf("Error during computing diff for %q and %q: %v\n", i.oldDiff, i.newDiff, err)
		return subcommands.ExitFailure
	}

	fmt.Println(result)
	return subcommands.ExitSuccess
}
