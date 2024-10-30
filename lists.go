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

	"github.com/dustin/go-humanize"

	"github.com/spf13/cobra"

	"git.sr.ht/~xenrox/hut/srht/listssrht"
	"git.sr.ht/~xenrox/hut/termfmt"
)

func newListsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lists",
		Short: "Use the lists API",
	}
	cmd.AddCommand(newListsDeleteCommand())
	cmd.AddCommand(newListsListCommand())
	cmd.AddCommand(newListsUpdateCommand())
	cmd.AddCommand(newListsSubscribeCommand())
	cmd.AddCommand(newListsUnsubscribeCommand())
	cmd.AddCommand(newListsCreateCommand())
	cmd.AddCommand(newListsArchiveCommand())
	cmd.AddCommand(newListsPatchsetCommand())
	cmd.AddCommand(newListsACLCommand())
	cmd.AddCommand(newListsUserWebhookCommand())
	cmd.AddCommand(newListsWebhookCommand())
	cmd.AddCommand(newListsSubscriptions())
	cmd.PersistentFlags().StringP("mailing-list", "l", "", "mailing list name")
	cmd.RegisterFlagCompletionFunc("mailing-list", completeList)
	return cmd
}

func newListsDeleteCommand() *cobra.Command {
	var autoConfirm bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseMailingListName(args[0])
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
			log.Println("Aborted")
			return
		}

		list, err := listssrht.DeleteMailingList(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if list == nil {
			log.Fatalf("failed to delete list with ID %d", id)
		}

		log.Printf("Deleted mailing list %s\n", list.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete [list]",
		Short:             "Delete a mailing list",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeList,
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

		err := pagerify(func(p pager) error {
			var lists *listssrht.MailingListCursor
			if len(username) > 0 {
				user, err := listssrht.MailingListsByUser(c.Client, ctx, username, cursor)
				if err != nil {
					return err
				} else if user == nil {
					return errors.New("no such user")
				}
				lists = user.Lists
			} else {
				var err error
				user, err := listssrht.MailingLists(c.Client, ctx, cursor)
				if err != nil {
					return err
				}
				lists = user.Lists
			}

			for _, list := range lists.Results {
				printList(p, &list)
			}

			cursor = lists.Cursor
			if cursor == nil {
				return pagerDone
			}
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	return cmd
}

func printList(w io.Writer, list *listssrht.MailingList) {
	fmt.Fprintf(w, "%s (%s)\n", termfmt.Bold.String(list.Name), list.Visibility.TermString())
	if list.Description != nil && *list.Description != "" {
		fmt.Fprintln(w, "\n"+indent(*list.Description, "  ")+"\n")
	}
}

func newListsUpdateCommand() *cobra.Command {
	var visibility string
	var description bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseMailingListName(args[0])
		} else {
			var err error
			name, owner, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("lists", cmd, instance)
		id := getMailingListID(c, ctx, name, owner)
		// TODO: Support permitMime, rejectMime
		var input listssrht.MailingListInput

		if visibility != "" {
			listVisibility, err := listssrht.ParseVisibility(visibility)
			if err != nil {
				log.Fatal(err)
			}
			input.Visibility = &listVisibility
		}

		if description {
			if !isStdinTerminal {
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					log.Fatalf("failed to read description: %v", err)
				}
				description := string(b)
				input.Description = &description
			} else {
				var (
					err      error
					user     *listssrht.User
					username string
				)

				if owner != "" {
					username = strings.TrimLeft(owner, ownerPrefixes)
					user, err = listssrht.MailingListDescriptionByUser(c.Client, ctx, username, name)
				} else {
					user, err = listssrht.MailingListDescription(c.Client, ctx, name)
				}

				if err != nil {
					log.Fatalf("failed to fetch description: %v", err)
				} else if user == nil {
					log.Fatalf("no such user %q", username)
				} else if user.List == nil {
					log.Fatalf("no such mailing list %q", name)
				}

				var prefill string
				if user.List.Description != nil {
					prefill = *user.List.Description
				}

				text, err := getInputWithEditor("hut_description*.md", prefill)
				if err != nil {
					log.Fatalf("failed to read description: %v", err)
				}

				if strings.TrimSpace(text) == "" {
					_, err := listssrht.ClearDescription(c.Client, ctx, id)
					if err != nil {
						log.Fatalf("failed to clear description: %v", err)
					}
				} else {
					input.Description = &text
				}
			}
		}

		_, err := listssrht.UpdateMailingList(c.Client, ctx, id, input)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Updated mailing list\n")
	}

	cmd := &cobra.Command{
		Use:               "update [list]",
		Short:             "Update a mailing list",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeList,
		Run:               run,
	}
	cmd.Flags().StringVarP(&visibility, "visibility", "v", "", "mailing list visibility")
	cmd.RegisterFlagCompletionFunc("visibility", completeVisibility)
	cmd.Flags().BoolVar(&description, "description", false, "edit description")
	return cmd
}

func newListsSubscribeCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseMailingListName(args[0])
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

		log.Printf("Subscribed to %s/%s/%s\n", c.BaseURL, subscription.List.Owner.CanonicalName, subscription.List.Name)
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
			name, owner, instance = parseMailingListName(args[0])
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

		log.Printf("Unsubscribed from %s/%s/%s\n", c.BaseURL, subscription.List.Owner.CanonicalName, subscription.List.Name)
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

		list, err := listssrht.CreateMailingList(c.Client, ctx, args[0], description, listVisibility)
		if err != nil {
			log.Fatal(err)
		} else if list == nil {
			log.Fatal("failed to create mailing list")
		}

		log.Printf("Created mailing list %q\n", list.Name)
	}
	cmd := &cobra.Command{
		Use:               "create <name>",
		Short:             "Create a mailing list",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().BoolVar(&stdin, "stdin", !isStdinTerminal, "read mailing list from stdin")
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
			name, owner, instance = parseMailingListName(args[0])
		} else {
			var err error
			name, owner, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("lists", cmd, instance)

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

		c.HTTP.Timeout = fileTransferTimeout
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
		ValidArgsFunction: completeList,
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
			name, owner, instance = parseMailingListName(args[0])
		} else {
			var err error
			name, owner, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}
		c := createClientWithInstance("lists", cmd, instance)

		var (
			cursor   *listssrht.Cursor
			patches  *listssrht.PatchsetCursor
			user     *listssrht.User
			username string
			err      error
		)

		if byUser {
			if len(args) > 0 {
				username = strings.TrimLeft(name, ownerPrefixes)
			}
		} else {
			if owner != "" {
				username = strings.TrimLeft(owner, ownerPrefixes)
			}
		}

		err = pagerify(func(p pager) error {
			if byUser {
				if username != "" {
					user, err = listssrht.PatchesByUser(c.Client, ctx, name, cursor)
				} else {
					user, err = listssrht.Patches(c.Client, ctx, cursor)
				}

				if err != nil {
					return err
				} else if user == nil {
					return fmt.Errorf("no such user %q", name)
				}

				patches = user.Patches
			} else {
				if username != "" {
					user, err = listssrht.ListPatchesByUser(c.Client, ctx, username, name, cursor)
				} else {
					user, err = listssrht.ListPatches(c.Client, ctx, name, cursor)
				}

				if err != nil {
					return err
				} else if user == nil {
					return fmt.Errorf("no such user %q", username)
				} else if user.List == nil {
					return fmt.Errorf("no such list %q", name)
				}

				patches = user.List.Patches
			}

			for _, patchset := range patches.Results {
				printPatchset(p, byUser, &patchset)
			}

			cursor = patches.Cursor
			if cursor == nil {
				return pagerDone
			}

			return nil
		})
		if err != nil {
			log.Fatal(err)
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

func printPatchset(w io.Writer, byUser bool, patchset *listssrht.Patchset) {
	s := fmt.Sprintf("%s\t%s\t", termfmt.DarkYellow.Sprintf("#%d", patchset.Id), patchset.Status.TermString())
	if patchset.Prefix != nil && *patchset.Prefix != "" {
		s += fmt.Sprintf("[%s] ", strings.TrimSpace(*patchset.Prefix))
	}
	s += patchset.Subject
	if patchset.Version != 1 {
		s += fmt.Sprintf(" v%d", patchset.Version)
	}

	created := termfmt.Dim.String(humanize.Time(patchset.Created.Time))

	if byUser {
		s += fmt.Sprintf("\t%s/%s\t%s", patchset.List.Owner.CanonicalName,
			patchset.List.Name, created)
	} else {
		s += fmt.Sprintf("\t%s\t%s", patchset.Submitter.CanonicalName, created)
	}
	fmt.Fprintln(w, s)
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

		log.Printf("Updated patchset %q by %s\n", patch.Subject, patch.Submitter.CanonicalName)
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
		var cursor *listssrht.Cursor

		for {
			patchset, err := listssrht.PatchsetById(c.Client, ctx, id, cursor)
			if err != nil {
				log.Fatal(err)
			} else if patchset == nil {
				log.Fatalf("no such patchset %d", id)
			}

			for _, patch := range patchset.Patches.Results {
				formatPatch(os.Stdout, &patch)
			}

			cursor = patchset.Patches.Cursor
			if cursor == nil {
				break
			}
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
		var cursor *listssrht.Cursor
		var mbox bytes.Buffer

		for {
			patchset, err := listssrht.PatchsetById(c.Client, ctx, id, cursor)
			if err != nil {
				log.Fatal(err)
			} else if patchset == nil {
				log.Fatalf("no such patchset %d", id)
			}

			for _, patch := range patchset.Patches.Results {
				formatPatch(&mbox, &patch)
			}

			cursor = patchset.Patches.Cursor
			if cursor == nil {
				break
			}
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

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseMailingListName(args[0])
		} else {
			var err error
			name, owner, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("lists", cmd, instance)
		var (
			cursor   *listssrht.Cursor
			user     *listssrht.User
			username string
			err      error
		)
		if owner != "" {
			username = strings.TrimLeft(owner, ownerPrefixes)
		}

		err = pagerify(func(p pager) error {
			if username != "" {
				user, err = listssrht.AclByUser(c.Client, ctx, username, name, cursor)
			} else {
				user, err = listssrht.AclByListName(c.Client, ctx, name, cursor)
			}

			if err != nil {
				return err
			} else if user == nil {
				return fmt.Errorf("no such user %q", username)
			} else if user.List == nil {
				return fmt.Errorf("no such list %q", name)
			}

			if cursor == nil {
				// only print once
				fmt.Fprintln(p, termfmt.Bold.Sprint("Default permissions"))
				fmt.Fprintln(p, user.List.DefaultACL.TermString())

				if len(user.List.Acl.Results) > 0 {
					fmt.Fprintln(p, termfmt.Bold.Sprint("\nUser permissions"))
				}
			}

			for _, acl := range user.List.Acl.Results {
				printListsACLEntry(p, &acl)
			}

			cursor = user.List.Acl.Cursor
			if cursor == nil {
				return pagerDone
			}

			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	cmd := &cobra.Command{
		Use:               "list",
		Short:             "List ACL entries",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeList,
		Run:               run,
	}
	return cmd
}

func printListsACLEntry(w io.Writer, acl *listssrht.MailingListACL) {
	s := fmt.Sprintf("%s browse  %s reply  %s post  %s moderate",
		listssrht.PermissionIcon(acl.Browse), listssrht.PermissionIcon(acl.Reply),
		listssrht.PermissionIcon(acl.Post), listssrht.PermissionIcon(acl.Moderate))
	created := termfmt.Dim.String(humanize.Time(acl.Created.Time))
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", termfmt.DarkYellow.Sprintf("#%d", acl.Id),
		acl.Entity.CanonicalName, s, created)
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

		log.Printf("Deleted ACL entry for %q in mailing list %q\n", acl.Entity.CanonicalName, acl.List.Name)
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
	cmd.RegisterFlagCompletionFunc("events", completeListsUserWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", !isStdinTerminal, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newListsUserWebhookListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)
		var cursor *listssrht.Cursor

		err := pagerify(func(p pager) error {
			webhooks, err := listssrht.UserWebhooks(c.Client, ctx, cursor)
			if err != nil {
				return err
			}

			for _, webhook := range webhooks.Results {
				fmt.Fprintf(p, "%s %s\n", termfmt.DarkYellow.Sprintf("#%d", webhook.Id), webhook.Url)
			}

			cursor = webhooks.Cursor
			if cursor == nil {
				return pagerDone
			}

			return nil
		})
		if err != nil {
			log.Fatal(err)
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
		}

		log.Printf("Deleted webhook %d\n", webhook.Id)
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

func newListsWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage mailing list webhooks",
	}
	cmd.AddCommand(newListsWebhookCreateCommand())
	cmd.AddCommand(newListsWebhookListCommand())
	cmd.AddCommand(newListsWebhookDeleteCommand())
	return cmd
}

func newListsWebhookCreateCommand() *cobra.Command {
	var events []string
	var stdin bool
	var url string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseMailingListName(args[0])
		} else {
			var err error
			name, owner, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("lists", cmd, instance)
		id := getMailingListID(c, ctx, name, owner)

		var config listssrht.MailingListWebhookInput
		config.Url = url

		whEvents, err := listssrht.ParseMailingListWebhookEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents
		config.Query = readWebhookQuery(stdin)

		webhook, err := listssrht.CreateMailingListWebhook(c.Client, ctx, id, config)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Created mailing list webhook with ID %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "create [list]",
		Short:             "Create a mailing list webhook",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeList,
		Run:               run,
	}
	cmd.Flags().StringSliceVarP(&events, "events", "e", nil, "webhook events")
	cmd.RegisterFlagCompletionFunc("events", completeMailingListWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", !isStdinTerminal, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newListsWebhookListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseMailingListName(args[0])
		} else {
			var err error
			name, owner, instance, err = getMailingListName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("lists", cmd, instance)

		var (
			cursor   *listssrht.Cursor
			user     *listssrht.User
			username string
			err      error
		)
		if owner != "" {
			username = strings.TrimLeft(owner, ownerPrefixes)
		}

		err = pagerify(func(p pager) error {
			if username != "" {
				user, err = listssrht.MailingListWebhooksByUser(c.Client, ctx, username, name, cursor)
			} else {
				user, err = listssrht.MailingListWebhooks(c.Client, ctx, name, cursor)
			}

			if err != nil {
				return err
			} else if user == nil {
				return fmt.Errorf("no such user %q", username)
			} else if user.List == nil {
				return fmt.Errorf("no such mailing list %q", name)
			}

			for _, webhook := range user.List.Webhooks.Results {
				fmt.Fprintf(p, "%s %s\n", termfmt.DarkYellow.Sprintf("#%d", webhook.Id), webhook.Url)
			}

			cursor = user.List.Webhooks.Cursor
			if cursor == nil {
				return pagerDone
			}

			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	cmd := &cobra.Command{
		Use:               "list [list]",
		Short:             "List mailing list webhooks",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeList,
		Run:               run,
	}
	return cmd
}

func newListsWebhookDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		webhook, err := listssrht.DeleteMailingListWebhook(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a mailing list webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newListsSubscriptions() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)
		var cursor *listssrht.Cursor

		err := pagerify(func(p pager) error {
			subscriptions, err := listssrht.Subscriptions(c.Client, ctx, cursor)
			if err != nil {
				return err
			}

			for _, sub := range subscriptions.Results {
				printMailingListSubscription(p, &sub)
			}

			cursor = subscriptions.Cursor
			if cursor == nil {
				return pagerDone
			}

			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "List mailing list subscriptions",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	return cmd
}

func printMailingListSubscription(w io.Writer, sub *listssrht.ActivitySubscription) {
	mlSub, ok := sub.Value.(*listssrht.MailingListSubscription)
	if !ok {
		return
	}

	created := termfmt.Dim.String(humanize.Time(sub.Created.Time))
	fmt.Fprintf(w, "%s/%s %s\n", mlSub.List.Owner.CanonicalName, mlSub.List.Name, created)
}

// parseMailingListName parses a mailing list name, following either the
// generic resource name syntax, or "<owner>/<name>@<instance>".
func parseMailingListName(s string) (name, owner, instance string) {
	slash := strings.Index(s, "/")
	at := strings.Index(s, "@")
	if strings.Count(s, "/") != 1 || strings.Count(s, "@") != 1 || at < slash {
		return parseResourceName(s)
	}

	owner = s[:slash]
	name = s[slash+1 : at]
	instance = s[at+1:]
	return name, owner, instance
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
		name, owner, instance = parseMailingListName(s)
		return name, owner, instance, nil
	}

	cfg, err := loadProjectConfig()
	if err != nil {
		return "", "", "", err
	}
	if cfg != nil && cfg.DevList != "" {
		name, owner, instance = parseMailingListName(cfg.DevList)
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

	name, owner, instance = parseMailingListName(addr.Address)
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

		if cmd.Name() == "update" && strings.EqualFold(cmd.Flag("status").Value.String(), string(patchset.Status)) {
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

	webhooks, err := listssrht.UserWebhooks(c.Client, ctx, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, webhook := range webhooks.Results {
		s := fmt.Sprintf("%d\t%s", webhook.Id, webhook.Url)
		webhookList = append(webhookList, s)
	}

	return webhookList, cobra.ShellCompDirectiveNoFileComp
}

func completeList(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("lists", cmd)
	var listsList []string

	user, err := listssrht.CompleteLists(c.Client, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, list := range user.Lists.Results {
		listsList = append(listsList, list.Name)
	}

	return listsList, cobra.ShellCompDirectiveNoFileComp
}

func completeMailingListWebhookEvents(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var eventList []string
	events := [4]string{"list_updated", "list_deleted", "email_received", "patchset_received"}
	set := strings.ToLower(cmd.Flag("events").Value.String())
	for _, event := range events {
		if !strings.Contains(set, event) {
			eventList = append(eventList, event)
		}
	}
	return eventList, cobra.ShellCompDirectiveNoFileComp
}
