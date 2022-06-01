package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/mail"
	"os"
	"os/exec"
	"strconv"
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
	cmd.AddCommand(newListsPatchsetCommand())
	cmd.AddCommand(newListsACLCommand())
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
			name, owner, instance = getMailingListName(ctx, cmd)
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

		var lists *listssrht.MailingListCursor
		if len(args) > 0 {
			username := strings.TrimLeft(args[0], ownerPrefixes)
			user, err := listssrht.MailingListsByUser(c.Client, ctx, username)
			if err != nil {
				log.Fatal(err)
			} else if user == nil {
				log.Fatal("no such user")
			}
			lists = user.Lists
		} else {
			var err error
			lists, err = listssrht.MailingLists(c.Client, ctx)
			if err != nil {
				log.Fatal(err)
			}
		}

		for _, list := range lists.Results {
			fmt.Println(termfmt.Bold.String(list.Name))
			if list.Description != nil && *list.Description != "" {
				fmt.Println("\n" + indent(*list.Description, "  ") + "\n")
			}
		}
	}
	return cmd
}

func newListsSubscribeCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			name, owner, instance = getMailingListName(ctx, cmd)
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
			name, owner, instance = getMailingListName(ctx, cmd)
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
	var stdin bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)

		var description *string

		if stdin {
			b, err := ioutil.ReadAll(os.Stdin)
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

		tracker, err := listssrht.CreateMailingList(c.Client, ctx, args[0], description)
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
			name, owner, instance = getMailingListName(ctx, cmd)
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
				user, err = listssrht.PatchesByMe(c.Client, ctx)
			}

			if err != nil {
				log.Fatal(err)
			} else if user == nil {
				log.Fatalf("no such user %q", name)
			}
			patches = user.Patches
		} else {
			var list *listssrht.MailingList
			if owner != "" {
				list, err = listssrht.PatchesByOwner(c.Client, ctx, owner, name)
			} else {
				list, err = listssrht.Patches(c.Client, ctx, name)
			}

			if err != nil {
				log.Fatal(err)
			} else if list == nil {
				log.Fatalf("no such list %q", name)
			}
			patches = list.Patches
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

		id, instance, err := parsePatchID(args[0])
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

		id, instance, err := parsePatchID(args[0])
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

		id, instance, err := parsePatchID(args[0])
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
			name, _, instance = getMailingListName(ctx, cmd)
		}

		c := createClientWithInstance("lists", cmd, instance)

		list, err := listssrht.AclByListName(c.Client, ctx, name)
		if err != nil {
			log.Fatal(err)
		} else if list == nil {
			log.Fatalf("no such list %q", name)
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Println(termfmt.Bold.Sprint("Global permissions"))
		fmt.Fprintf(tw, "Non-subscriber\t%s\n", list.Nonsubscriber.TermString())
		fmt.Fprintf(tw, "Subscriber\t%s\n", list.Subscriber.TermString())
		fmt.Fprintf(tw, "Account holder\t%s\n", list.Identified.TermString())
		tw.Flush()

		if len(list.Acl.Results) > 0 {
			fmt.Println(termfmt.Bold.Sprint("\nUser permissions"))
		}
		for _, acl := range list.Acl.Results {
			s := fmt.Sprintf("%s browse  %s reply  %s post  %s moderate",
				listssrht.PermissionIcon(acl.Browse), listssrht.PermissionIcon(acl.Reply),
				listssrht.PermissionIcon(acl.Post), listssrht.PermissionIcon(acl.Moderate))
			created := termfmt.Dim.Sprintf("%s ago", timeDelta(acl.Created))
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", termfmt.DarkYellow.Sprintf("#%d", acl.Id),
				acl.Entity.CanonicalName, s, created)
		}
		tw.Flush()
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

func getMailingListID(c *Client, ctx context.Context, name, owner string) int32 {
	var (
		list *listssrht.MailingList
		err  error
	)
	if owner == "" {
		list, err = listssrht.MailingListIDByName(c.Client, ctx, name)
	} else {
		list, err = listssrht.MailingListIDByOwner(c.Client, ctx, owner, name)
	}
	if err != nil {
		log.Fatal(err)
	} else if list == nil {
		if owner == "" {
			log.Fatalf("no such mailing list %s", name)
		}
		log.Fatalf("no such mailing list %s/%s/%s", c.BaseURL, owner, name)
	}
	return list.Id
}

func getMailingListName(ctx context.Context, cmd *cobra.Command) (name, owner, instance string) {
	if s, err := cmd.Flags().GetString("mailing-list"); err != nil {
		log.Fatal(err)
	} else if s != "" {
		return parseResourceName(s)
	}
	return guessMailingListName(ctx)
}

func guessMailingListName(ctx context.Context) (name, owner, instance string) {
	addr, err := getGitSendEmailTo(ctx)
	if err != nil {
		log.Fatal(err)
	} else if addr == nil {
		log.Fatal("no mailing list specified and no mailing list configured for current Git repository")
	}

	parts := strings.SplitN(addr.Address, "@", 2)
	if len(parts) != 2 {
		log.Fatalf("invalid email address %q", addr.Address)
	}

	name, owner, _ = parseResourceName(parts[0])
	instance = parts[1]
	return name, owner, instance
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

func parsePatchID(s string) (id int32, instance string, err error) {
	s, _, instance = parseResourceName(s)
	split := strings.Split(s, "/")
	s = split[len(split)-1]
	id64, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, "", fmt.Errorf("invalid patchset ID: %v", err)
	}

	return int32(id64), instance, nil
}

func completePatchsetStatus(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"unknown", "proposed", "needs_revision", "superseded",
		"approved", "rejected", "applied"}, cobra.ShellCompDirectiveNoFileComp
}

func completePatchsetID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	var patchList []string

	name, owner, instance := getMailingListName(ctx, cmd)
	c := createClientWithInstance("lists", cmd, instance)

	var (
		err  error
		list *listssrht.MailingList
	)
	if owner != "" {
		list, err = listssrht.CompletePatchsetIdByOwner(c.Client, ctx, owner, name)
	} else {
		list, err = listssrht.CompletePatchsetId(c.Client, ctx, name)
	}

	if err != nil || list == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, patchset := range list.Patches.Results {
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
