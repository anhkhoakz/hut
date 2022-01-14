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
	return cmd
}

func newListsDeleteCommand() *cobra.Command {
	var autoConfirm bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)

		list, err := listssrht.MailingListByName(c.Client, ctx, args[0])
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
	// TODO: Parse owner from argument
	var owner string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("lists", cmd)

		list, err := listssrht.MailingListIDByOwner(c.Client, ctx, owner, args[0])
		if err != nil {
			log.Fatal(err)
		} else if list == nil {
			log.Fatalf("no such list %s/%s/%s", c.BaseURL, owner, args[0])
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
	cmd.Flags().StringVarP(&owner, "owner", "o", "", "list owner (canonical form)")
	cmd.RegisterFlagCompletionFunc("owner", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("owner")
	return cmd
}
