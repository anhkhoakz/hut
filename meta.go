package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"

	"git.sr.ht/~emersion/hut/srht/metasrht"
)

func newMetaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meta",
		Short: "Use the meta API",
	}
	cmd.AddCommand(newMetaShowCommand())
	return cmd
}

func newMetaShowCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		username := strings.TrimLeft(args[0], "~")

		ctx := cmd.Context()
		c := createClient("meta")

		user, err := metasrht.FetchUser(c.Client, ctx, username)
		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatal("no such user")
		}

		fmt.Printf("%v <%v>\n", user.CanonicalName, user.Email)
		if user.Url != nil {
			fmt.Println(*user.Url)
		}
		if user.Location != nil {
			fmt.Println(*user.Location)
		}
		if user.Bio != nil {
			fmt.Printf("\n%v\n", *user.Bio)
		}
	}

	cmd := &cobra.Command{
		Use:   "show <user>",
		Short: "Show a user profile",
		Args:  cobra.ExactArgs(1),
		Run:   run,
	}
	return cmd
}
