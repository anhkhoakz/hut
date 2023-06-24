package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"

	"git.sr.ht/~emersion/hut/srht/todosrht"
	"git.sr.ht/~emersion/hut/termfmt"
	"github.com/dustin/go-humanize"
	"github.com/juju/ansiterm/tabwriter"
	"github.com/spf13/cobra"
)

func newTodoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todo",
		Short: "Use the todo API",
	}
	cmd.AddCommand(newTodoListCommand())
	cmd.AddCommand(newTodoDeleteCommand())
	cmd.AddCommand(newTodoSubscribeCommand())
	cmd.AddCommand(newTodoUnsubscribeCommand())
	cmd.AddCommand(newTodoCreateCommand())
	cmd.AddCommand(newTodoTicketCommand())
	cmd.AddCommand(newTodoLabelCommand())
	cmd.AddCommand(newTodoACLCommand())
	cmd.AddCommand(newTodoWebhookCommand())
	cmd.AddCommand(newTodoUserWebhookCommand())
	cmd.PersistentFlags().StringP("tracker", "t", "", "name of tracker")
	cmd.RegisterFlagCompletionFunc("tracker", completeTracker)
	return cmd
}

func newTodoListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("todo", cmd)
		var cursor *todosrht.Cursor
		var username string
		if len(args) > 0 {
			username = strings.TrimLeft(args[0], ownerPrefixes)
		}
		pagerify(func(p pager) bool {
			var trackers *todosrht.TrackerCursor
			if len(username) > 0 {
				user, err := todosrht.TrackersByUser(c.Client, ctx, username, cursor)
				if err != nil {
					log.Fatal(err)
				} else if user == nil {
					log.Fatal("no such user")
				}
				trackers = user.Trackers
			} else {
				var err error
				trackers, err = todosrht.Trackers(c.Client, ctx, cursor)
				if err != nil {
					log.Fatal(err)
				}
			}

			for _, tracker := range trackers.Results {
				printTracker(p, &tracker)
			}

			cursor = trackers.Cursor
			return cursor == nil
		})
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

func printTracker(w io.Writer, tracker *todosrht.Tracker) {
	fmt.Fprintf(w, "%s (%s)\n", termfmt.Bold.String(tracker.Name), tracker.Visibility.TermString())
	if tracker.Description != nil && *tracker.Description != "" {
		fmt.Fprintln(w, indent(strings.TrimSpace(*tracker.Description), "  "))
	}
	fmt.Fprintln(w)
}

func newTodoDeleteCommand() *cobra.Command {
	var autoConfirm bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		name, owner, instance := parseResourceName(args[0])
		c := createClientWithInstance("todo", cmd, instance)
		id := getTrackerID(c, ctx, name, owner)

		if !autoConfirm && !getConfirmation(fmt.Sprintf("Do you really want to delete the tracker %s", name)) {
			log.Println("Aborted")
			return
		}

		tracker, err := todosrht.DeleteTracker(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if tracker == nil {
			log.Fatalf("failed to delete tracker %q", name)
		}

		log.Printf("Deleted tracker %s\n", tracker.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete <tracker>",
		Short:             "Delete a tracker",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTracker,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "auto confirm")
	return cmd
}

func newTodoSubscribeCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getTrackerName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}
		c := createClientWithInstance("todo", cmd, instance)
		id := getTrackerID(c, ctx, name, owner)

		subscription, err := todosrht.TrackerSubscribe(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if subscription == nil {
			log.Fatalf("failed to subscribe to tracker %q", name)
		}

		log.Printf("Subscribed to %s/%s/%s\n", c.BaseURL, subscription.Tracker.Owner.CanonicalName, subscription.Tracker.Name)
	}

	cmd := &cobra.Command{
		Use:               "subscribe [tracker]",
		Short:             "Subscribe to a tracker",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newTodoUnsubscribeCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getTrackerName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}
		c := createClientWithInstance("todo", cmd, instance)
		id := getTrackerID(c, ctx, name, owner)

		subscription, err := todosrht.TrackerUnsubscribe(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if subscription == nil {
			log.Fatalf("you were not subscribed to %s/%s/%s", c.BaseURL, owner, name)
		}

		log.Printf("Unsubscribed from %s/%s/%s\n", c.BaseURL, subscription.Tracker.Owner.CanonicalName, subscription.Tracker.Name)
	}

	cmd := &cobra.Command{
		Use:               "unsubscribe [tracker]",
		Short:             "Unubscribe from a tracker",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

const todoCreatePrefill = `
<!--
Please write the Markdown description of the new tracker above.
-->`

func newTodoCreateCommand() *cobra.Command {
	var visibility string
	var stdin bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("todo", cmd)

		todoVisibility, err := todosrht.ParseVisibility(visibility)
		if err != nil {
			log.Fatal(err)
		}

		var description *string

		if stdin {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				log.Fatalf("failed to read tracker description: %v", err)
			}

			desc := string(b)
			description = &desc
		} else {
			text, err := getInputWithEditor("hut_tracker*.md", todoCreatePrefill)
			if err != nil {
				log.Fatalf("failed to read description: %v", err)
			}

			text = dropComment(text, todoCreatePrefill)
			description = &text
		}

		tracker, err := todosrht.CreateTracker(c.Client, ctx, args[0], description, todoVisibility)
		if err != nil {
			log.Fatal(err)
		} else if tracker == nil {
			log.Fatal("failed to create tracker")
		}

		log.Printf("Created tracker %q\n", tracker.Name)
	}
	cmd := &cobra.Command{
		Use:               "create <name>",
		Short:             "Create a tracker",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringVarP(&visibility, "visibility", "v", "public", "tracker visibility")
	cmd.RegisterFlagCompletionFunc("visibility", completeVisibility)
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read tracker from stdin")
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
	cmd.AddCommand(newTodoTicketSubscribeCommand())
	cmd.AddCommand(newTodoTicketUnsubscribeCommand())
	cmd.AddCommand(newTodoTicketAssignCommand())
	cmd.AddCommand(newTodoTicketUnassignCommand())
	cmd.AddCommand(newTodoTicketDeleteCommand())
	cmd.AddCommand(newTodoTicketShowCommand())
	cmd.AddCommand(newTodoTicketWebhookCommand())
	cmd.AddCommand(newTodoTicketCreateCommand())
	cmd.AddCommand(newTodoTicketLabelCommand())
	cmd.AddCommand(newTodoTicketUnlabelCommand())
	return cmd
}

func newTodoTicketListCommand() *cobra.Command {
	// TODO: Filter by ticket status
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		name, owner, instance, err := getTrackerName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		var (
			cursor   *todosrht.Cursor
			user     *todosrht.User
			username string
		)
		if owner != "" {
			username = strings.TrimLeft(owner, ownerPrefixes)
		}

		pagerify(func(p pager) bool {
			if username != "" {
				user, err = todosrht.TicketsByUser(c.Client, ctx, username, name, cursor)
			} else {
				user, err = todosrht.Tickets(c.Client, ctx, name, cursor)
			}

			if err != nil {
				log.Fatal(err)
			} else if user == nil {
				log.Fatalf("no such user %q", username)
			} else if user.Tracker == nil {
				log.Fatalf("no such tracker %q", name)
			}

			for _, ticket := range user.Tracker.Tickets.Results {
				printTicket(p, &ticket)
			}

			cursor = user.Tracker.Tickets.Cursor
			return cursor == nil
		})

	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tickets",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	return cmd
}

func printTicket(w io.Writer, ticket *todosrht.Ticket) {
	var labels string
	s := termfmt.DarkYellow.Sprintf("#%d %s ", ticket.Id, ticket.Status.TermString())
	if ticket.Status == todosrht.TicketStatusResolved && ticket.Resolution != todosrht.TicketResolutionClosed {
		s += termfmt.Green.Sprintf("%s ", strings.ToLower(string(ticket.Resolution)))
	}

	if len(ticket.Labels) > 0 {
		labels = " "
		for i, label := range ticket.Labels {
			labels += label.TermString()
			if i != len(ticket.Labels)-1 {
				labels += " "
			}
		}
	}
	s += fmt.Sprintf("%s%s (%s %s)", ticket.Subject, labels,
		ticket.Submitter.CanonicalName, humanize.Time(ticket.Created.Time))
	fmt.Fprintln(w, s)
}

func newTodoTicketCommentCommand() *cobra.Command {
	var stdin bool
	var status, resolution string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		var input todosrht.SubmitCommentInput

		if resolution != "" {
			ticketResolution, err := todosrht.ParseTicketResolution(resolution)
			if err != nil {
				log.Fatal(err)
			}
			input.Resolution = &ticketResolution

			if status == "" {
				status = "resolved"
			}
		}

		if status != "" {
			ticketStatus, err := todosrht.ParseTicketStatus(status)
			if err != nil {
				log.Fatal(err)
			}
			input.Status = &ticketStatus
		}

		if input.Status != nil {
			if *input.Status != todosrht.TicketStatusResolved && input.Resolution != nil {
				log.Fatalf("resolution %q specified, but ticket not marked as resolved", resolution)
			}
			if *input.Status == todosrht.TicketStatusResolved && input.Resolution == nil {
				log.Fatalf("resolution is required when status is RESOLVED")
			}
		}

		if stdin {
			fmt.Printf("Comment %s:\n", termfmt.Dim.String("(Markdown supported)"))
			text, err := io.ReadAll(os.Stdin)
			if err != nil {
				log.Fatalf("failed to read comment: %v", err)
			}
			input.Text = string(text)
		} else {
			text, err := getInputWithEditor("hut_comment*.md", "")
			if err != nil {
				log.Fatalf("failed to read comment: %v", err)
			}
			input.Text = text
		}

		if strings.TrimSpace(input.Text) == "" {
			log.Println("Aborted writing empty comment")
			return
		}

		event, err := todosrht.SubmitComment(c.Client, ctx, trackerID, ticketID, input)
		if err != nil {
			log.Fatal(err)
		} else if event == nil {
			log.Fatalf("failed to comment on ticket with ID %d", ticketID)
		}

		log.Printf("Commented on %s\n", event.Ticket.Subject)
	}

	cmd := &cobra.Command{
		Use:               "comment <ID>",
		Short:             "Comment on a ticket",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read comment from stdin")
	cmd.Flags().StringVarP(&status, "status", "s", "", "ticket status")
	cmd.RegisterFlagCompletionFunc("status", completeTicketStatus)
	cmd.Flags().StringVarP(&resolution, "resolution", "r", "", "ticket resolution")
	cmd.RegisterFlagCompletionFunc("resolution", completeTicketResolution)
	return cmd
}

func newTodoTicketStatusCommand() *cobra.Command {
	var status, resolution string
	run := func(cmd *cobra.Command, args []string) {
		var input todosrht.UpdateStatusInput
		ctx := cmd.Context()

		if resolution != "" {
			ticketResolution, err := todosrht.ParseTicketResolution(resolution)
			if err != nil {
				log.Fatal(err)
			}
			input.Resolution = &ticketResolution

			if status == "" {
				status = "resolved"
			}
		}

		ticketStatus, err := todosrht.ParseTicketStatus(status)
		if err != nil {
			log.Fatal(err)
		}
		input.Status = ticketStatus

		if ticketStatus != todosrht.TicketStatusResolved && input.Resolution != nil {
			log.Fatalf("resolution %q specified, but ticket not marked as resolved", resolution)
		}

		if ticketStatus == todosrht.TicketStatusResolved && input.Resolution == nil {
			res := todosrht.TicketResolutionClosed
			input.Resolution = &res
		}

		ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		event, err := todosrht.UpdateTicketStatus(c.Client, ctx, trackerID, ticketID, input)
		if err != nil {
			log.Fatal(err)
		} else if event == nil {
			log.Fatalf("failed to update status of ticket with ID %d", ticketID)
		}

		log.Printf("Updated status of %s\n", event.Ticket.Subject)
	}

	cmd := &cobra.Command{
		Use:               "update-status <ID>",
		Short:             "Update ticket status",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	cmd.Flags().StringVarP(&status, "status", "s", "", "ticket status")
	cmd.RegisterFlagCompletionFunc("status", completeTicketStatus)
	cmd.Flags().StringVarP(&resolution, "resolution", "r", "", "ticket resolution")
	cmd.RegisterFlagCompletionFunc("resolution", completeTicketResolution)
	return cmd
}

func newTodoTicketSubscribeCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		subscription, err := todosrht.TicketSubscribe(c.Client, ctx, trackerID, ticketID)
		if err != nil {
			log.Fatal(err)
		} else if subscription == nil {
			log.Fatalf("failed to subscribe to ticket %d", ticketID)
		}

		log.Printf("Subscribed to %s/%s/%s/%d\n", c.BaseURL, owner, name, subscription.Ticket.Id)
	}

	cmd := &cobra.Command{
		Use:               "subscribe <ID>",
		Short:             "Subscribe to a ticket",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	return cmd
}

func newTodoTicketUnsubscribeCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		subscription, err := todosrht.TicketUnsubscribe(c.Client, ctx, trackerID, ticketID)
		if err != nil {
			log.Fatal(err)
		} else if subscription == nil {
			log.Fatalf("you were not subscribed to ticket with ID %d", ticketID)
		}

		log.Printf("Unsubscribed from %s/%s/%s/%d\n", c.BaseURL, owner, name, subscription.Ticket.Id)
	}

	cmd := &cobra.Command{
		Use:               "unsubscribe <ID>",
		Short:             "Unsubscribe from a ticket",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	return cmd
}

func newTodoTicketAssignCommand() *cobra.Command {
	var userName string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		user, err := todosrht.UserIDByName(c.Client, ctx, userName)
		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatalf("no such user %q", userName)
		}

		event, err := todosrht.AssignUser(c.Client, ctx, trackerID, ticketID, user.Id)
		if err != nil {
			log.Fatal(err)
		} else if event == nil {
			log.Fatal("failed to assign user")
		}

		log.Printf("Assigned %q to %q\n", userName, event.Ticket.Subject)
	}

	cmd := &cobra.Command{
		Use:               "assign <ID>",
		Short:             "Assign a user to a ticket",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	cmd.Flags().StringVarP(&userName, "user", "u", "", "username")
	cmd.MarkFlagRequired("user")
	cmd.RegisterFlagCompletionFunc("user", completeTicketAssign)
	return cmd
}

func newTodoTicketUnassignCommand() *cobra.Command {
	var userName string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		user, err := todosrht.UserIDByName(c.Client, ctx, userName)
		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatalf("no such user %q", userName)
		}

		event, err := todosrht.UnassignUser(c.Client, ctx, trackerID, ticketID, user.Id)
		if err != nil {
			log.Fatal(err)
		} else if event == nil {
			log.Fatal("failed to unassign user")
		}

		log.Printf("Unassigned %q from %q\n", userName, event.Ticket.Subject)
	}

	cmd := &cobra.Command{
		Use:               "unassign <ID>",
		Short:             "Unassign a user from a ticket",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	cmd.Flags().StringVarP(&userName, "user", "u", "", "username")
	cmd.MarkFlagRequired("user")
	cmd.RegisterFlagCompletionFunc("user", completeTicketUnassign)
	return cmd
}

func newTodoTicketDeleteCommand() *cobra.Command {
	var autoConfirm bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		if !autoConfirm && !getConfirmation(fmt.Sprintf("Do you really want to delete the ticket with ID %d", ticketID)) {
			log.Println("Aborted")
			return
		}

		ticket, err := todosrht.DeleteTicket(c.Client, ctx, trackerID, ticketID)
		if err != nil {
			log.Fatal(err)
		} else if ticket == nil {
			log.Fatalf("failed to delete ticket %d", ticketID)
		}

		log.Printf("Deleted ticket %q\n", ticket.Subject)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a ticket",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "auto confirm")
	return cmd
}

func newTodoTicketShowCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		var (
			user     *todosrht.User
			username string
		)

		if owner != "" {
			username = strings.TrimLeft(owner, ownerPrefixes)
			user, err = todosrht.TicketByUser(c.Client, ctx, username, name, ticketID)
		} else {
			user, err = todosrht.TicketByName(c.Client, ctx, name, ticketID)
		}
		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatalf("no such user %q", username)
		} else if user.Tracker == nil {
			log.Fatalf("no such tracker %q", name)
		}

		ticket := user.Tracker.Ticket
		fmt.Printf("%s\n\n", termfmt.Bold.String(ticket.Subject))

		fmt.Printf("Status: %s\n", termfmt.Green.Sprintf("%s %s", ticket.Status, ticket.Resolution))
		fmt.Printf("Submitter: %s\n", ticket.Submitter.CanonicalName)

		assigned := "Assigned to: "
		if len(ticket.Assignees) == 0 {
			assigned += "No-one"
		} else {
			for i, assignee := range ticket.Assignees {
				assigned += assignee.CanonicalName
				if i != len(ticket.Assignees)-1 {
					assigned += ", "
				}
			}
		}
		fmt.Println(assigned)

		fmt.Printf("Submitted: %s\n", humanize.Time(ticket.Created.Time))
		fmt.Printf("Updated: %s\n", humanize.Time(ticket.Updated.Time))

		labels := "Labels: "
		if len(ticket.Labels) == 0 {
			labels += "No labels applied."
		} else {
			for i, label := range ticket.Labels {
				labels += label.TermString()
				if i != len(ticket.Labels)-1 {
					labels += " "
				}
			}
		}
		fmt.Println(labels)
		fmt.Println()

		if ticket.Body != nil {
			fmt.Println(*ticket.Body)
		}

		for i := len(ticket.Events.Results) - 1; i >= 0; i-- {
			event := ticket.Events.Results[i]
			for _, change := range event.Changes {
				comment, ok := change.Value.(*todosrht.Comment)
				if !ok {
					continue
				}
				author := termfmt.Bold.String(comment.Author.CanonicalName)
				created := termfmt.Dim.String("(" + humanize.Time(event.Created.Time) + ")")
				fmt.Println()
				fmt.Printf("%v %v\n", author, created)
				fmt.Print(indent(comment.Text, "  "))
				fmt.Println()
			}
		}
	}
	cmd := &cobra.Command{
		Use:               "show <ID>",
		Short:             "Show a ticket",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	return cmd
}

func newTodoTicketWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage ticket webhooks",
	}
	cmd.AddCommand(newTodoTicketWebhookCreateCommand())
	cmd.AddCommand(newTodoTicketWebhookListCommand())
	cmd.AddCommand(newTodoTicketWebhookDeleteCommand())
	return cmd
}

func newTodoTicketWebhookCreateCommand() *cobra.Command {
	var events []string
	var stdin bool
	var url string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		var config todosrht.TicketWebhookInput
		config.Url = url

		whEvents, err := todosrht.ParseTicketWebhookEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents
		config.Query = readWebhookQuery(stdin)

		webhook, err := todosrht.CreateTicketWebhook(c.Client, ctx, trackerID, ticketID, config)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Created ticket webhook with ID %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "create <ID>",
		Short:             "Create a ticket webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	cmd.Flags().StringSliceVarP(&events, "events", "e", nil, "webhook events")
	cmd.RegisterFlagCompletionFunc("events", completeTicketWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newTodoTicketWebhookListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		var (
			user     *todosrht.User
			username string
		)

		if owner != "" {
			username = strings.TrimLeft(owner, ownerPrefixes)
			user, err = todosrht.TicketWebhooksByUser(c.Client, ctx, username, name, ticketID)
		} else {
			user, err = todosrht.TicketWebhooks(c.Client, ctx, name, ticketID)
		}

		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatalf("no such user %q", username)
		} else if user.Tracker == nil {
			log.Fatalf("no such tracker %q", name)
		}

		for _, webhook := range user.Tracker.Ticket.Webhooks.Results {
			fmt.Printf("%s %s\n", termfmt.DarkYellow.Sprintf("#%d", webhook.Id), webhook.Url)
		}
	}

	cmd := &cobra.Command{
		Use:               "list <ID>",
		Short:             "List ticket webhooks",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	return cmd
}

func newTodoTicketWebhookDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("todo", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		webhook, err := todosrht.DeleteTicketWebhook(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a ticket webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newTodoLabelCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage labels",
	}
	cmd.AddCommand(newTodoLabelListCommand())
	cmd.AddCommand(newTodoLabelDeleteCommand())
	cmd.AddCommand(newTodoLabelCreateCommand())
	return cmd
}

func newTodoLabelListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		name, owner, instance, err := getTrackerName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		var (
			user     *todosrht.User
			username string
		)

		if owner != "" {
			username = strings.TrimLeft(owner, ownerPrefixes)
			user, err = todosrht.LabelsByUser(c.Client, ctx, username, name)
		} else {
			user, err = todosrht.Labels(c.Client, ctx, name)
		}

		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatalf("no such user %q", username)
		} else if user.Tracker == nil {
			log.Fatalf("no such tracker %q", name)
		}

		for _, label := range user.Tracker.Labels.Results {
			fmt.Println(label.TermString())
		}
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List labels",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	return cmd
}

func newTodoLabelDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		trackerName, owner, instance, err := getTrackerName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		id, err := getLabelID(c, ctx, trackerName, args[0], owner)
		if err != nil {
			log.Fatalf("failed to get label ID: %v", err)
		}

		label, err := todosrht.DeleteLabel(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted label %s\n", label.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete <name>",
		Short:             "Delete a label",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeLabel,
		Run:               run,
	}
	return cmd
}

func newTodoLabelCreateCommand() *cobra.Command {
	var fg, bg string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		name, owner, instance, err := getTrackerName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		id := getTrackerID(c, ctx, name, owner)

		if fg == "" {
			fg = calcLabelForeground(bg)
		}

		label, err := todosrht.CreateLabel(c.Client, ctx, id, args[0], fg, bg)
		if err != nil {
			log.Fatal(err)
		} else if label == nil {
			log.Fatal("failed to create label")
		}

		log.Printf("Created label %s\n", label.TermString())
	}

	cmd := &cobra.Command{
		Use:               "create <name>",
		Short:             "Create a label",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringVarP(&fg, "foreground", "f", "", "foreground color")
	cmd.RegisterFlagCompletionFunc("foreground", completeLabelColor)
	cmd.Flags().StringVarP(&bg, "background", "b", "", "background color")
	cmd.MarkFlagRequired("background")
	cmd.RegisterFlagCompletionFunc("background", completeLabelColor)
	return cmd
}

func newTodoACLCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acl",
		Short: "Manage access-control lists",
	}
	cmd.AddCommand(newTodoACLListCommand())
	cmd.AddCommand(newTodoACLDeleteCommand())
	return cmd
}

func newTodoACLListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		var name, instance string
		if len(args) > 0 {
			// TODO: handle owner
			name, _, instance = parseResourceName(args[0])
		} else {
			var err error
			name, _, instance, err = getTrackerName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("todo", cmd, instance)

		user, err := todosrht.AclByTrackerName(c.Client, ctx, name)
		if err != nil {
			log.Fatal(err)
		} else if user.Tracker == nil {
			log.Fatalf("no such tracker %q", name)
		}

		fmt.Println(termfmt.Bold.Sprint("Default permissions"))
		fmt.Println(user.Tracker.DefaultACL.TermString())

		if len(user.Tracker.Acls.Results) > 0 {
			fmt.Println(termfmt.Bold.Sprint("\nUser permissions"))
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer tw.Flush()
		for _, acl := range user.Tracker.Acls.Results {
			s := fmt.Sprintf("%s browse  %s submit  %s comment  %s edit  %s triage",
				todosrht.PermissionIcon(acl.Browse), todosrht.PermissionIcon(acl.Submit),
				todosrht.PermissionIcon(acl.Comment), todosrht.PermissionIcon(acl.Edit),
				todosrht.PermissionIcon(acl.Triage))
			created := termfmt.Dim.String(humanize.Time(acl.Created.Time))
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", termfmt.DarkYellow.Sprintf("#%d", acl.Id),
				acl.Entity.CanonicalName, s, created)
		}
	}

	cmd := &cobra.Command{
		Use:               "list",
		Short:             "List ACL entries",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newTodoACLDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("todo", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		acl, err := todosrht.DeleteACL(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if acl == nil {
			log.Fatalf("failed to delete ACL entry with ID %d", id)
		}

		log.Printf("Deleted ACL entry for %q in tracker %q\n", acl.Entity.CanonicalName, acl.Tracker.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete an ACL entry",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newTodoWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage tracker webhooks",
	}
	cmd.AddCommand(newTodoWebhookCreateCommand())
	cmd.AddCommand(newTodoWebhookListCommand())
	cmd.AddCommand(newTodoWebhookDeleteCommand())
	return cmd
}

func newTodoWebhookCreateCommand() *cobra.Command {
	var events []string
	var stdin bool
	var url string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getTrackerName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("todo", cmd, instance)
		id := getTrackerID(c, ctx, name, owner)

		var config todosrht.TrackerWebhookInput
		config.Url = url

		whEvents, err := todosrht.ParseTrackerWebhookEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents
		config.Query = readWebhookQuery(stdin)

		webhook, err := todosrht.CreateTrackerWebhook(c.Client, ctx, id, config)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Created tracker webhook with ID %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "create [tracker]",
		Short:             "Create a tracker webhook",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeTracker,
		Run:               run,
	}
	cmd.Flags().StringSliceVarP(&events, "events", "e", nil, "webhook events")
	cmd.RegisterFlagCompletionFunc("events", completeTrackerWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newTodoWebhookListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getTrackerName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("todo", cmd, instance)

		var (
			user     *todosrht.User
			username string
			err      error
		)

		if owner != "" {
			username = strings.TrimLeft(owner, ownerPrefixes)
			user, err = todosrht.TrackerWebhooksByUser(c.Client, ctx, username, name)
		} else {
			user, err = todosrht.TrackerWebhooks(c.Client, ctx, name)
		}

		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatalf("no such user %q", username)
		} else if user.Tracker == nil {
			log.Fatalf("no such tracker %q", name)
		}

		for _, webhook := range user.Tracker.Webhooks.Results {
			fmt.Printf("%s %s\n", termfmt.DarkYellow.Sprintf("#%d", webhook.Id), webhook.Url)
		}
	}

	cmd := &cobra.Command{
		Use:               "list [tracker]",
		Short:             "List tracker webhooks",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeTracker,
		Run:               run,
	}
	return cmd
}

func newTodoWebhookDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("todo", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		webhook, err := todosrht.DeleteTrackerWebhook(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a tracker webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newTodoUserWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user-webhook",
		Short: "Manage user webhooks",
	}
	cmd.AddCommand(newTodoUserWebhookCreateCommand())
	cmd.AddCommand(newTodoUserWebhookListCommand())
	cmd.AddCommand(newTodoUserWebhookDeleteCommand())
	return cmd
}

func newTodoUserWebhookCreateCommand() *cobra.Command {
	var events []string
	var stdin bool
	var url string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("todo", cmd)

		var config todosrht.UserWebhookInput
		config.Url = url

		whEvents, err := todosrht.ParseUserEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents
		config.Query = readWebhookQuery(stdin)

		webhook, err := todosrht.CreateUserWebhook(c.Client, ctx, config)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Created user webhook with ID %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "create",
		Short:             "Create a user webhook",
		Args:              cobra.ExactArgs(0),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringSliceVarP(&events, "events", "e", nil, "webhook events")
	cmd.RegisterFlagCompletionFunc("events", completeTodoUserWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newTodoUserWebhookListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("todo", cmd)

		webhooks, err := todosrht.UserWebhooks(c.Client, ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, webhook := range webhooks.Results {
			fmt.Printf("%s %s\n", termfmt.DarkYellow.Sprintf("#%d", webhook.Id), webhook.Url)
		}
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List user webhooks",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	return cmd
}

func newTodoUserWebhookDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("todo", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		webhook, err := todosrht.DeleteUserWebhook(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a user webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTodoUserWebhookID,
		Run:               run,
	}
	return cmd
}

const todoTicketCreatePrefill = `
<!--
Please enter the subject of the new ticket above. The subject line
can be followed by a blank line and a Markdown description. An
empty subject aborts the ticket.
-->`

func newTodoTicketCreateCommand() *cobra.Command {
	var stdin bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		name, owner, instance, err := getTrackerName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}
		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		var input todosrht.SubmitTicketInput
		if stdin {
			br := bufio.NewReader(os.Stdin)
			fmt.Printf("Subject: ")

			var err error
			input.Subject, err = br.ReadString('\n')
			if err != nil {
				log.Fatalf("failed to read subject: %v", err)
			}
			input.Subject = strings.TrimSpace(input.Subject)
			if input.Subject == "" {
				fmt.Println("Aborting due to empty subject.")
				os.Exit(1)
			}

			fmt.Printf("Description %s:\n", termfmt.Dim.String("(Markdown supported)"))
			bodyBytes, err := io.ReadAll(br)
			if err != nil {
				log.Fatalf("failed to read description: %v", err)
			}
			if body := strings.TrimSpace(string(bodyBytes)); body != "" {
				input.Body = &body
			}
		} else {
			text, err := getInputWithEditor("hut_ticket*.md", todoTicketCreatePrefill)
			if err != nil {
				log.Fatalf("failed to read ticket subject and description: %v", err)
			}

			text = dropComment(text, todoTicketCreatePrefill)

			parts := strings.SplitN(text, "\n", 2)
			input.Subject = strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				body := strings.TrimSpace(parts[1])
				input.Body = &body
			}
		}

		if input.Subject == "" {
			log.Println("Aborting due to empty subject.")
			os.Exit(1)
		}

		ticket, err := todosrht.SubmitTicket(c.Client, ctx, trackerID, input)
		if err != nil {
			log.Fatal(err)
		} else if ticket == nil {
			log.Fatal("failed to create ticket")
		}

		log.Printf("Created new ticket %v\n", termfmt.DarkYellow.Sprintf("#%v", ticket.Id))
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new ticket",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read ticket from stdin")
	return cmd
}

func newTodoTicketLabelCommand() *cobra.Command {
	var labelName string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, trackerName, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, trackerName, owner)

		labelID, err := getLabelID(c, ctx, trackerName, labelName, owner)
		if err != nil {
			log.Fatalf("failed to get label ID: %v", err)
		}

		event, err := todosrht.LabelTicket(c.Client, ctx, trackerID, ticketID, labelID)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Added label to %q\n", event.Ticket.Subject)
	}

	cmd := &cobra.Command{
		Use:               "label <ID>",
		Short:             "Add a label to a ticket",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	cmd.Flags().StringVarP(&labelName, "label", "l", "", "label name")
	cmd.MarkFlagRequired("label")
	// TODO: complete unassigned labels
	cmd.RegisterFlagCompletionFunc("label", cobra.NoFileCompletions)
	return cmd
}

func newTodoTicketUnlabelCommand() *cobra.Command {
	var labelName string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, trackerName, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, trackerName, owner)

		labelID, err := getLabelID(c, ctx, trackerName, labelName, owner)
		if err != nil {
			log.Fatalf("failed to get label ID: %v", err)
		}

		event, err := todosrht.UnlabelTicket(c.Client, ctx, trackerID, ticketID, labelID)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Removed label from %q\n", event.Ticket.Subject)
	}

	cmd := &cobra.Command{
		Use:               "unlabel <ID>",
		Short:             "Remove a label from a ticket",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTicketID,
		Run:               run,
	}
	cmd.Flags().StringVarP(&labelName, "label", "l", "", "label name")
	cmd.MarkFlagRequired("label")
	// TODO: complete assigned labels
	cmd.RegisterFlagCompletionFunc("label", cobra.NoFileCompletions)
	return cmd
}

func getTrackerID(c *Client, ctx context.Context, name, owner string) int32 {
	var (
		user     *todosrht.User
		username string
		err      error
	)

	if owner == "" {
		user, err = todosrht.TrackerIDByName(c.Client, ctx, name)
	} else {
		username = strings.TrimLeft(owner, ownerPrefixes)
		user, err = todosrht.TrackerIDByUser(c.Client, ctx, username, name)
	}
	if err != nil {
		log.Fatalf("failed to get tracker ID: %v", err)
	} else if user == nil {
		log.Fatalf("user %q does not exist", username)
	} else if user.Tracker == nil {
		log.Fatalf("tracker %q does not exist", name)
	}

	return user.Tracker.Id
}

func getTrackerName(ctx context.Context, cmd *cobra.Command) (name, owner, instance string, err error) {
	s, err := cmd.Flags().GetString("tracker")
	if err != nil {
		return "", "", "", err
	} else if s != "" {
		name, owner, instance = parseResourceName(s)
		return name, owner, instance, nil
	}

	// TODO: Use hub.sr.ht API to determine trackers
	name, owner, instance, err = guessGitRepoName(ctx, cmd)
	if err != nil {
		return "", "", "", err
	}

	return name, owner, instance, nil
}

func parseTicketResource(ctx context.Context, cmd *cobra.Command, ticket string) (ticketID int32, name, owner, instance string, err error) {
	if strings.Contains(ticket, "/") {
		var resource string
		resource, owner, instance = parseResourceName(ticket)
		split := strings.Split(resource, "/")
		if len(split) != 2 {
			return 0, "", "", "", errors.New("failed to parse tracker name and/or ID")
		}

		name = split[0]
		var err error
		ticketID, err = parseInt32(split[1])
		if err != nil {
			return 0, "", "", "", err
		}
	} else {
		var err error
		ticketID, err = parseInt32(ticket)
		if err != nil {
			return 0, "", "", "", err
		}
		name, owner, instance, err = getTrackerName(ctx, cmd)
		if err != nil {
			return 0, "", "", "", err
		}
	}

	return ticketID, name, owner, instance, nil
}

func calcLabelForeground(bg string) string {
	const white = "#FFFFFF"
	const black = "#000000"
	bgLuminance := calcLuminance(bg)
	contrastWhite := calcContrastRatio(bgLuminance, calcLuminance(white))
	contrastBlack := calcContrastRatio(bgLuminance, calcLuminance(black))

	if contrastBlack > contrastWhite {
		return black
	}
	return white
}

func calcLuminance(hex string) float64 {
	// https://www.w3.org/TR/WCAG/#dfn-relative-luminance
	rgb := termfmt.HexToRGB(hex)
	rsRGB := float64(rgb.Red) / 255
	gsRGB := float64(rgb.Green) / 255
	bsRGB := float64(rgb.Blue) / 255

	var r, g, b float64
	if rsRGB <= 0.03928 {
		r = rsRGB / 12.92
	} else {
		r = math.Pow((rsRGB+0.055)/1.055, 2.4)
	}
	if gsRGB <= 0.03928 {
		g = gsRGB / 12.92
	} else {
		g = math.Pow((gsRGB+0.055)/1.055, 2.4)
	}
	if bsRGB <= 0.03928 {
		b = bsRGB / 12.92
	} else {
		b = math.Pow((bsRGB+0.055)/1.055, 2.4)
	}

	return 0.2126*r + 0.7152*g + 0.0722*b
}

func calcContrastRatio(l1, l2 float64) float64 {
	// https://www.w3.org/TR/WCAG/#dfn-contrast-ratio
	if l1 > l2 {
		return (l1 + 0.05) / (l2 + 0.05)
	}

	return (l2 + 0.05) / (l1 + 0.05)
}

func getLabelID(c *Client, ctx context.Context, trackerName, labelName, owner string) (int32, error) {
	var (
		user     *todosrht.User
		username string
		err      error
	)

	if owner == "" {
		user, err = todosrht.LabelIDByName(c.Client, ctx, trackerName, labelName)
	} else {
		username = strings.TrimLeft(owner, ownerPrefixes)
		user, err = todosrht.LabelIDByUser(c.Client, ctx, username, trackerName, labelName)
	}
	if err != nil {
		return 0, err
	} else if user == nil {
		return 0, fmt.Errorf("user %q does not exist", username)
	} else if user.Tracker == nil {
		return 0, fmt.Errorf("tracker %q does not exist", trackerName)
	} else if user.Tracker.Label == nil {
		return 0, fmt.Errorf("label %q does not exist", labelName)
	}

	return user.Tracker.Label.Id, nil
}

func completeTicketID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var tickets []string
	ctx := cmd.Context()

	name, owner, instance, err := getTrackerName(ctx, cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	c := createClientWithInstance("todo", cmd, instance)
	var user *todosrht.User

	includeSubscription := false
	if cmd.Name() == "subscribe" || cmd.Name() == "unsubscribe" {
		includeSubscription = true
	}

	if owner != "" {
		username := strings.TrimLeft(owner, ownerPrefixes)
		user, err = todosrht.CompleteTicketIdByUser(c.Client, ctx, username, name, includeSubscription)
	} else {
		user, err = todosrht.CompleteTicketId(c.Client, ctx, name, includeSubscription)
	}
	if err != nil || user == nil || user.Tracker == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, ticket := range user.Tracker.Tickets.Results {
		if cmd.Name() == "subscribe" && ticket.Subscription != nil {
			continue
		} else if cmd.Name() == "unsubscribe" && ticket.Subscription == nil {
			continue
		}

		s := fmt.Sprintf("%d\t%s", ticket.Id, ticket.Subject)
		tickets = append(tickets, s)
	}

	return tickets, cobra.ShellCompDirectiveNoFileComp
}

var completeTicketStatus = cobra.FixedCompletions([]string{
	"reported",
	"confirmed",
	"in_progress",
	"pending",
	"resolved",
}, cobra.ShellCompDirectiveNoFileComp)

var completeTicketResolution = cobra.FixedCompletions([]string{
	"unresolved",
	"closed",
	"fixed",
	"implemented",
	"wont_fix",
	"by_design",
	"invalid",
	"duplicate",
	"not_our_bug",
}, cobra.ShellCompDirectiveNoFileComp)

func completeLabelColor(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var colors []string
	colorMap := map[string]string{"black": "#000000", "white": "#FFFFFF", "blue": "#3584E4", "green": "#33D17A",
		"yellow": "#F6D32D", "orange": "#FF7800", "red": "#E01B24", "purple": "#9141AC", "brown": "#986A44"}

	for k, v := range colorMap {
		colors = append(colors, fmt.Sprintf("%s\t%s", v, k))
	}
	return colors, cobra.ShellCompDirectiveNoFileComp
}

func completeTicketUnassign(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx := cmd.Context()
	var assignees []string
	ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	c := createClientWithInstance("todo", cmd, instance)

	var user *todosrht.User

	if owner != "" {
		username := strings.TrimLeft(owner, ownerPrefixes)
		user, err = todosrht.AssigneesByUser(c.Client, ctx, username, name, ticketID)
	} else {
		user, err = todosrht.Assignees(c.Client, ctx, name, ticketID)
	}
	if err != nil || user == nil || user.Tracker == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, user := range user.Tracker.Ticket.Assignees {
		userName := strings.TrimLeft(user.CanonicalName, ownerPrefixes)
		assignees = append(assignees, userName)
	}

	return assignees, cobra.ShellCompDirectiveNoFileComp
}

func completeTicketAssign(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx := cmd.Context()
	ticketID, name, owner, instance, err := parseTicketResource(ctx, cmd, args[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	c := createClientWithInstance("todo", cmd, instance)

	var (
		me   *todosrht.User
		user *todosrht.User
	)
	candidates := make(map[string]struct{})

	if owner != "" {
		username := strings.TrimLeft(owner, ownerPrefixes)
		me, user, err = todosrht.CompleteTicketAssignByUser(c.Client, ctx, username, name, ticketID)
		candidates[me.CanonicalName] = struct{}{}
	} else {
		user, err = todosrht.CompleteTicketAssign(c.Client, ctx, name, ticketID)
		candidates[user.CanonicalName] = struct{}{}
	}
	if err != nil || user == nil || user.Tracker == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, ticket := range user.Tracker.Tickets.Results {
		for _, user := range ticket.Assignees {
			candidates[user.CanonicalName] = struct{}{}
		}
	}

	assignedUsers := make(map[string]struct{})
	for _, user := range user.Tracker.Ticket.Assignees {
		assignedUsers[user.CanonicalName] = struct{}{}
	}

	var potentialAssignees []string
	for user := range candidates {
		// user already assigned
		if _, ok := assignedUsers[user]; ok {
			continue
		}

		userName := strings.TrimLeft(user, ownerPrefixes)
		potentialAssignees = append(potentialAssignees, userName)
	}

	return potentialAssignees, cobra.ShellCompDirectiveNoFileComp
}

func completeTracker(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("todo", cmd)
	var trackerList []string

	trackers, err := todosrht.TrackerNames(c.Client, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, tracker := range trackers.Results {
		trackerList = append(trackerList, tracker.Name)
	}

	return trackerList, cobra.ShellCompDirectiveNoFileComp
}

func completeTicketWebhookEvents(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var eventList []string
	events := [3]string{"event_created", "ticket_update", "ticket_deleted"}
	set := strings.ToLower(cmd.Flag("events").Value.String())
	for _, event := range events {
		if !strings.Contains(set, event) {
			eventList = append(eventList, event)
		}
	}
	return eventList, cobra.ShellCompDirectiveNoFileComp
}

func completeTodoUserWebhookEvents(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var eventList []string
	events := [4]string{"tracker_created", "tracker_update", "tracker_deleted", "ticket_created"}
	set := strings.ToLower(cmd.Flag("events").Value.String())
	for _, event := range events {
		if !strings.Contains(set, event) {
			eventList = append(eventList, event)
		}
	}
	return eventList, cobra.ShellCompDirectiveNoFileComp
}

func completeTodoUserWebhookID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("todo", cmd)
	var webhookList []string

	webhooks, err := todosrht.UserWebhooks(c.Client, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, webhook := range webhooks.Results {
		s := fmt.Sprintf("%d\t%s", webhook.Id, webhook.Url)
		webhookList = append(webhookList, s)
	}

	return webhookList, cobra.ShellCompDirectiveNoFileComp
}

func completeTrackerWebhookEvents(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var eventList []string
	events := [9]string{"tracker_update", "tracker_deleted", "label_created", "label_update", "label_deleted",
		"ticket_created", "ticket_update", "ticket_deleted", "event_created"}
	set := strings.ToLower(cmd.Flag("events").Value.String())
	for _, event := range events {
		if !strings.Contains(set, event) {
			eventList = append(eventList, event)
		}
	}
	return eventList, cobra.ShellCompDirectiveNoFileComp
}

func completeLabel(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	var labelList []string

	name, owner, instance, err := getTrackerName(ctx, cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	c := createClientWithInstance("todo", cmd, instance)
	var user *todosrht.User

	if owner != "" {
		username := strings.TrimLeft(owner, ownerPrefixes)
		user, err = todosrht.CompleteLabelByUser(c.Client, ctx, username, name)
	} else {
		user, err = todosrht.CompleteLabel(c.Client, ctx, name)
	}
	if err != nil || user == nil || user.Tracker == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, label := range user.Tracker.Labels.Results {
		labelList = append(labelList, label.Name)
	}

	return labelList, cobra.ShellCompDirectiveNoFileComp
}
