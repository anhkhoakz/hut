package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
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
		name, owner, instance := parseResourceName(args[0])
		c := createClientWithInstance("todo", cmd, instance)
		id := getTrackerID(c, ctx, name, owner)

		if !autoConfirm && !getConfirmation(fmt.Sprintf("Do you really want to delete the tracker %s", name)) {
			fmt.Println("Aborted")
			return
		}

		tracker, err := todosrht.DeleteTracker(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if tracker == nil {
			log.Fatalf("failed to delete tracker %q", name)
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
	cmd.AddCommand(newTodoTicketCommentCommand())
	cmd.AddCommand(newTodoTicketStatusCommand())
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
					labels += label.TermString()
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

func newTodoTicketCommentCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance := parseTicketResource(ctx, cmd, args[0])

		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		var input todosrht.SubmitCommentInput
		fmt.Printf("Comment %s:\n", termfmt.Dim.String("(Markdown supported)"))
		text, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("failed to read comment: %v", err)
		}
		input.Text = string(text)

		event, err := todosrht.SubmitComment(c.Client, ctx, trackerID, ticketID, input)
		if err != nil {
			log.Fatal(err)
		} else if event == nil {
			log.Fatalf("failed to comment on ticket with ID %d", ticketID)
		}

		fmt.Printf("Commented on %s\n", event.Ticket.Subject)
	}

	cmd := &cobra.Command{
		Use:   "comment <ID>",
		Short: "Comment on a ticket",
		Args:  cobra.ExactArgs(1),
		Run:   run,
	}
	return cmd
}

func newTodoTicketStatusCommand() *cobra.Command {
	var status, resolution string
	run := func(cmd *cobra.Command, args []string) {
		var input todosrht.UpdateStatusInput
		ctx := cmd.Context()

		ticketStatus, err := todosrht.ParseTicketStatus(status)
		if err != nil {
			log.Fatal(err)
		}
		input.Status = ticketStatus

		if ticketStatus == todosrht.TicketStatusResolved {
			ticketResolution, err := todosrht.ParseTicketResolution(resolution)
			if err != nil {
				log.Fatal(err)
			}
			input.Resolution = &ticketResolution
		} else if resolution != "" {
			log.Fatalf("resolution %q specified, but ticket not marked as resolved", resolution)
		}

		ticketID, name, owner, instance := parseTicketResource(ctx, cmd, args[0])
		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		event, err := todosrht.UpdateTicketStatus(c.Client, ctx, trackerID, ticketID, input)
		if err != nil {
			log.Fatal(err)
		} else if event == nil {
			log.Fatalf("failed to update status of ticket with ID %d", ticketID)
		}

		fmt.Printf("Updated status of %s\n", event.Ticket.Subject)
	}

	cmd := &cobra.Command{
		Use:               "update-status <ID>",
		Short:             "Update ticket status",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringVarP(&status, "status", "s", "", "ticket status")
	cmd.RegisterFlagCompletionFunc("status", completeTicketStatus)
	cmd.MarkFlagRequired("status")
	cmd.Flags().StringVarP(&resolution, "resolution", "r", "", "ticket resolution")
	cmd.RegisterFlagCompletionFunc("resolution", completeTicketResolution)
	return cmd
}

func getTrackerID(c *Client, ctx context.Context, name, owner string) int32 {
	var (
		tracker *todosrht.Tracker
		err     error
	)

	if owner == "" {
		tracker, err = todosrht.TrackerIDByName(c.Client, ctx, name)
	} else {
		tracker, err = todosrht.TrackerIDByOwner(c.Client, ctx, owner, name)
	}
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

func parseTicketResource(ctx context.Context, cmd *cobra.Command, ticket string) (ticketID int32, name, owner, instance string) {
	if strings.Contains(ticket, "/") {
		var resource string
		resource, owner, instance = parseResourceName(ticket)
		split := strings.Split(resource, "/")
		if len(split) != 2 {
			log.Fatal("failed to parse tracker name and/or ID")
		}

		name = split[0]
		var err error
		ticketID, err = parseInt32(split[1])
		if err != nil {
			log.Fatal(err)
		}
	} else {
		var err error
		ticketID, err = parseInt32(ticket)
		if err != nil {
			log.Fatal(err)
		}
		name, owner, instance = getTrackerName(ctx, cmd)
	}

	return ticketID, name, owner, instance
}

func completeTicketStatus(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"reported", "confirmed", "in_progress", "pending", "resolved"},
		cobra.ShellCompDirectiveNoFileComp
}

func completeTicketResolution(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"unresolved", "fixed", "implemented", "wont_fix", "by_design",
		"invalid", "duplicate", "not_our_bug"}, cobra.ShellCompDirectiveNoFileComp
}
