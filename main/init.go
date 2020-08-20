package main

import "github.com/google/subcommands"

func init() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&interdiffCmd{}, "")
	subcommands.Register(&mixedCmd{}, "")
}
