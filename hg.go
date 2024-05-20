package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"git.sr.ht/~xenrox/hut/srht/hgsrht"
	"git.sr.ht/~xenrox/hut/termfmt"
	"github.com/spf13/cobra"
)

func newHgCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hg",
		Short: "Use the hg API",
	}
	cmd.AddCommand(newHgListCommand())
	cmd.AddCommand(newHgCreateCommand())
	cmd.AddCommand(newHgDeleteCommand())
	cmd.AddCommand(newHgUpdateCommand())
	cmd.AddCommand(newHgUserWebhookCommand())
	return cmd
}

func newHgListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("hg", cmd)
		var cursor *hgsrht.Cursor
		var username string
		if len(args) > 0 {
			username = strings.TrimLeft(args[0], ownerPrefixes)
		}

		err := pagerify(func(p pager) error {
			var repos *hgsrht.RepositoryCursor
			if len(username) > 0 {
				user, err := hgsrht.RepositoriesByUser(c.Client, ctx, username, cursor)
				if err != nil {
					return err
				} else if user == nil {
					return errors.New("no such user")
				}
				repos = user.Repositories
			} else {
				var err error
				repos, err = hgsrht.Repositories(c.Client, ctx, cursor)
				if err != nil {
					return err
				}
			}

			for _, repo := range repos.Results {
				printHgRepo(p, repo)
			}

			cursor = repos.Cursor
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
		Use:   "list [user]",
		Short: "List repos",
		Args:  cobra.MaximumNArgs(1),
		Run:   run,
	}
	return cmd
}

func printHgRepo(w io.Writer, repo *hgsrht.Repository) {
	fmt.Fprintf(w, "%s (%s)\n", termfmt.Bold.String(repo.Name), repo.Visibility.TermString())
	if repo.Description != nil && *repo.Description != "" {
		fmt.Fprintf(w, "  %s\n", *repo.Description)
	}
	fmt.Fprintln(w)
}

func newHgCreateCommand() *cobra.Command {
	var visibility, desc string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("hg", cmd)

		hgVisibility, err := hgsrht.ParseVisibility(visibility)
		if err != nil {
			log.Fatal(err)
		}

		repo, err := hgsrht.CreateRepository(c.Client, ctx, args[0], hgVisibility, desc)
		if err != nil {
			log.Fatal(err)
		} else if repo == nil {
			log.Fatal("failed to create repository")
		}

		log.Printf("Created repository %s\n", repo.Name)
	}

	cmd := &cobra.Command{
		Use:               "create <name>",
		Short:             "Create a repository",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringVarP(&visibility, "visibility", "v", "public", "repo visibility")
	cmd.RegisterFlagCompletionFunc("visibility", completeVisibility)
	cmd.Flags().StringVarP(&desc, "description", "d", "", "repo description")
	cmd.RegisterFlagCompletionFunc("description", cobra.NoFileCompletions)
	return cmd
}

func newHgDeleteCommand() *cobra.Command {
	var autoConfirm bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		name, owner, instance := parseResourceName(args[0])

		c := createClientWithInstance("hg", cmd, instance)
		id := getHgRepoID(c, ctx, name, owner)

		if !autoConfirm && !getConfirmation(fmt.Sprintf("Do you really want to delete the repo %s", name)) {
			log.Println("Aborted")
			return
		}
		repo, err := hgsrht.DeleteRepository(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted repository %s\n", repo.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete <repo>",
		Short:             "Delete a repository",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "auto confirm")
	return cmd
}

func newHgUpdateCommand() *cobra.Command {
	var readme string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		name, owner, instance := parseResourceName(args[0])

		c := createClientWithInstance("hg", cmd, instance)
		id := getHgRepoID(c, ctx, name, owner)

		var input hgsrht.RepoInput

		if readme == "" && cmd.Flags().Changed("readme") {
			_, err := hgsrht.ClearCustomReadme(c.Client, ctx, id)
			if err != nil {
				log.Fatalf("failed to unset custom README: %v", err)
			}
		} else if readme != "" {
			var (
				b   []byte
				err error
			)

			if readme == "-" {
				b, err = io.ReadAll(os.Stdin)
			} else {
				b, err = os.ReadFile(readme)
			}
			if err != nil {
				log.Fatalf("failed to read custom README: %v", err)
			}

			s := string(b)
			input.Readme = &s
		}

		repo, err := hgsrht.UpdateRepository(c.Client, ctx, id, input)
		if err != nil {
			log.Fatal(err)
		} else if repo == nil {
			log.Fatalf("failed to update repository %q", name)
		}

		log.Printf("Successfully updated repository %q\n", repo.Name)
	}
	cmd := &cobra.Command{
		Use:               "update <repo>",
		Short:             "Update a repository",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringVar(&readme, "readme", "", "update the custom README")
	return cmd
}

func newHgUserWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user-webhook",
		Short: "Manage user webhooks",
	}
	cmd.AddCommand(newHgUserWebhookCreateCommand())
	cmd.AddCommand(newHgUserWebhookListCommand())
	cmd.AddCommand(newHgUserWebhookDeleteCommand())
	return cmd
}

func newHgUserWebhookCreateCommand() *cobra.Command {
	var events []string
	var stdin bool
	var url string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("hg", cmd)

		var config hgsrht.UserWebhookInput
		config.Url = url

		whEvents, err := hgsrht.ParseUserEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents
		config.Query = readWebhookQuery(stdin)

		webhook, err := hgsrht.CreateUserWebhook(c.Client, ctx, config)
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
	cmd.RegisterFlagCompletionFunc("events", completeHgUserWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newHgUserWebhookListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("hg", cmd)
		var cursor *hgsrht.Cursor

		err := pagerify(func(p pager) error {
			webhooks, err := hgsrht.UserWebhooks(c.Client, ctx, cursor)
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

func newHgUserWebhookDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("hg", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		webhook, err := hgsrht.DeleteUserWebhook(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a user webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHgUserWebhookID,
		Run:               run,
	}
	return cmd
}

func getHgRepoID(c *Client, ctx context.Context, name, owner string) int32 {
	var (
		user     *hgsrht.User
		username string
		err      error
	)
	if owner == "" {
		user, err = hgsrht.RepositoryIDByName(c.Client, ctx, name)
	} else {
		username = strings.TrimLeft(owner, ownerPrefixes)
		user, err = hgsrht.RepositoryIDByUser(c.Client, ctx, username, name)
	}
	if err != nil {
		log.Fatalf("failed to get repository ID: %v", err)
	} else if user == nil {
		log.Fatalf("no such user %q", username)
	} else if user.Repository == nil {
		log.Fatalf("no such repository %q", name)
	}
	return user.Repository.Id
}

func completeHgUserWebhookEvents(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var eventList []string
	events := [3]string{"repo_created", "repo_update", "repo_deleted"}
	set := strings.ToLower(cmd.Flag("events").Value.String())
	for _, event := range events {
		if !strings.Contains(set, event) {
			eventList = append(eventList, event)
		}
	}
	return eventList, cobra.ShellCompDirectiveNoFileComp
}

func completeHgUserWebhookID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("hg", cmd)
	var webhookList []string

	webhooks, err := hgsrht.UserWebhooks(c.Client, ctx, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, webhook := range webhooks.Results {
		s := fmt.Sprintf("%d\t%s", webhook.Id, webhook.Url)
		webhookList = append(webhookList, s)
	}

	return webhookList, cobra.ShellCompDirectiveNoFileComp
}
