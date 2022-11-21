package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"os"
	"os/exec"
	"strings"

	"github.com/juju/ansiterm/tabwriter"

	"github.com/spf13/cobra"

	"git.sr.ht/~emersion/hut/srht/listssrht"
	"git.sr.ht/~emersion/hut/termfmt"
)

func newListsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lists",
		Short: "Use the lists API",
	}
	cmd.AddCommand(newListsDeleteCommand())
	cmd.AddCommand(newListsListCommand())
	cmd.AddCommand(newListsSubscribeCommand())
	cmd.AddCommand(newListsUnsubscribeCommand())
	cmd.AddCommand(newListsCreateCommand())
	cmd.AddCommand(newListsArchiveCommand())
	cmd.AddCommand(newListsPatchsetCommand())
	cmd.AddCommand(newListsACLCommand())
	cmd.AddCommand(newListsUserWebhookCommand())
	cmd.PersistentFlags().StringP("mailing-list", "l", "", "mailing list name")
	return cmd
}

func newListsDeleteCommand() *cobra.Command {
	var autoConfirm bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}
		c := createClientWithInstance("lists", cmd, instance)
		id := getMailingListID(c, ctx, name, owner)

		if !autoConfirm && !getConfirmation(fmt.Sprintf("Do you really want to delete the list %s", name)) {
			fmt.Println("Aborted")
			return
		}

		list, err := listssrht.DeleteMailingList(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if list == nil {
			log.Fatalf("failed to delete list with ID %d", id)
		}

		fmt.Printf("Deleted mailing list %s\n", list.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete [list]",
		Short:             "Delete a mailing list",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "auto confirm")
	return cmd
}

func newListsListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list [username]",
		Short:             "List mailing lists",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	cmd.Run = func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)
		var cursor *listssrht.Cursor
		var username string
		if len(args) > 0 {
			username = strings.TrimLeft(args[0], ownerPrefixes)
		}
		pagerify(func(p pager) bool {
			var lists *listssrht.MailingListCursor
			if len(username) > 0 {
				user, err := listssrht.MailingListsByUser(c.Client, ctx, username, cursor)
				if err != nil {
					log.Fatal(err)
				} else if user == nil {
					log.Fatal("no such user")
				}
				lists = user.Lists
			} else {
				var err error
				user, err := listssrht.MailingLists(c.Client, ctx, cursor)
				if err != nil {
					log.Fatal(err)
				}
				lists = user.Lists
			}

			for _, list := range lists.Results {
				printList(p, &list)
			}

			cursor = lists.Cursor
			return cursor == nil
		})
	}
	return cmd
}

func printList(w io.Writer, list *listssrht.MailingList) {
	fmt.Fprintf(w, "%s (%s)\n", termfmt.Bold.String(list.Name), list.Visibility.TermString())
	if list.Description != nil && *list.Description != "" {
		fmt.Fprintln(w, "\n"+indent(*list.Description, "  ")+"\n")
	}
}

func newListsSubscribeCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}
		c := createClientWithInstance("lists", cmd, instance)
		id := getMailingListID(c, ctx, name, owner)

		subscription, err := listssrht.MailingListSubscribe(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Subscribed to %s/%s/%s\n", c.BaseURL, subscription.List.Owner.CanonicalName, subscription.List.Name)
	}

	cmd := &cobra.Command{
		Use:               "subscribe [list]",
		Short:             "Subscribe to a mailing list",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newListsUnsubscribeCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}
		c := createClientWithInstance("lists", cmd, instance)
		id := getMailingListID(c, ctx, name, owner)

		subscription, err := listssrht.MailingListUnsubscribe(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if subscription == nil {
			log.Fatalf("you were not subscribed to %s/%s/%s", c.BaseURL, owner, name)
		}

		fmt.Printf("Unsubscribed from %s/%s/%s\n", c.BaseURL, subscription.List.Owner.CanonicalName, subscription.List.Name)
	}

	cmd := &cobra.Command{
		Use:               "unsubscribe [list]",
		Short:             "Unubscribe from a mailing list",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

const listsCreatePrefill = `
<!--
Please write the Markdown description of the new mailing list above.
-->`

func newListsCreateCommand() *cobra.Command {
	var visibility string
	var stdin bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)

		listVisibility, err := listssrht.ParseVisibility(visibility)
		if err != nil {
			log.Fatal(err)
		}

		var description *string

		if stdin {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				log.Fatalf("failed to read mailing list description: %v", err)
			}

			desc := string(b)
			description = &desc
		} else {
			text, err := getInputWithEditor("hut_mailing-list*.md", listsCreatePrefill)
			if err != nil {
				log.Fatalf("failed to read mailing list description: %v", err)
			}

			text = dropComment(text, listsCreatePrefill)
			description = &text
		}

		tracker, err := listssrht.CreateMailingList(c.Client, ctx, args[0], description, listVisibility)
		if err != nil {
			log.Fatal(err)
		} else if tracker == nil {
			log.Fatal("failed to create mailing list")
		}

		fmt.Printf("Created mailing list %q\n", tracker.Name)
	}
	cmd := &cobra.Command{
		Use:               "create <name>",
		Short:             "Create a mailing list",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read mailing list from stdin")
	cmd.Flags().StringVarP(&visibility, "visibility", "v", "public", "mailing list visibility")
	cmd.RegisterFlagCompletionFunc("visibility", completeVisibility)
	return cmd
}

func newListsArchiveCommand() *cobra.Command {
	var days int
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("lists", cmd, instance)
		c.HTTP.Timeout = fileTransferTimeout

		var (
			user     *listssrht.User
			username string
			err      error
		)
		if owner == "" {
			user, err = listssrht.Archive(c.Client, ctx, name)
		} else {
			username = strings.TrimLeft(owner, ownerPrefixes)
			user, err = listssrht.ArchiveByUser(c.Client, ctx, username, name)
		}
		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatalf("no such user %q", username)
		} else if user.List == nil {
			if owner == "" {
				log.Fatalf("no such mailing list %s", name)
			}
			log.Fatalf("no such mailing list %s/%s/%s", c.BaseURL, owner, name)
		}

		url := string(user.List.Archive)
		if days != 0 {
			url = fmt.Sprintf("%s?since=%d", url, days)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(url), nil)
		if err != nil {
			log.Fatalf("Failed to create request to fetch archive: %v", err)
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			log.Fatalf("Failed to fetch archive: %v", err)
		}
		defer resp.Body.Close()

		if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
			log.Fatalf("Failed to copy to stdout: %v", err)
		}
	}

	cmd := &cobra.Command{
		Use:               "archive [list]",
		Short:             "Download a mailing list archive",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().IntVarP(&days, "days", "d", 0, "number of last days to download")
	cmd.RegisterFlagCompletionFunc("days", cobra.NoFileCompletions)
	return cmd
}

func newListsPatchsetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patchset",
		Short: "Manage patchsets",
	}
	cmd.AddCommand(newListsPatchsetListCommand())
	cmd.AddCommand(newListsPatchsetUpdateCommand())
	cmd.AddCommand(newListsPatchsetApplyCommand())
	cmd.AddCommand(newListsPatchsetShowCommand())
	return cmd
}

func newListsPatchsetListCommand() *cobra.Command {
	var byUser bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}
		c := createClientWithInstance("lists", cmd, instance)

		var (
			err     error
			patches *listssrht.PatchsetCursor
		)

		if byUser {
			var user *listssrht.User
			if len(args) > 0 {
				name = strings.TrimLeft(name, ownerPrefixes)
				user, err = listssrht.PatchesByUser(c.Client, ctx, name)
			} else {
				user, err = listssrht.Patches(c.Client, ctx)
			}

			if err != nil {
				log.Fatal(err)
			} else if user == nil {
				log.Fatalf("no such user %q", name)
			}
			patches = user.Patches
		} else {
			var (
				user     *listssrht.User
				username string
			)

			if owner != "" {
				username = strings.TrimLeft(owner, ownerPrefixes)
				user, err = listssrht.ListPatchesByUser(c.Client, ctx, username, name)
			} else {
				user, err = listssrht.ListPatches(c.Client, ctx, name)
			}

			if err != nil {
				log.Fatal(err)
			} else if user == nil {
				log.Fatalf("no such user %q", username)
			} else if user.List == nil {
				log.Fatalf("no such list %q", name)
			}
			patches = user.List.Patches
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		defer tw.Flush()
		for _, patchset := range patches.Results {
			s := fmt.Sprintf("%s\t%s\t", termfmt.DarkYellow.Sprintf("#%d", patchset.Id), patchset.Status.TermString())
			if patchset.Prefix != nil && *patchset.Prefix != "" {
				s += fmt.Sprintf("[%s] ", *patchset.Prefix)
			}
			s += patchset.Subject
			if patchset.Version != 1 {
				s += fmt.Sprintf(" v%d", patchset.Version)
			}

			created := termfmt.Dim.Sprintf("%s ago", timeDelta(patchset.Created))

			if byUser {
				s += fmt.Sprintf("\t%s/%s\t%s", patchset.List.Owner.CanonicalName,
					patchset.List.Name, created)
			} else {
				s += fmt.Sprintf("\t%s\t%s", patchset.Submitter.CanonicalName, created)
			}
			fmt.Fprintln(tw, s)
		}
	}

	cmd := &cobra.Command{
		Use:               "list [list]",
		Short:             "List patchsets",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&byUser, "user", "u", false, "list patches by user")
	return cmd
}

func newListsPatchsetUpdateCommand() *cobra.Command {
	var status string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		patchStatus, err := listssrht.ParsePatchsetStatus(status)
		if err != nil {
			log.Fatal(err)
		}

		id, instance, err := parsePatchID(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}
		c := createClientWithInstance("lists", cmd, instance)

		patch, err := listssrht.UpdatePatchset(c.Client, ctx, id, patchStatus)
		if err != nil {
			log.Fatal(err)
		} else if patch == nil {
			log.Fatalf("failed to update patchset with ID %d", id)
		}

		fmt.Printf("Updated patchset %q by %s\n", patch.Subject, patch.Submitter.CanonicalName)
	}

	cmd := &cobra.Command{
		Use:               "update <ID>",
		Short:             "Update a patchset",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completePatchsetID,
		Run:               run,
	}
	cmd.Flags().StringVarP(&status, "status", "s", "", "patchset status")
	cmd.RegisterFlagCompletionFunc("status", completePatchsetStatus)
	cmd.MarkFlagRequired("status")
	return cmd
}

func newListsPatchsetShowCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:               "show <ID>",
		Short:             "Show a patchset",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completePatchsetID,
	}
	cmd.Run = func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		id, instance, err := parsePatchID(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}
		c := createClientWithInstance("lists", cmd, instance)

		patchset, err := listssrht.PatchsetById(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if patchset == nil {
			log.Fatalf("no such patchset %d", id)
		}

		for _, patch := range patchset.Patches.Results {
			formatPatch(os.Stdout, &patch)
		}
	}
	return &cmd
}

func newListsPatchsetApplyCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		id, instance, err := parsePatchID(ctx, cmd, args[0])
		if err != nil {
			log.Fatal(err)
		}
		c := createClientWithInstance("lists", cmd, instance)

		patchset, err := listssrht.PatchsetById(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if patchset == nil {
			log.Fatalf("no such patchset %d", id)
		}

		var mbox bytes.Buffer
		for _, patch := range patchset.Patches.Results {
			formatPatch(&mbox, &patch)
		}

		applyCmd := exec.Command("git", "am", "-3")
		applyCmd.Stdin = &mbox
		applyCmd.Stdout = os.Stdout
		applyCmd.Stderr = os.Stderr

		if err := applyCmd.Run(); err != nil {
			log.Fatal(err)
		}
	}

	cmd := &cobra.Command{
		Use:               "apply <ID>",
		Short:             "Apply a patchset",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completePatchsetID,
		Run:               run,
	}
	return cmd
}

func formatPatch(w io.Writer, email *listssrht.Email) {
	fmt.Fprintf(w, "From nobody %s\n", email.Date.Format(dateLayout))
	fmt.Fprintf(w, "From: %s\n", email.Header[0])
	fmt.Fprintf(w, "Subject: %s\n", email.Subject)
	fmt.Fprintf(w, "Date: %s\n\n", email.Date.Format(dateLayout))
	io.WriteString(w, strings.ReplaceAll(email.Body, "\r\n", "\n"))
}

func newListsACLCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acl",
		Short: "Manage access-control lists",
	}
	cmd.AddCommand(newListsACLListCommand())
	cmd.AddCommand(newListsACLDeleteCommand())
	return cmd
}

func newListsACLListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, instance string
		if len(args) > 0 {
			// TODO: handle owner
			name, _, instance = parseResourceName(args[0])
		} else {
			var err error
			name, _, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("lists", cmd, instance)

		user, err := listssrht.AclByListName(c.Client, ctx, name)
		if err != nil {
			log.Fatal(err)
		} else if user.List == nil {
			log.Fatalf("no such list %q", name)
		}

		fmt.Println(termfmt.Bold.Sprint("Default permissions"))
		fmt.Println(user.List.DefaultACL.TermString())

		if len(user.List.Acl.Results) > 0 {
			fmt.Println(termfmt.Bold.Sprint("\nUser permissions"))
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer tw.Flush()
		for _, acl := range user.List.Acl.Results {
			s := fmt.Sprintf("%s browse  %s reply  %s post  %s moderate",
				listssrht.PermissionIcon(acl.Browse), listssrht.PermissionIcon(acl.Reply),
				listssrht.PermissionIcon(acl.Post), listssrht.PermissionIcon(acl.Moderate))
			created := termfmt.Dim.Sprintf("%s ago", timeDelta(acl.Created))
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

func newListsACLDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		acl, err := listssrht.DeleteACL(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if acl == nil {
			log.Fatalf("failed to delete ACL entry with ID %d", id)
		}

		fmt.Printf("Deleted ACL entry for %q in mailing list %q\n", acl.Entity.CanonicalName, acl.List.Name)
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

func newListsUserWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user-webhook",
		Short: "Manage user webhooks",
	}
	cmd.AddCommand(newListsUserWebhookCreateCommand())
	cmd.AddCommand(newListsUserWebhookListCommand())
	cmd.AddCommand(newListsUserWebhookDeleteCommand())
	return cmd
}

func newListsUserWebhookCreateCommand() *cobra.Command {
	var events []string
	var stdin bool
	var url string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)

		var config listssrht.UserWebhookInput
		config.Url = url

		whEvents, err := listssrht.ParseUserEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents
		config.Query = readWebhookQuery(stdin)

		webhook, err := listssrht.CreateUserWebhook(c.Client, ctx, config)
		if err != nil {
			log.Fatal(err)
		} else if webhook == nil {
			log.Fatal("failed to create webhook")
		}

		fmt.Printf("Created user webhook with ID %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "create",
		Short:             "Create a user webhook",
		Args:              cobra.ExactArgs(0),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringSliceVarP(&events, "events", "e", nil, "webhook events")
	cmd.RegisterFlagCompletionFunc("events", completeListsUserWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newListsUserWebhookListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)

		webhooks, err := listssrht.UserWebhooks(c.Client, ctx)
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

func newListsUserWebhookDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		webhook, err := listssrht.DeleteUserWebhook(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if webhook == nil {
			log.Fatal("failed to delete webhook")
		}

		fmt.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a user webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeListsUserWebhookID,
		Run:               run,
	}
	return cmd
}

func getMailingListID(c *Client, ctx context.Context, name, owner string) int32 {
	var (
		user     *listssrht.User
		username string
		err      error
	)

	if owner == "" {
		user, err = listssrht.MailingListIDByName(c.Client, ctx, name)
	} else {
		username = strings.TrimLeft(owner, ownerPrefixes)
		user, err = listssrht.MailingListIDByUser(c.Client, ctx, username, name)
	}
	if err != nil {
		log.Fatal(err)
	} else if user == nil {
		log.Fatalf("no such user %q", username)
	} else if user.List == nil {
		if owner == "" {
			log.Fatalf("no such mailing list %s", name)
		}
		log.Fatalf("no such mailing list %s/%s/%s", c.BaseURL, owner, name)
	}
	return user.List.Id
}

func getMailingListName(ctx context.Context, cmd *cobra.Command) (name, owner, instance string, err error) {
	s, err := cmd.Flags().GetString("mailing-list")
	if err != nil {
		return "", "", "", err
	} else if s != "" {
		name, owner, instance = parseResourceName(s)
		return name, owner, instance, nil
	}

	name, owner, instance, err = guessMailingListName(ctx)
	if err != nil {
		return "", "", "", err
	}

	return name, owner, instance, nil
}

func guessMailingListName(ctx context.Context) (name, owner, instance string, err error) {
	addr, err := getGitSendEmailTo(ctx)
	if err != nil {
		return "", "", "", err
	} else if addr == nil {
		return "", "", "", errors.New("no mailing list specified and no mailing list configured for current Git repository")
	}

	parts := strings.SplitN(addr.Address, "@", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid email address %q", addr.Address)
	}

	name, owner, _ = parseResourceName(parts[0])
	instance = parts[1]
	return name, owner, instance, nil
}

func getGitSendEmailTo(ctx context.Context) (*mail.Address, error) {
	out, err := exec.CommandContext(ctx, "git", "config", "--default=", "sendemail.to").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git sendemail.to config: %v", err)
	}
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return nil, nil
	}
	addr, err := mail.ParseAddress(string(out))
	if err != nil {
		return nil, fmt.Errorf("failed to parse git sendemail.to: %v", err)
	}
	return addr, nil
}

func parsePatchID(ctx context.Context, cmd *cobra.Command, s string) (id int32, instance string, err error) {
	if strings.Contains(s, "/") {
		s, _, instance = parseResourceName(s)
		split := strings.Split(s, "/")
		s = split[len(split)-1]
		id, err = parseInt32(s)
		if err != nil {
			return 0, "", fmt.Errorf("invalid patchset ID: %v", err)
		}
	} else {
		id, err = parseInt32(s)
		if err != nil {
			return 0, "", err
		}

		_, _, instance, err = getMailingListName(ctx, cmd)
		if err != nil {
			return 0, "", err
		}
	}

	return id, instance, nil
}

var completePatchsetStatus = cobra.FixedCompletions([]string{
	"unknown",
	"proposed",
	"needs_revision",
	"superseded",
	"approved",
	"rejected",
	"applied",
}, cobra.ShellCompDirectiveNoFileComp)

func completePatchsetID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	var patchList []string

	name, owner, instance, err := getMailingListName(ctx, cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	c := createClientWithInstance("lists", cmd, instance)

	var user *listssrht.User
	if owner != "" {
		username := strings.TrimLeft(owner, ownerPrefixes)
		user, err = listssrht.CompletePatchsetIdByUser(c.Client, ctx, username, name)
	} else {
		user, err = listssrht.CompletePatchsetId(c.Client, ctx, name)
	}

	if err != nil || user == nil || user.List == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, patchset := range user.List.Patches.Results {
		// TODO: filter with API
		if cmd.Name() == "apply" && !patchsetApplicable(patchset.Status) {
			continue
		}

		s := fmt.Sprintf("%d\t", patchset.Id)
		if patchset.Prefix != nil && *patchset.Prefix != "" {
			s += fmt.Sprintf("[%s] ", *patchset.Prefix)
		}
		s += patchset.Subject
		if patchset.Version != 1 {
			s += fmt.Sprintf(" v%d", patchset.Version)
		}
		patchList = append(patchList, s)
	}

	return patchList, cobra.ShellCompDirectiveNoFileComp
}

func patchsetApplicable(status listssrht.PatchsetStatus) bool {
	switch status {
	case listssrht.PatchsetStatusApplied, listssrht.PatchsetStatusNeedsRevision, listssrht.PatchsetStatusSuperseded, listssrht.PatchsetStatusRejected:
		return false
	default:
		return true
	}
}

func completeListsUserWebhookEvents(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var eventList []string
	events := [5]string{"list_created", "list_updated", "list_deleted", "email_received", "patchset_received"}
	set := strings.ToLower(cmd.Flag("events").Value.String())
	for _, event := range events {
		if !strings.Contains(set, event) {
			eventList = append(eventList, event)
		}
	}
	return eventList, cobra.ShellCompDirectiveNoFileComp
}

func completeListsUserWebhookID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("lists", cmd)
	var webhookList []string

	webhooks, err := listssrht.UserWebhooks(c.Client, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, webhook := range webhooks.Results {
		s := fmt.Sprintf("%d\t%s", webhook.Id, webhook.Url)
		webhookList = append(webhookList, s)
	}

	return webhookList, cobra.ShellCompDirectiveNoFileComp
}
