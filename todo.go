package main

import (
	"fmt"
	"log"
	"strings"

	"git.sr.ht/~emersion/hut/srht/todosrht"
	"git.sr.ht/~emersion/hut/termfmt"
	"github.com/spf13/cobra"
)

func newTodoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todo",
		Short: "Use the todo API",
	}
	cmd.AddCommand(newTodoListCommand())
	return cmd
}

func newTodoListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("todo", cmd)

		var trackers *todosrht.TrackerCursor

		if len(args) > 0 {
			username := strings.TrimLeft(args[0], ownerPrefixes)
			user, err := todosrht.TrackersByUser(c.Client, ctx, username)
			if err != nil {
				log.Fatal(err)
			} else if user == nil {
				log.Fatal("no such user")
			}
			trackers = user.Trackers
		} else {
			var err error
			trackers, err = todosrht.Trackers(c.Client, ctx)
			if err != nil {
				log.Fatal(err)
			}
		}

		for _, tracker := range trackers.Results {
			fmt.Printf("%s (%s)\n", termfmt.Bold.String(tracker.Name), tracker.Visibility.TermString())
			if tracker.Description != nil && *tracker.Description != "" {
				fmt.Println(indent(strings.TrimSpace(*tracker.Description), "  "))
			}
			fmt.Println()
		}
	}

	cmd := &cobra.Command{
		Use:               "list [user]",
		Short:             "List trackers",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}
