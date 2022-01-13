package main

import (
	"fmt"
	"log"
	"strings"

	"git.sr.ht/~emersion/hut/srht/hgsrht"
	"git.sr.ht/~emersion/hut/termfmt"
	"github.com/spf13/cobra"
)

func newHgCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hg",
		Short: "Use the hg API",
	}
	cmd.AddCommand(newHgListCommand())
	return cmd
}

func newHgListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("hg", cmd)

		var repos *hgsrht.RepositoryCursor

		if len(args) > 0 {
			username := strings.TrimLeft(args[0], ownerPrefixes)
			user, err := hgsrht.RepositoriesByUser(c.Client, ctx, username)
			if err != nil {
				log.Fatal(err)
			} else if user == nil {
				log.Fatal("no such user")
			}
			repos = user.Repositories
		} else {
			var err error
			repos, err = hgsrht.Repositories(c.Client, ctx)
			if err != nil {
				log.Fatal(err)
			}
		}

		for _, repo := range repos.Results {
			fmt.Printf("%s %s (%s)\n", termfmt.DarkYellow.Sprintf("#%d", repo.Id), termfmt.Bold.String(repo.Name), repo.Visibility.TermString())
			if repo.Description != nil && *repo.Description != "" {
				fmt.Printf("  %s\n", *repo.Description)
			}
			fmt.Println()
		}
	}

	cmd := &cobra.Command{
		Use:   "list [user]",
		Short: "List repos",
		Args:  cobra.MaximumNArgs(1),
		Run:   run,
	}
	return cmd
}
