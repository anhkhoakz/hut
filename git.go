package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.sr.ht/~emersion/gqlclient"
	"github.com/juju/ansiterm/tabwriter"
	"github.com/spf13/cobra"

	"git.sr.ht/~emersion/hut/srht/gitsrht"
	"git.sr.ht/~emersion/hut/termfmt"
)

func newGitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "Use the git API",
	}
	cmd.AddCommand(newGitArtifactCommand())
	cmd.AddCommand(newGitCreateCommand())
	cmd.AddCommand(newGitListCommand())
	cmd.AddCommand(newGitDeleteCommand())
	cmd.AddCommand(newGitACLCommand())
	cmd.AddCommand(newGitShowCommand())
	cmd.AddCommand(newGitUserWebhookCommand())
	cmd.PersistentFlags().StringP("repo", "r", "", "name of repository")
	cmd.RegisterFlagCompletionFunc("repo", completeRepo)
	return cmd
}

func newGitCreateCommand() *cobra.Command {
	var visibility, desc, importURL string
	var clone bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git", cmd)

		gitVisibility, err := gitsrht.ParseVisibility(visibility)
		if err != nil {
			log.Fatal(err)
		}

		var importURLPtr *string
		if importURL != "" {
			importURLPtr = &importURL
		}

		repo, err := gitsrht.CreateRepository(c.Client, ctx, args[0],
			gitVisibility, desc, importURLPtr)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Created repository %s\n", repo.Name)

		if clone {
			ver, err := gitsrht.SshSettings(c.Client, ctx)
			if err != nil {
				log.Fatalf("failed to retrieve settings: %v", err)
			}

			u, err := url.Parse(c.BaseURL)
			if err != nil {
				log.Fatalf("failed to parse base URL: %v", err)
			}

			cloneURL := fmt.Sprintf("%s@%s:%s/%s", ver.Settings.SshUser, u.Hostname(),
				repo.Owner.CanonicalName, repo.Name)
			cloneCmd := exec.Command("git", "clone", cloneURL)
			cloneCmd.Stdin = os.Stdin
			cloneCmd.Stdout = os.Stdout
			cloneCmd.Stderr = os.Stderr

			err = cloneCmd.Run()
			if err != nil {
				log.Fatal(err)
			}
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
	cmd.Flags().StringVar(&importURL, "import-url", "", "import repo from given URL")
	cmd.RegisterFlagCompletionFunc("import-url", cobra.NoFileCompletions)
	return cmd
}

func newGitListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var repos *gitsrht.RepositoryCursor
		if len(args) > 0 {
			owner, instance := parseOwnerName(args[0])
			c := createClientWithInstance("git", cmd, instance)
			username := strings.TrimLeft(owner, ownerPrefixes)
			user, err := gitsrht.RepositoriesByUser(c.Client, ctx, username)
			if err != nil {
				log.Fatal(err)
			} else if user == nil {
				log.Fatal("no such user")
			}
			repos = user.Repositories
		} else {
			c := createClient("git", cmd)
			var err error
			repos, err = gitsrht.Repositories(c.Client, ctx)
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
		Use:               "list [user]",
		Short:             "List repos",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newGitDeleteCommand() *cobra.Command {
	var autoConfirm bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getRepoName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("git", cmd, instance)
		id := getRepoID(c, ctx, name, owner)

		if !autoConfirm && !getConfirmation(fmt.Sprintf("Do you really want to delete the repo %s", name)) {
			fmt.Println("Aborted")
			return
		}
		repo, err := gitsrht.DeleteRepository(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Deleted repository %s\n", repo.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete [repo]",
		Short:             "Delete a repository",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeRepo,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "auto confirm")
	return cmd
}

func newGitArtifactCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifact",
		Short: "Manage artifacts",
	}
	cmd.AddCommand(newGitArtifactUploadCommand())
	cmd.AddCommand(newGitArtifactListCommand())
	cmd.AddCommand(newGitArtifactDeleteCommand())
	return cmd
}

func newGitArtifactUploadCommand() *cobra.Command {
	var rev string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		repoName, owner, instance, err := getRepoName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("git", cmd, instance)
		repoID := getRepoID(c, ctx, repoName, owner)

		if rev == "" {
			var err error
			rev, err = guessRev()
			if err != nil {
				log.Fatal(err)
			}
		}

		for _, filename := range args {
			f, err := os.Open(filename)
			if err != nil {
				log.Fatalf("failed to open input file: %v", err)
			}
			defer f.Close()

			file := gqlclient.Upload{Filename: filepath.Base(filename), Body: f}
			artifact, err := gitsrht.UploadArtifact(c.Client, ctx, repoID, rev, file)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("Uploaded %s\n", artifact.Filename)
		}
	}

	cmd := &cobra.Command{
		Use:   "upload <filename...>",
		Short: "Upload artifacts",
		Args:  cobra.MinimumNArgs(1),
		Run:   run,
	}
	cmd.Flags().StringVar(&rev, "rev", "", "revision tag")
	cmd.RegisterFlagCompletionFunc("rev", completeRev)
	return cmd
}

func newGitArtifactListCommand() *cobra.Command {
	// TODO: Filter by rev

	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		repoName, owner, instance, err := getRepoName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("git", cmd, instance)

		var (
			username string
			user     *gitsrht.User
		)

		if owner != "" {
			username = strings.TrimLeft(owner, ownerPrefixes)
			user, err = gitsrht.ListArtifactsByUser(c.Client, ctx, username, repoName)
		} else {
			user, err = gitsrht.ListArtifacts(c.Client, ctx, repoName)
		}

		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatalf("no such user %q", username)
		} else if user.Repository == nil {
			log.Fatalf("no such repository %q", repoName)
		}

		for _, ref := range user.Repository.References.Results {
			if len(ref.Artifacts.Results) == 0 {
				continue
			}

			name := ref.Name[strings.LastIndex(ref.Name, "/")+1:]
			fmt.Printf("Tag %s:\n", termfmt.Bold.String(name))
			for _, artifact := range ref.Artifacts.Results {
				fmt.Printf("  %s %s\n", termfmt.DarkYellow.Sprintf("#%d", artifact.Id), artifact.Filename)
			}
		}
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List artifacts",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	return cmd
}

func newGitArtifactDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		artifact, err := gitsrht.DeleteArtifact(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Deleted artifact %s\n", artifact.Filename)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete an artifact",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeArtifacts,
		Run:               run,
	}
	return cmd
}

func newGitACLCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acl",
		Short: "Manage access-control lists",
	}
	cmd.AddCommand(newGitACLListCommand())
	cmd.AddCommand(newGitACLUpdateCommand())
	cmd.AddCommand(newGitACLDeleteCommand())
	return cmd
}

func newGitACLListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		var name, instance string
		if len(args) > 0 {
			// TODO: handle owner
			name, _, instance = parseResourceName(args[0])
		} else {
			var err error
			name, _, instance, err = getRepoName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("git", cmd, instance)

		user, err := gitsrht.AclByRepoName(c.Client, ctx, name)
		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatal("no such user")
		} else if user.Repository == nil {
			log.Fatalf("no such repository %q", name)
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer tw.Flush()
		for _, acl := range user.Repository.AccessControlList.Results {
			var mode string
			if acl.Mode != nil {
				mode = string(*acl.Mode)
			}

			created := termfmt.Dim.Sprintf("%s ago", timeDelta(acl.Created))
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", termfmt.DarkYellow.Sprintf("#%d", acl.Id),
				acl.Entity.CanonicalName, mode, created)
		}
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

func newGitACLUpdateCommand() *cobra.Command {
	var mode string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		accessMode, err := gitsrht.ParseAccessMode(mode)
		if err != nil {
			log.Fatal(err)
		}

		if strings.IndexAny(args[0], ownerPrefixes) != 0 {
			log.Fatal("user must be in canonical form")
		}

		name, owner, instance, err := getRepoName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("git", cmd, instance)
		id := getRepoID(c, ctx, name, owner)

		acl, err := gitsrht.UpdateACL(c.Client, ctx, id, accessMode, args[0])
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Updated access rights for %s\n", acl.Entity.CanonicalName)
	}

	cmd := &cobra.Command{
		Use:               "update <user>",
		Short:             "Update/add ACL entries",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringVarP(&mode, "mode", "m", "", "access mode")
	cmd.RegisterFlagCompletionFunc("mode", completeAccessMode)
	cmd.MarkFlagRequired("mode")
	return cmd
}

func newGitACLDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		acl, err := gitsrht.DeleteACL(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if acl == nil {
			log.Fatalf("failed to delete ACL entry with ID %d", id)
		}

		fmt.Printf("Deleted ACL entry for %s in repository %s\n", acl.Entity.CanonicalName, acl.Repository.Name)
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

func newGitShowCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getRepoName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("git", cmd, instance)

		var (
			user     *gitsrht.User
			username string
			err      error
		)
		if owner == "" {
			user, err = gitsrht.RepositoryByName(c.Client, ctx, name)
		} else {
			username = strings.TrimLeft(owner, ownerPrefixes)
			user, err = gitsrht.RepositoryByUser(c.Client, ctx, username, name)
		}
		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatalf("no such user %q", username)
		} else if user.Repository == nil {
			log.Fatalf("no such repository %q", name)
		}
		repo := user.Repository

		// prints basic information
		fmt.Printf("%s (%s)\n", termfmt.Bold.String(repo.Name), repo.Visibility.TermString())
		if repo.Description != nil && *repo.Description != "" {
			fmt.Printf("  %s\n", *repo.Description)
		}

		// prints latest tag
		tags := repo.References.Tags()
		if len(tags) > 0 {
			fmt.Println()
			fmt.Printf("  Latest tag: %s\n", tags[len(tags)-1])
		}

		// prints branches
		branches := repo.References.Heads()
		if len(branches) > 0 {
			fmt.Println()
			fmt.Printf("  Branches:\n")
			for i := 0; i < len(branches); i++ {
				fmt.Printf("    %s\n", branches[i])
			}
		}

		// prints the three most recent commits
		if len(repo.Log.Results) >= 3 {
			fmt.Println()
			fmt.Printf("  Recent log:\n")

			for _, commit := range repo.Log.Results[:3] {
				fmt.Printf("    %s %s <%s> (%s ago)\n",
					termfmt.Yellow.Sprintf("%s", commit.ShortId),
					commit.Author.Name,
					commit.Author.Email,
					timeDelta(commit.Author.Time))

				commitLines := strings.Split(commit.Message, "\n")
				fmt.Printf("      %s\n", commitLines[0])
			}
		}
	}

	cmd := &cobra.Command{
		Use:               "show [repo]",
		Short:             "Shows a repository",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeRepo,
		Run:               run,
	}

	return cmd
}

func newGitUserWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user-webhook",
		Short: "Manage user webhooks",
	}
	cmd.AddCommand(newGitUserWebhookCreateCommand())
	cmd.AddCommand(newGitUserWebhookListCommand())
	cmd.AddCommand(newGitUserWebhookDeleteCommand())
	return cmd
}

func newGitUserWebhookCreateCommand() *cobra.Command {
	var events []string
	var stdin bool
	var url string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git", cmd)

		var config gitsrht.UserWebhookInput
		config.Url = url
		config.Query = readWebhookQuery(stdin)

		whEvents, err := gitsrht.ParseEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents

		webhook, err := gitsrht.CreateUserWebhook(c.Client, ctx, config)
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
	cmd.RegisterFlagCompletionFunc("events", completeGitUserWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newGitUserWebhookListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git", cmd)

		webhooks, err := gitsrht.UserWebhooks(c.Client, ctx)
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

func newGitUserWebhookDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		webhook, err := gitsrht.DeleteUserWebhook(c.Client, ctx, id)
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
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func getRepoName(ctx context.Context, cmd *cobra.Command) (repoName, owner, instance string, err error) {
	repoName, err = cmd.Flags().GetString("repo")
	if err != nil {
		return "", "", "", err
	} else if repoName != "" {
		repoName, owner, instance = parseResourceName(repoName)
		return repoName, owner, instance, nil
	}

	repoName, owner, instance, err = guessGitRepoName(ctx)
	if err != nil {
		return "", "", "", err
	}

	return repoName, owner, instance, nil
}

func guessGitRepoName(ctx context.Context) (repoName, owner, instance string, err error) {
	remoteURL, err := gitRemoteURL(ctx)
	if err != nil {
		return "", "", "", err
	}

	parts := strings.Split(strings.Trim(remoteURL.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("failed to parse Git URL %q: expected 2 path components", remoteURL)
	}
	owner, repoName = parts[0], parts[1]

	// TODO: ignore port in host
	return repoName, owner, remoteURL.Host, nil
}

func getRepoID(c *Client, ctx context.Context, name, owner string) int32 {
	var (
		user     *gitsrht.User
		username string
		err      error
	)
	if owner == "" {
		user, err = gitsrht.RepositoryIDByName(c.Client, ctx, name)
	} else {
		username = strings.TrimLeft(owner, ownerPrefixes)
		user, err = gitsrht.RepositoryIDByUser(c.Client, ctx, username, name)
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

func gitRemoteURL(ctx context.Context) (*url.URL, error) {
	// TODO: iterate over all remotes, find one which matches the config file, etc
	out, err := exec.CommandContext(ctx, "git", "remote", "get-url", "origin").Output()
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
			return nil, fmt.Errorf("invalid scp-like Git URL %q: missing colon", raw)
		}
		host, path := raw[:i], raw[i+1:]

		// Strip optional user
		if i := strings.Index(host, "@"); i >= 0 {
			host = host[i+1:]
		}

		return &url.URL{Scheme: "ssh", Host: host, Path: path}, nil
	}
}

func guessRev() (string, error) {
	out, err := exec.Command("git", "describe", "--abbrev=0").Output()
	if err != nil {
		return "", fmt.Errorf("failed to autodetect revision tag: %v", err)
	}

	return strings.TrimSpace(string(out)), nil
}

func completeRepo(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("git", cmd)
	var repoList []string

	repos, err := gitsrht.RepoNames(c.Client, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, repo := range repos.Results {
		repoList = append(repoList, repo.Name)
	}

	return repoList, cobra.ShellCompDirectiveNoFileComp
}

func completeRev(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	repo, err := cmd.Flags().GetString("repo")
	if err == nil && repo != "" {
		ctx := cmd.Context()
		c := createClient("git", cmd)

		user, err := gitsrht.RevsByRepoName(c.Client, ctx, repo)
		if err != nil || user.Repository == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return user.Repository.References.Tags(), cobra.ShellCompDirectiveNoFileComp
	}

	output, err := exec.Command("git", "tag").Output()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	revs := strings.Split(string(output), "\n")
	return revs, cobra.ShellCompDirectiveNoFileComp
}

func completeAccessMode(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"RO", "RW"}, cobra.ShellCompDirectiveNoFileComp
}

func completeArtifacts(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	repoName, _, instance, err := getRepoName(ctx, cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	c := createClientWithInstance("git", cmd, instance)
	var artifactList []string

	user, err := gitsrht.ListArtifacts(c.Client, ctx, repoName)
	if err != nil || user.Repository == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, ref := range user.Repository.References.Results {
		for _, artifact := range ref.Artifacts.Results {
			name := ref.Name[strings.LastIndex(ref.Name, "/")+1:]
			s := fmt.Sprintf("%d\t%s (%s)", artifact.Id, artifact.Filename, name)
			artifactList = append(artifactList, s)
		}
	}
	return artifactList, cobra.ShellCompDirectiveNoFileComp
}

func completeGitUserWebhookEvents(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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
