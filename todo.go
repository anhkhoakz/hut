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
	cmd.AddCommand(newTodoSubscribeCommand())
	cmd.AddCommand(newTodoUnsubscribeCommand())
	cmd.AddCommand(newTodoTicketCommand())
	cmd.AddCommand(newTodoLabelCommand())
	cmd.AddCommand(newTodoACLCommand())
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

func newTodoSubscribeCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			name, owner, instance = getTrackerName(ctx, cmd)
		}
		c := createClientWithInstance("todo", cmd, instance)
		id := getTrackerID(c, ctx, name, owner)

		subscription, err := todosrht.TrackerSubscribe(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if subscription == nil {
			log.Fatalf("failed to subscribe to tracker %q", name)
		}

		fmt.Printf("Subscribed to %s/%s/%s\n", c.BaseURL, subscription.Tracker.Owner.CanonicalName, subscription.Tracker.Name)
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
			name, owner, instance = getTrackerName(ctx, cmd)
		}
		c := createClientWithInstance("todo", cmd, instance)
		id := getTrackerID(c, ctx, name, owner)

		subscription, err := todosrht.TrackerUnsubscribe(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if subscription == nil {
			log.Fatalf("you were not subscribed to %s/%s/%s", c.BaseURL, owner, name)
		}

		fmt.Printf("Unsubscribed from %s/%s/%s\n", c.BaseURL, subscription.Tracker.Owner.CanonicalName, subscription.Tracker.Name)
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

func newTodoTicketCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ticket",
		Short: "Manage tickets",
	}
	cmd.AddCommand(newTodoTicketListCommand())
	cmd.AddCommand(newTodoTicketCommentCommand())
	cmd.AddCommand(newTodoTicketStatusCommand())
	cmd.AddCommand(newTodoTicketSubscribeCommand())
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
				labels = " "
				for i, label := range ticket.Labels {
					labels += label.TermString()
					if i != len(ticket.Labels)-1 {
						labels += " "
					}
				}
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
	var stdin bool
	var status, resolution string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance := parseTicketResource(ctx, cmd, args[0])

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

		if *input.Status != todosrht.TicketStatusResolved && input.Resolution != nil {
			log.Fatalf("resolution %q specified, but ticket not marked as resolved", resolution)
		}
		if *input.Status == todosrht.TicketStatusResolved && input.Resolution == nil {
			log.Fatalf("resolution is required when status is RESOLVED")
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
			fmt.Println("Aborted writing empty comment")
			return
		}

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
			log.Fatalf("resolution is required when status is RESOLVED")
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
	cmd.Flags().StringVarP(&resolution, "resolution", "r", "", "ticket resolution")
	cmd.RegisterFlagCompletionFunc("resolution", completeTicketResolution)
	return cmd
}

func newTodoTicketSubscribeCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ticketID, name, owner, instance := parseTicketResource(ctx, cmd, args[0])
		c := createClientWithInstance("todo", cmd, instance)
		trackerID := getTrackerID(c, ctx, name, owner)

		subscription, err := todosrht.TicketSubscribe(c.Client, ctx, trackerID, ticketID)
		if err != nil {
			log.Fatal(err)
		} else if subscription == nil {
			log.Fatalf("failed to subscribe to ticket %d", ticketID)
		}

		fmt.Printf("Subscribed to %s/%s/%s/%d\n", c.BaseURL, owner, name, subscription.Ticket.Id)
	}

	cmd := &cobra.Command{
		Use:               "subscribe <ID>",
		Short:             "Subscribe to a ticket",
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
		name, owner, instance := getTrackerName(ctx, cmd)
		c := createClientWithInstance("todo", cmd, instance)

		var (
			tracker *todosrht.Tracker
			err     error
		)

		if owner != "" {
			tracker, err = todosrht.LabelsByOwner(c.Client, ctx, owner, name)
		} else {
			tracker, err = todosrht.Labels(c.Client, ctx, name)
		}

		if err != nil {
			log.Fatal(err)
		} else if tracker == nil {
			log.Fatalf("no such tracker %q", name)
		}

		for _, label := range tracker.Labels.Results {
			fmt.Printf("%s %s\n", termfmt.DarkYellow.Sprintf("#%d", label.Id), label.TermString())
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
		_, _, instance := getTrackerName(ctx, cmd)
		c := createClientWithInstance("todo", cmd, instance)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		label, err := todosrht.DeleteLabel(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if label == nil {
			log.Fatal("failed to delete label")
		}

		fmt.Printf("Deleted label %s\n", label.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a label",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newTodoLabelCreateCommand() *cobra.Command {
	var fg, bg string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		name, owner, instance := getTrackerName(ctx, cmd)
		c := createClientWithInstance("todo", cmd, instance)

		id := getTrackerID(c, ctx, name, owner)

		label, err := todosrht.CreateLabel(c.Client, ctx, id, args[0], fg, bg)
		if err != nil {
			log.Fatal(err)
		} else if label == nil {
			log.Fatal("failed to create label")
		}

		fmt.Printf("Created label %s\n", label.TermString())
	}

	cmd := &cobra.Command{
		Use:               "create <name>",
		Short:             "Create a label",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringVarP(&fg, "foreground", "f", "", "foreground color")
	cmd.MarkFlagRequired("foreground")
	cmd.RegisterFlagCompletionFunc("foreground", cobra.NoFileCompletions)
	cmd.Flags().StringVarP(&bg, "background", "b", "", "background color")
	cmd.MarkFlagRequired("background")
	cmd.RegisterFlagCompletionFunc("background", cobra.NoFileCompletions)
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
			name, _, instance = getTrackerName(ctx, cmd)
		}

		c := createClientWithInstance("todo", cmd, instance)

		tracker, err := todosrht.AclByTrackerName(c.Client, ctx, name)
		if err != nil {
			log.Fatal(err)
		} else if tracker == nil {
			log.Fatalf("no such tracker %q", name)
		}

		fmt.Println(termfmt.Bold.Sprint("Default permissions"))
		fmt.Println(tracker.DefaultACL.TermString())

		if len(tracker.Acls.Results) > 0 {
			fmt.Println(termfmt.Bold.Sprint("\nUser permissions"))
		}
		for _, acl := range tracker.Acls.Results {
			s := fmt.Sprintf("%s browse  %s submit  %s comment  %s edit %s triage",
				todosrht.PermissionIcon(acl.Browse), todosrht.PermissionIcon(acl.Submit),
				todosrht.PermissionIcon(acl.Comment), todosrht.PermissionIcon(acl.Edit),
				todosrht.PermissionIcon(acl.Triage))
			fmt.Printf("%s %s %s %s ago\n", termfmt.DarkYellow.Sprintf("#%d", acl.Id),
				acl.Entity.CanonicalName, s, timeDelta(acl.Created))
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

		fmt.Printf("Deleted ACL entry for %q in tracker %q\n", acl.Entity.CanonicalName, acl.Tracker.Name)
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
