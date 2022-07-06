package main

import (
	"context"
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
	cmd.AddCommand(newHgCreateCommand())
	cmd.AddCommand(newHgDeleteCommand())
	cmd.AddCommand(newHgUserWebhookCommand())
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
			fmt.Printf("%s (%s)\n", termfmt.Bold.String(repo.Name), repo.Visibility.TermString())
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

		fmt.Printf("Created repository %s\n", repo.Name)
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
			fmt.Println("Aborted")
			return
		}
		repo, err := hgsrht.DeleteRepository(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Deleted repository %s\n", repo.Name)
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

		webhooks, err := hgsrht.UserWebhooks(c.Client, ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, webhook := range webhooks.Results {
			fmt.Printf("%s %s %s\n", termfmt.DarkYellow.Sprintf("#%d", webhook.Id),
				webhook.Url, webhook.Events)
			fmt.Println(indent(webhook.Query, "  "))
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
		} else if webhook == nil {
			log.Fatal("failed to delete webhook")
		}

		fmt.Printf("Deleted webhook %d\n", webhook.Id)
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

	webhooks, err := hgsrht.CompleteUserWebhookId(c.Client, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, webhook := range webhooks.Results {
		s := fmt.Sprintf("%d\t%s", webhook.Id, webhook.Url)
		webhookList = append(webhookList, s)
	}

	return webhookList, cobra.ShellCompDirectiveNoFileComp
}
