package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"git.sr.ht/~xenrox/hut/srht/hgsrht"
	"git.sr.ht/~xenrox/hut/termfmt"
	"github.com/dustin/go-humanize"
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
	cmd.AddCommand(newHgACLCommand())
	cmd.AddCommand(newHgUserWebhookCommand())
	cmd.PersistentFlags().StringP("repo", "r", "", "name of repository")
	cmd.RegisterFlagCompletionFunc("repo", completeHgRepo)
	return cmd
}

func newHgListCommand() *cobra.Command {
	var count int
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
			if p.IsDone(cursor, len(repos.Results)) {
				return pagerDone
			}

			return nil
		}, count)
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
	cmd.Flags().IntVar(&count, "count", 0, "number of repos to fetch")
	cmd.RegisterFlagCompletionFunc("count", cobra.NoFileCompletions)
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
	var clone bool
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

		log.Printf("Created repository %q\n", repo.Name)

		ver, err := hgsrht.SshSettings(c.Client, ctx)
		if err != nil {
			log.Fatalf("failed to retrieve settings: %v", err)
		}

		u, err := url.Parse(c.BaseURL)
		if err != nil {
			log.Fatalf("failed to parse base URL: %v", err)
		}

		cloneURL := fmt.Sprintf("ssh://%s@%s/%s/%s", ver.Settings.SshUser, u.Hostname(),
			repo.Owner.CanonicalName, repo.Name)

		if clone {
			cloneCmd := exec.Command("hg", "clone", cloneURL)
			cloneCmd.Stdin = os.Stdin
			cloneCmd.Stdout = os.Stdout
			cloneCmd.Stderr = os.Stderr

			err = cloneCmd.Run()
			if err != nil {
				log.Fatal(err)
			}
		} else {
			fmt.Printf("%s\n", cloneURL)
		}
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
	cmd.Flags().BoolVarP(&clone, "clone", "c", false, "autoclone repo")
	return cmd
}

func newHgDeleteCommand() *cobra.Command {
	var autoConfirm bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getHgRepoName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("hg", cmd, instance)
		id, err := getHgRepoID(c, ctx, name, owner)
		if err != nil {
			log.Fatal(err)
		}

		if !autoConfirm && !getConfirmation(fmt.Sprintf("Do you really want to delete the repo %s", name)) {
			log.Println("Aborted")
			return
		}
		repo, err := hgsrht.DeleteRepository(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted repository %q\n", repo.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete [repo]",
		Short:             "Delete a repository",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHgRepo,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "auto confirm")
	return cmd
}

func newHgUpdateCommand() *cobra.Command {
	var description, nonPublishing, readme, visibility string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getHgRepoName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("hg", cmd, instance)
		id, err := getHgRepoID(c, ctx, name, owner)
		if err != nil {
			log.Fatal(err)
		}

		var input hgsrht.RepoInput

		if cmd.Flags().Changed("description") {
			if description == "" {
				_, err := hgsrht.ClearDescription(c.Client, ctx, id)
				if err != nil {
					log.Fatalf("failed to clear description: %v", err)
				}
			} else {
				input.Description = &description
			}
		}

		if nonPublishing != "" {
			b, err := strconv.ParseBool(nonPublishing)
			if err != nil {
				log.Fatalf("failure with %q: %v", "non-publishing", err)
			}
			input.NonPublishing = &b
		}

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

		if visibility != "" {
			repoVisibility, err := hgsrht.ParseVisibility(visibility)
			if err != nil {
				log.Fatal(err)
			}
			input.Visibility = &repoVisibility
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
		Use:               "update [repo]",
		Short:             "Update a repository",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHgRepo,
		Run:               run,
	}
	cmd.Flags().StringVarP(&description, "description", "d", "", "repository description")
	cmd.RegisterFlagCompletionFunc("description", cobra.NoFileCompletions)
	cmd.Flags().StringVar(&nonPublishing, "non-publishing", "", "non-publishing repository")
	cmd.RegisterFlagCompletionFunc("non-publishing", completeBoolean)
	cmd.Flags().StringVar(&readme, "readme", "", "update the custom README")
	cmd.Flags().StringVarP(&visibility, "visibility", "v", "", "repository visibility")
	cmd.RegisterFlagCompletionFunc("visibility", completeVisibility)
	return cmd
}

func newHgACLCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acl",
		Short: "Manage access-control lists",
	}
	cmd.AddCommand(newHgACLListCommand())
	cmd.AddCommand(newHgACLUpdateCommand())
	cmd.AddCommand(newHgACLDeleteCommand())
	return cmd
}

func newHgACLListCommand() *cobra.Command {
	var count int
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getHgRepoName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("hg", cmd, instance)
		var (
			cursor   *hgsrht.Cursor
			user     *hgsrht.User
			username string
			err      error
		)
		if owner != "" {
			username = strings.TrimLeft(owner, ownerPrefixes)
		}

		err = pagerify(func(p pager) error {
			if username != "" {
				user, err = hgsrht.AclByUser(c.Client, ctx, username, name, cursor)
			} else {
				user, err = hgsrht.AclByRepoName(c.Client, ctx, name, cursor)
			}

			if err != nil {
				return err
			} else if user == nil {
				return errors.New("no such user")
			} else if user.Repository == nil {
				return fmt.Errorf("no such repository %q", name)
			}

			for _, acl := range user.Repository.AccessControlList.Results {
				printHgACLEntry(p, acl)
			}

			cursor = user.Repository.AccessControlList.Cursor
			if p.IsDone(cursor, len(user.Repository.AccessControlList.Results)) {
				return pagerDone
			}

			return nil
		}, count)
		if err != nil {
			log.Fatal(err)
		}
	}

	cmd := &cobra.Command{
		Use:               "list [repo]",
		Short:             "List ACL entries",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHgRepo,
		Run:               run,
	}
	cmd.Flags().IntVar(&count, "count", 0, "number of ACL entries to fetch")
	cmd.RegisterFlagCompletionFunc("count", cobra.NoFileCompletions)
	return cmd
}

func printHgACLEntry(w io.Writer, acl *hgsrht.ACL) {
	var mode string
	if acl.Mode != nil {
		mode = string(*acl.Mode)
	}

	created := termfmt.Dim.String(humanize.Time(acl.Created.Time))
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", termfmt.DarkYellow.Sprintf("#%d", acl.Id),
		acl.Entity.CanonicalName, mode, created)
}

func newHgACLUpdateCommand() *cobra.Command {
	var mode string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		accessMode, err := hgsrht.ParseAccessMode(mode)
		if err != nil {
			log.Fatal(err)
		}

		if strings.IndexAny(args[0], ownerPrefixes) != 0 {
			log.Fatal("user must be in canonical form")
		}

		name, owner, instance, err := getHgRepoName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("hg", cmd, instance)
		id, err := getHgRepoID(c, ctx, name, owner)
		if err != nil {
			log.Fatal(err)
		}

		acl, err := hgsrht.UpdateACL(c.Client, ctx, id, accessMode, args[0])
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Updated access rights for %s\n", acl.Entity.CanonicalName)
	}

	cmd := &cobra.Command{
		Use:               "update <user>",
		Short:             "Update/add ACL entries",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringVarP(&mode, "mode", "m", "", "access mode")
	cmd.RegisterFlagCompletionFunc("mode", completeRepoAccessMode)
	cmd.MarkFlagRequired("mode")
	return cmd
}

func newHgACLDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("hg", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		acl, err := hgsrht.DeleteACL(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if acl == nil {
			log.Fatalf("failed to delete ACL entry with ID %d", id)
		}

		log.Printf("Deleted ACL entry for %q in repository %q\n", acl.Entity.CanonicalName, acl.Repository.Name)
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
	cmd.Flags().BoolVar(&stdin, "stdin", !isStdinTerminal, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newHgUserWebhookListCommand() *cobra.Command {
	var count int
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
			if p.IsDone(cursor, len(webhooks.Results)) {
				return pagerDone
			}

			return nil
		}, count)
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
	cmd.Flags().IntVar(&count, "count", 0, "number of webhooks to fetch")
	cmd.RegisterFlagCompletionFunc("count", cobra.NoFileCompletions)
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

func getHgRepoName(ctx context.Context, cmd *cobra.Command) (repoName, owner, instance string, err error) {
	repoName, err = cmd.Flags().GetString("repo")
	if err != nil {
		return "", "", "", err
	} else if repoName != "" {
		repoName, owner, instance = parseResourceName(repoName)
		return repoName, owner, instance, nil
	}
	return guessHgRepoName(ctx)
}

func guessHgRepoName(ctx context.Context) (repoName, owner, instance string, err error) {
	remoteURL, err := hgRemoteUrl(ctx)
	if err != nil {
		return "", "", "", err
	}

	parts := strings.Split(strings.Trim(remoteURL.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("failed to parse Hg URL %q: expected 2 path components", remoteURL)
	}
	owner, repoName = parts[0], parts[1]

	// TODO: ignore port in host
	return repoName, owner, remoteURL.Host, nil
}

func hgRemoteUrl(ctx context.Context) (*url.URL, error) {
	out, err := exec.CommandContext(ctx, "hg", "paths", "default").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get remote URL: %v", err)
	}

	raw := strings.TrimSpace(string(out))
	switch {
	case strings.Contains(raw, "://"):
		return url.Parse(raw)
	case strings.HasPrefix(raw, "/"):
		return &url.URL{Scheme: "file", Path: raw}, nil
	default:
		i := strings.Index(raw, ":")
		if i < 0 {
			return nil, fmt.Errorf("invalid scp-like Hg URL %q: missing colon", raw)
		}
		host, path := raw[:i], raw[i+1:]

		// Strip optional user
		if i := strings.Index(host, "@"); i >= 0 {
			host = host[i+1:]
		}

		return &url.URL{Scheme: "ssh", Host: host, Path: path}, nil
	}
}

func getHgRepoID(c *Client, ctx context.Context, name, owner string) (int32, error) {
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
		return 0, fmt.Errorf("failed to get repository ID: %v", err)
	} else if user == nil {
		return 0, fmt.Errorf("no such user %q", username)
	} else if user.Repository == nil {
		return 0, fmt.Errorf("no such repository %q", name)
	}
	return user.Repository.Id, nil
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

func completeHgRepo(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("hg", cmd)
	var repoList []string

	repos, err := hgsrht.CompleteRepositories(c.Client, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, repo := range repos.Results {
		repoList = append(repoList, repo.Name)
	}

	return repoList, cobra.ShellCompDirectiveNoFileComp
}
