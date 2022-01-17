package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/mail"
	"os/exec"
	"strings"

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
	cmd.AddCommand(newListsPatchsetCommand())
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
			log.Fatalf("you were not subscribed to %s/%s/%s", c.BaseURL, owner, args[0])
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

func newListsPatchsetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patchset",
		Short: "Manage patchsets",
	}
	cmd.AddCommand(newListsPatchsetListCommand())
	return cmd
}

func newListsPatchsetListCommand() *cobra.Command {
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
			err  error
			list *listssrht.MailingList
		)

		if owner != "" {
			list, err = listssrht.PatchesByOwner(c.Client, ctx, owner, name)
		} else {
			list, err = listssrht.Patches(c.Client, ctx, name)
		}

		if err != nil {
			log.Fatal(err)
		} else if list == nil {
			log.Fatal("no such list")
		}

		for _, patchset := range list.Patches.Results {
			// TODO: Improve formatting (version)
			fmt.Printf("%s %s %s (%s %s ago)\n", termfmt.DarkYellow.Sprintf("#%d", patchset.Id),
				patchset.Status.TermString(), patchset.Subject, patchset.Submitter.CanonicalName,
				timeDelta(patchset.Created))
		}
	}

	cmd := &cobra.Command{
		Use:               "list [list]",
		Short:             "List patchsets",
		Args:              cobra.MaximumNArgs(1),
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
