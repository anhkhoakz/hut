package main

import (
	"fmt"
	"log"
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
	return cmd
}

func newListsDeleteCommand() *cobra.Command {
	var autoConfirm bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)

		list, err := listssrht.MailingListIDByName(c.Client, ctx, args[0])
		if err != nil {
			log.Fatalf("failed to get list ID: %v", err)
		} else if list == nil {
			log.Fatalf("mailing list %s does not exist", args[0])
		}

		if !autoConfirm && !getConfirmation(fmt.Sprintf("Do you really want to delete the list %s", args[0])) {
			fmt.Println("Aborted")
			return
		}

		list, err = listssrht.DeleteMailingList(c.Client, ctx, list.Id)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Deleted mailing list %s\n", list.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete <list>",
		Short:             "Delete a mailing list",
		Args:              cobra.ExactArgs(1),
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

		name, owner, instance := parseMailingListName(cmd, args[0])
		c := createClientWithInstance("lists", cmd, instance)

		list, err := listssrht.MailingListIDByOwner(c.Client, ctx, owner, name)
		if err != nil {
			log.Fatal(err)
		} else if list == nil {
			log.Fatalf("no such list %s/%s/%s", c.BaseURL, owner, name)
		}

		subscription, err := listssrht.MailingListSubscribe(c.Client, ctx, list.Id)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Subscribed to %s/%s/%s\n", c.BaseURL, subscription.List.Owner.CanonicalName, subscription.List.Name)
	}

	cmd := &cobra.Command{
		Use:               "subscribe <list>",
		Short:             "Subscribe to a mailing list",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringP("owner", "o", "", "list owner (canonical form)")
	cmd.RegisterFlagCompletionFunc("owner", cobra.NoFileCompletions)
	return cmd
}

func newListsUnsubscribeCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		name, owner, instance := parseMailingListName(cmd, args[0])
		c := createClientWithInstance("lists", cmd, instance)

		list, err := listssrht.MailingListIDByOwner(c.Client, ctx, owner, name)
		if err != nil {
			log.Fatal(err)
		} else if list == nil {
			log.Fatalf("no such list %s/%s/%s", c.BaseURL, owner, name)
		}

		subscription, err := listssrht.MailingListUnsubscribe(c.Client, ctx, list.Id)
		if err != nil {
			log.Fatal(err)
		} else if subscription == nil {
			log.Fatalf("you were not subscribed to %s/%s/%s", c.BaseURL, owner, args[0])
		}

		fmt.Printf("Unsubscribed from %s/%s/%s\n", c.BaseURL, subscription.List.Owner.CanonicalName, subscription.List.Name)
	}

	cmd := &cobra.Command{
		Use:               "unsubscribe <list>",
		Short:             "Unubscribe from a mailing list",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringP("owner", "o", "", "list owner (canonical form)")
	cmd.RegisterFlagCompletionFunc("owner", cobra.NoFileCompletions)
	return cmd
}

func parseMailingListName(cmd *cobra.Command, s string) (name, owner, instance string) {
	name, owner, instance = parseResourceName(s)

	if ownerFlag, err := cmd.Flags().GetString("owner"); err != nil {
		log.Fatal(err)
	} else if ownerFlag != "" {
		if owner != "" && ownerFlag != owner {
			log.Fatalf("conflicting owners: %v and --owner=%v", owner, ownerFlag)
		}
		owner = ownerFlag
	}

	return name, owner, instance
}
