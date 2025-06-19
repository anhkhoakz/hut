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
	"path/filepath"
	"strings"

	"git.sr.ht/~emersion/gqlclient"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"git.sr.ht/~xenrox/hut/srht/gitsrht"
	"git.sr.ht/~xenrox/hut/termfmt"
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
	cmd.AddCommand(newGitCloneCommand())
	cmd.AddCommand(newGitSetupCommand())
	cmd.AddCommand(newGitACLCommand())
	cmd.AddCommand(newGitShowCommand())
	cmd.AddCommand(newGitUserWebhookCommand())
	cmd.AddCommand(newGitUpdateCommand())
	cmd.AddCommand(newGitWebhookCommand())
	cmd.PersistentFlags().StringP("repo", "r", "", "name of repository")
	cmd.RegisterFlagCompletionFunc("repo", completeGitRepo)
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

		var description *string
		if desc != "" {
			description = &desc
		}

		repo, err := gitsrht.CreateRepository(c.Client, ctx, args[0],
			gitVisibility, description, importURLPtr)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Created repository %s\n", repo.Name)

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

		if clone {
			cloneCmd := exec.Command("git", "clone", cloneURL)
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
	cmd.Flags().StringVar(&importURL, "import-url", "", "import repo from given URL")
	cmd.RegisterFlagCompletionFunc("import-url", cobra.NoFileCompletions)
	return cmd
}

func newGitListCommand() *cobra.Command {
	var count int
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		var cursor *gitsrht.Cursor
		var owner, instance string
		if len(args) > 0 {
			owner, instance = parseOwnerName(args[0])
		}

		err := pagerify(func(p pager) error {
			var repos *gitsrht.RepositoryCursor
			if len(owner) > 0 {
				c := createClientWithInstance("git", cmd, instance)
				username := strings.TrimLeft(owner, ownerPrefixes)
				user, err := gitsrht.RepositoriesByUser(c.Client, ctx, username, cursor)
				if err != nil {
					return err
				} else if user == nil {
					return errors.New("no such user")
				}
				repos = user.Repositories
			} else {
				c := createClient("git", cmd)
				var err error
				repos, err = gitsrht.Repositories(c.Client, ctx, cursor)
				if err != nil {
					return err
				}
			}

			for _, repo := range repos.Results {
				printGitRepo(p, &repo)
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
		Use:               "list [user]",
		Short:             "List repos",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().IntVar(&count, "count", 0, "number of repos to fetch")
	cmd.RegisterFlagCompletionFunc("count", cobra.NoFileCompletions)
	return cmd
}

func printGitRepo(w io.Writer, repo *gitsrht.Repository) {
	fmt.Fprintf(w, "%s (%s)\n", termfmt.Bold.String(repo.Name), repo.Visibility.TermString())
	if repo.Description != nil && *repo.Description != "" {
		fmt.Fprintf(w, "  %s\n", *repo.Description)
	}
	fmt.Fprintln(w)
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
			name, owner, instance, err = getGitRepoName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("git", cmd, instance)
		id, err := getGitRepoID(c, ctx, name, owner)
		if err != nil {
			log.Fatal(err)
		}

		if !autoConfirm && !getConfirmation(fmt.Sprintf("Do you really want to delete the repo %s", name)) {
			log.Println("Aborted")
			return
		}
		repo, err := gitsrht.DeleteRepository(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted repository %s\n", repo.Name)
	}

	cmd := &cobra.Command{
		Use:               "delete [repo]",
		Short:             "Delete a repository",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeGitRepo,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "auto confirm")
	return cmd
}

func newGitCloneCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		log.Println("Cloning repository")
		cloneCmd := exec.Command("git", "clone", args[0])
		cloneCmd.Stdin = os.Stdin
		cloneCmd.Stdout = os.Stdout
		cloneCmd.Stderr = os.Stderr

		err := cloneCmd.Run()
		if err != nil {
			log.Fatalf("failed to clone repo: %v", err)
		}

		s := strings.Split(args[0], "/")
		dir, err := os.Getwd()
		if err != nil {
			log.Fatalf("failed to get current directory: %v", err)
		}

		err = os.Chdir(filepath.Join(dir, s[len(s)-1]))
		if err != nil {
			log.Fatalf("failed to change current working directory: %v", err)
		}

		cfg, err := loadProjectConfig()
		if err != nil {
			log.Fatalf("failed to load project config: %v", err)
		}

		if cfg != nil {
			if cfg.DevList != "" {
				log.Printf("Configuring repository for %q\n", "git send-email")

				sendemailCmd := exec.Command("git", "config", "sendemail.to", cfg.DevList)
				sendemailCmd.Stdin = os.Stdin
				sendemailCmd.Stdout = os.Stdout
				sendemailCmd.Stderr = os.Stderr

				err = sendemailCmd.Run()
				if err != nil {
					log.Fatalf("failed to set %q: %v", "git config sendemail.to", err)
				}
			}

			if cfg.PatchPrefix {
				prefixCmd := exec.Command("git", "config", "format.subjectPrefix", fmt.Sprintf("PATCH %s", s[len(s)-1]))
				prefixCmd.Stdin = os.Stdin
				prefixCmd.Stdout = os.Stdout
				prefixCmd.Stderr = os.Stderr

				err = prefixCmd.Run()
				if err != nil {
					log.Fatalf("failed to set %q: %v", "git config format.subjectPrefix", err)
				}
			}
		}
	}
	cmd := &cobra.Command{
		Use:               "clone <URL>",
		Short:             "Clone a repository",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func newGitSetupCommand() *cobra.Command {
	var force bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		success := false

		checkCmd := exec.Command("git", "config", "sendemail.to")
		checkCmd.Stdin = os.Stdin
		checkCmd.Stderr = os.Stderr

		b, err := checkCmd.Output()
		if err != nil {
			log.Fatalf("failed to check current git settings: %v", err)
		}

		if strings.TrimSpace(string(b)) != "" && !force {
			log.Println("Repository is already configured. Skipping setup.")
			return
		}

		// TODO: hub.sr.ht API, .b4-config
		cfg, err := loadProjectConfig()
		if err != nil {
			log.Fatalf("failed to load project config: %v", err)
		}

		if cfg != nil {
			if cfg.DevList != "" {
				sendemailCmd := exec.Command("git", "config", "sendemail.to", cfg.DevList)
				sendemailCmd.Stdin = os.Stdin
				sendemailCmd.Stdout = os.Stdout
				sendemailCmd.Stderr = os.Stderr

				err = sendemailCmd.Run()
				if err != nil {
					log.Fatalf("failed to set %q: %v", "git config sendemail.to", err)
				}
				success = true
			}

			if cfg.PatchPrefix {
				name, _, _, err := guessGitRepoName(ctx, cmd)
				if err != nil {
					log.Fatalf("failed to get repository name: %v", err)
				}

				prefixCmd := exec.Command("git", "config", "format.subjectPrefix", fmt.Sprintf("PATCH %s", name))
				prefixCmd.Stdin = os.Stdin
				prefixCmd.Stdout = os.Stdout
				prefixCmd.Stderr = os.Stderr

				err = prefixCmd.Run()
				if err != nil {
					log.Fatalf("failed to set %q: %v", "git config format.subjectPrefix", err)
				}
			}
		}

		if success {
			log.Println("Configured repository")
		} else {
			log.Fatalln("Failed to configure repository")
		}
	}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup a repository for `git send-email`",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "force setup even if already configured")
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
		repoName, owner, instance, err := getGitRepoName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("git", cmd, instance)
		c.HTTP.Timeout = fileTransferTimeout
		repoID, err := getGitRepoID(c, ctx, repoName, owner)
		if err != nil {
			log.Fatal(err)
		}

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

			log.Printf("Uploaded %s\n", artifact.Filename)
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
		repoName, owner, instance, err := getGitRepoName(ctx, cmd)
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

		log.Printf("Deleted artifact %s\n", artifact.Filename)
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
	var count int
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getGitRepoName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("git", cmd, instance)
		var (
			cursor   *gitsrht.Cursor
			user     *gitsrht.User
			username string
			err      error
		)
		if owner != "" {
			username = strings.TrimLeft(owner, ownerPrefixes)
		}

		err = pagerify(func(p pager) error {
			if username != "" {
				user, err = gitsrht.AclByUser(c.Client, ctx, username, name, cursor)
			} else {
				user, err = gitsrht.AclByRepoName(c.Client, ctx, name, cursor)
			}

			if err != nil {
				return err
			} else if user == nil {
				return errors.New("no such user")
			} else if user.Repository == nil {
				return fmt.Errorf("no such repository %q", name)
			}

			for _, acl := range user.Repository.Acls.Results {
				printGitACLEntry(p, &acl)
			}

			cursor = user.Repository.Acls.Cursor
			if p.IsDone(cursor, len(user.Repository.Acls.Results)) {
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
		ValidArgsFunction: completeGitRepo,
		Run:               run,
	}
	cmd.Flags().IntVar(&count, "count", 0, "number of ACL entries to fetch")
	cmd.RegisterFlagCompletionFunc("count", cobra.NoFileCompletions)
	return cmd
}

func printGitACLEntry(w io.Writer, acl *gitsrht.ACL) {
	var mode string
	if acl.Mode != nil {
		mode = string(*acl.Mode)
	}

	created := termfmt.Dim.String(humanize.Time(acl.Created.Time))
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", termfmt.DarkYellow.Sprintf("#%d", acl.Id),
		acl.Entity.CanonicalName, mode, created)
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

		name, owner, instance, err := getGitRepoName(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}

		c := createClientWithInstance("git", cmd, instance)
		id, err := getGitRepoID(c, ctx, name, owner)
		if err != nil {
			log.Fatal(err)
		}

		acl, err := gitsrht.UpdateACL(c.Client, ctx, id, accessMode, args[0])
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

		log.Printf("Deleted ACL entry for %s in repository %s\n", acl.Entity.CanonicalName, acl.Repository.Name)
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
	var web bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getGitRepoName(ctx, cmd)
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
		repoUrl := fmt.Sprintf("%s/%s/%s", c.BaseURL, owner, name)

		if web {
			err := openURL(repoUrl)
			if err != nil {
				log.Fatal(err)
			}
			os.Exit(0)
		}

		// prints url for easy access
		fmt.Printf("%s\n\n", repoUrl)

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
				fmt.Printf("    %s %s <%s> (%s)\n",
					termfmt.Yellow.Sprintf("%s", commit.ShortId),
					commit.Author.Name,
					commit.Author.Email,
					humanize.Time(commit.Author.Time.Time))

				commitLines := strings.Split(commit.Message, "\n")
				fmt.Printf("      %s\n", commitLines[0])
			}
		}
	}

	cmd := &cobra.Command{
		Use:               "show [repo]",
		Short:             "Shows a repository",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeGitRepo,
		Run:               run,
	}
	cmd.Flags().BoolVar(&web, "web", false, "open in browser")

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

		whEvents, err := gitsrht.ParseEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents
		config.Query = readWebhookQuery(stdin)

		webhook, err := gitsrht.CreateUserWebhook(c.Client, ctx, config)
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
	cmd.RegisterFlagCompletionFunc("events", completeGitUserWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", !isStdinTerminal, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newGitUserWebhookListCommand() *cobra.Command {
	var count int
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git", cmd)
		var cursor *gitsrht.Cursor

		err := pagerify(func(p pager) error {
			webhooks, err := gitsrht.UserWebhooks(c.Client, ctx, cursor)
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
		}

		log.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a user webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeGitUserWebhookID,
		Run:               run,
	}
	return cmd
}

func newGitUpdateCommand() *cobra.Command {
	var visibility, branch, readme, description, newName string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getGitRepoName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("git", cmd, instance)
		id, err := getGitRepoID(c, ctx, name, owner)
		if err != nil {
			log.Fatal(err)
		}
		var input gitsrht.RepoInput

		if visibility != "" {
			repoVisibility, err := gitsrht.ParseVisibility(visibility)
			if err != nil {
				log.Fatal(err)
			}
			input.Visibility = &repoVisibility
		}

		if cmd.Flags().Changed("description") {
			if description == "" {
				_, err := gitsrht.ClearDescription(c.Client, ctx, id)
				if err != nil {
					log.Fatalf("failed to clear description: %v", err)
				}
			} else {
				input.Description = &description
			}
		}

		if branch != "" {
			input.HEAD = &branch
		}

		if newName != "" {
			input.Name = &newName
		}

		if readme == "" && cmd.Flags().Changed("readme") {
			_, err := gitsrht.ClearCustomReadme(c.Client, ctx, id)
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

		repo, err := gitsrht.UpdateRepository(c.Client, ctx, id, input)
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
		ValidArgsFunction: completeGitRepo,
		Run:               run,
	}
	cmd.Flags().StringVarP(&visibility, "visibility", "v", "", "repository visibility")
	cmd.RegisterFlagCompletionFunc("visibility", completeVisibility)
	cmd.Flags().StringVarP(&branch, "default-branch", "b", "", "default branch")
	cmd.RegisterFlagCompletionFunc("default-branch", completeBranches)
	cmd.Flags().StringVar(&readme, "readme", "", "update the custom README")
	cmd.Flags().StringVarP(&description, "description", "d", "", "repository description")
	cmd.RegisterFlagCompletionFunc("description", cobra.NoFileCompletions)
	cmd.Flags().StringVarP(&newName, "name", "n", "", "repository name")
	cmd.RegisterFlagCompletionFunc("name", cobra.NoFileCompletions)
	return cmd
}

func newGitWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage git webhooks",
	}
	cmd.AddCommand(newGitWebhookCreateCommand())
	cmd.AddCommand(newGitWebhookListCommand())
	cmd.AddCommand(newGitWebhookDeleteCommand())
	return cmd
}

func newGitWebhookCreateCommand() *cobra.Command {
	var events []string
	var stdin bool
	var url string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getGitRepoName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("git", cmd, instance)
		id, err := getGitRepoID(c, ctx, name, owner)
		if err != nil {
			log.Fatal(err)
		}

		var config gitsrht.GitWebhookInput
		config.RepositoryID = id
		config.Url = url

		whEvents, err := gitsrht.ParseGitWebhookEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents
		config.Query = readWebhookQuery(stdin)

		webhook, err := gitsrht.CreateGitWebhook(c.Client, ctx, config)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Created git webhook with ID %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "Create [repo]",
		Short:             "Create a git webhook",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeGitRepo,
		Run:               run,
	}
	cmd.Flags().StringSliceVarP(&events, "events", "e", nil, "webhook events")
	cmd.RegisterFlagCompletionFunc("events", completeGitWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", !isStdinTerminal, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newGitWebhookListCommand() *cobra.Command {
	var count int
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var name, owner, instance string
		if len(args) > 0 {
			name, owner, instance = parseResourceName(args[0])
		} else {
			var err error
			name, owner, instance, err = getGitRepoName(ctx, cmd)
			if err != nil {
				log.Fatal(err)
			}
		}

		c := createClientWithInstance("git", cmd, instance)
		id, err := getGitRepoID(c, ctx, name, owner)
		if err != nil {
			log.Fatal(err)
		}

		var cursor *gitsrht.Cursor
		err = pagerify(func(p pager) error {
			webhooks, err := gitsrht.GitWebhooks(c.Client, ctx, id, cursor)
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
		Use:               "list [repo]",
		Short:             "List git webhooks",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeGitRepo,
		Run:               run,
	}
	cmd.Flags().IntVar(&count, "count", 0, "number of webhooks to fetch")
	cmd.RegisterFlagCompletionFunc("count", cobra.NoFileCompletions)
	return cmd
}

func newGitWebhookDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		webhook, err := gitsrht.DeleteGitWebhook(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a git webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func getGitRepoName(ctx context.Context, cmd *cobra.Command) (repoName, owner, instance string, err error) {
	repoName, err = cmd.Flags().GetString("repo")
	if err != nil {
		return "", "", "", err
	} else if repoName != "" {
		repoName, owner, instance = parseResourceName(repoName)
		return repoName, owner, instance, nil
	}
	return guessGitRepoName(ctx, cmd)
}

func guessGitRepoName(ctx context.Context, cmd *cobra.Command) (repoName, owner, instance string, err error) {
	remoteURLs, err := gitRemoteURLs(ctx)
	if err != nil {
		return "", "", "", err
	}

	cfg := loadConfig(cmd)
	for _, remoteURL := range remoteURLs {
		if remoteURL.Host == "" {
			continue
		}

		match := false
		for _, instance := range cfg.Instances {
			if instance.match(remoteURL.Host) {
				match = true
				break
			}
		}
		if !match {
			continue
		}

		parts := strings.Split(strings.Trim(remoteURL.Path, "/"), "/")
		if len(parts) != 2 {
			return "", "", "", fmt.Errorf("failed to parse Git URL %q: expected 2 path components", remoteURL)
		}
		owner, repoName = parts[0], parts[1]

		// TODO: ignore port in host
		return repoName, owner, remoteURL.Host, nil
	}

	return "", "", "", fmt.Errorf("no sr.ht Git repository found in current directory")
}

func getGitRepoID(c *Client, ctx context.Context, name, owner string) (int32, error) {
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
		return 0, fmt.Errorf("failed to get repository ID: %v", err)
	} else if user == nil {
		return 0, fmt.Errorf("no such user %q", username)
	} else if user.Repository == nil {
		return 0, fmt.Errorf("no such repository %q", name)
	}
	return user.Repository.Id, nil
}

func gitRemoteURLs(ctx context.Context) ([]*url.URL, error) {
	var urls []*url.URL

	out, err := exec.CommandContext(ctx, "git", "remote", "-v").Output()
	if err != nil {
		eerr, ok := err.(*exec.ExitError)
		if ok && eerr.ExitCode() == 128 {
			return urls, nil
		}
		return nil, fmt.Errorf("failed to get remote URL: %v", err)
	}

	l := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, raw := range l {
		_, raw, _ = strings.Cut(raw, "\t") // remote name
		raw = strings.TrimSuffix(raw, " (fetch)")
		raw = strings.TrimSuffix(raw, " (push)")

		var u *url.URL
		switch {
		case strings.Contains(raw, "://"):
			u, err = url.Parse(raw)
			if err != nil {
				return nil, err
			}
		case strings.HasPrefix(raw, "/"):
			u = &url.URL{Scheme: "file", Path: raw}
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

			u = &url.URL{Scheme: "ssh", Host: host, Path: path}
		}
		urls = append(urls, u)
	}

	return urls, nil
}

func guessRev() (string, error) {
	out, err := exec.Command("git", "describe", "--abbrev=0").Output()
	if err != nil {
		return "", fmt.Errorf("failed to autodetect revision tag: %v", err)
	}

	return strings.TrimSpace(string(out)), nil
}

func completeGitRepo(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("git", cmd)
	var repoList []string

	repos, err := gitsrht.CompleteRepositories(c.Client, ctx)
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

func completeArtifacts(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	repoName, owner, instance, err := getGitRepoName(ctx, cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	c := createClientWithInstance("git", cmd, instance)
	var user *gitsrht.User
	var artifactList []string

	if owner != "" {
		username := strings.TrimLeft(owner, ownerPrefixes)
		user, err = gitsrht.ListArtifactsByUser(c.Client, ctx, username, repoName)
	} else {
		user, err = gitsrht.ListArtifacts(c.Client, ctx, repoName)
	}

	if err != nil || user == nil || user.Repository == nil {
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

func completeGitUserWebhookID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("git", cmd)
	var webhookList []string

	webhooks, err := gitsrht.UserWebhooks(c.Client, ctx, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, webhook := range webhooks.Results {
		s := fmt.Sprintf("%d\t%s", webhook.Id, webhook.Url)
		webhookList = append(webhookList, s)
	}

	return webhookList, cobra.ShellCompDirectiveNoFileComp
}

func completeGitWebhookEvents(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var eventList []string
	events := [2]string{"git_pre_receive", "git_post_receive"}
	set := strings.ToLower(cmd.Flag("events").Value.String())
	for _, event := range events {
		if !strings.Contains(set, event) {
			eventList = append(eventList, event)
		}
	}
	return eventList, cobra.ShellCompDirectiveNoFileComp
}

func completeBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	repoName, owner, instace, err := getGitRepoName(ctx, cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	c := createClientWithInstance("git", cmd, instace)

	var user *gitsrht.User
	if owner != "" {
		username := strings.TrimLeft(owner, ownerPrefixes)
		user, err = gitsrht.RevsByUser(c.Client, ctx, username, repoName)
	} else {
		user, err = gitsrht.RevsByRepoName(c.Client, ctx, repoName)
	}

	if err != nil || user == nil || user.Repository == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return user.Repository.References.Heads(), cobra.ShellCompDirectiveNoFileComp
}

func completeCoMaintainers(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	// Since completeCoMaintainers is intended to be called from other services
	// than git, we cannot use getRepoName which requires the "repo" flag to be set.
	repoName, _, instace, err := guessGitRepoName(ctx, cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	c := createClientWithInstance("git", cmd, instace)

	var userList []string
	user, err := gitsrht.CompleteCoMaintainers(c.Client, ctx, repoName)
	if err != nil || user.Repositories == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, acl := range user.Repository.Acls.Results {
		userList = append(userList, acl.Entity.CanonicalName)
	}

	return userList, cobra.ShellCompDirectiveNoFileComp
}
