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
	cmd.AddCommand(newTodoTicketCommand())
	cmd.PersistentFlags().StringP("tracker", "t", "", "name of tracker")
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

func newTodoTicketCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ticket",
		Short: "Manage tickets",
	}
	cmd.AddCommand(newTodoTicketListCommand())
	return cmd
}

func newTodoTicketListCommand() *cobra.Command {
	// TODO: Filter by ticket status
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		name, owner, instance := getTrackerName(ctx, cmd)
		c := createClientWithInstance("todo", cmd, instance)

		var (
			tracker *todosrht.Tracker
			err     error
		)

		if owner != "" {
			tracker, err = todosrht.TicketsByOwner(c.Client, ctx, owner, name)
		} else {
			tracker, err = todosrht.Tickets(c.Client, ctx, name)
		}

		if err != nil {
			log.Fatal(err)
		} else if tracker == nil {
			log.Fatalf("no such tracker %q", name)
		}

		for _, ticket := range tracker.Tickets.Results {
			var labels string
			s := termfmt.DarkYellow.Sprintf("#%d %s ", ticket.Id, ticket.Status.TermString())
			if ticket.Status == todosrht.TicketStatusResolved {
				s += termfmt.Green.Sprintf("%s ", strings.ToLower(string(ticket.Resolution)))
			}

			if len(ticket.Labels) > 0 {
				labels = " ["
				for i, label := range ticket.Labels {
					labels += label.Name
					if i != len(ticket.Labels)-1 {
						labels += ", "
					}
				}
				labels += "]"
			}
			s += fmt.Sprintf("%s%s (%s %s ago)", ticket.Subject, labels,
				ticket.Submitter.CanonicalName, timeDelta(ticket.Created))
			fmt.Println(s)
		}

	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tickets",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
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

func getTrackerName(ctx context.Context, cmd *cobra.Command) (name, owner, instance string) {
	if s, err := cmd.Flags().GetString("tracker"); err != nil {
		log.Fatal(err)
	} else if s != "" {
		return parseResourceName(s)
	}

	// TODO: Use hub.sr.ht API to determine trackers
	name, owner, instance, err := guessGitRepoName(ctx)
	if err != nil {
		log.Fatal(err)
	}

	return name, owner, instance
}
