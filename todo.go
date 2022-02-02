package main

import (
	"context"
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
	cmd.AddCommand(newTodoDeleteCommand())
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

func newTodoDeleteCommand() *cobra.Command {
	var autoConfirm bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		name, _, instance := parseResourceName(args[0])
		c := createClientWithInstance("todo", cmd, instance)
		id := getTrackerID(c, ctx, name)

		if !autoConfirm && !getConfirmation(fmt.Sprintf("Do you really want to delete the tracker %s", name)) {
			fmt.Println("Aborted")
			return
		}

		tracker, err := todosrht.DeleteTracker(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Deleted tracker %s\n", tracker.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete <tracker>",
		Short:             "Delete a tracker",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeRepo,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "auto confirm")
	return cmd
}

func getTrackerID(c *Client, ctx context.Context, name string) int32 {
	tracker, err := todosrht.TrackerIDByName(c.Client, ctx, name)
	if err != nil {
		log.Fatalf("failed to get tracker ID: %v", err)
	} else if tracker == nil {
		log.Fatalf("tracker %q does not exist", name)
	}
	return tracker.Id
}
