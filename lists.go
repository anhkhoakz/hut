package main

import (
	"fmt"
	"log"

	"git.sr.ht/~emersion/hut/srht/listssrht"
	"github.com/spf13/cobra"
)

func newListsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lists",
		Short: "Use the lists API",
	}

	cmd.AddCommand(newListsDeleteCommand())
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
