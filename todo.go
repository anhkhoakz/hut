package main

import (
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
	cmd.PersistentFlags().StringP("tracker", "t", "", "name of tracker")
	cmd.RegisterFlagCompletionFunc("tracker", completeTracker)
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

		fmt.Printf("Created tracker %q\n", tracker.Name)
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
			user     *todosrht.User
			username string
		)

		if owner != "" {
			username = strings.TrimLeft(owner, ownerPrefixes)
			user, err = todosrht.TicketsByUser(c.Client, ctx, username, name)
		} else {
			user, err = todosrht.Tickets(c.Client, ctx, name)
		}

		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatalf("no such user %q", username)
		} else if user.Tracker == nil {
			log.Fatalf("no such tracker %q", name)
		}

		for _, ticket := range user.Tracker.Tickets.Results {
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
			log.Fatalf("resolution is required when status is RESOLVED")
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

		fmt.Printf("Updated status of %s\n", event.Ticket.Subject)
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

		fmt.Printf("Subscribed to %s/%s/%s/%d\n", c.BaseURL, owner, name, subscription.Ticket.Id)
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

		fmt.Printf("Unsubscribed from %s/%s/%s/%d\n", c.BaseURL, owner, name, subscription.Ticket.Id)
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

		fmt.Printf("Assigned %q to %q\n", userName, event.Ticket.Subject)
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

		fmt.Printf("Unassigned %q from %q\n", userName, event.Ticket.Subject)
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
		_, _, instance, err := getTrackerName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}

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
		for _, acl := range user.Tracker.Acls.Results {
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
	name, owner, instance, err = guessGitRepoName(ctx)
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

func completeTicketStatus(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"reported", "confirmed", "in_progress", "pending", "resolved"},
		cobra.ShellCompDirectiveNoFileComp
}

func completeTicketResolution(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"unresolved", "fixed", "implemented", "wont_fix", "by_design",
		"invalid", "duplicate", "not_our_bug"}, cobra.ShellCompDirectiveNoFileComp
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
