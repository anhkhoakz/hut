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
	cmd.PersistentFlags().StringP("repo", "r", "", "name of repository")
	cmd.RegisterFlagCompletionFunc("repo", completeRepo)
	return cmd
}

func newGitCreateCommand() *cobra.Command {
	var visibility, desc string
	var clone bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git", cmd)

		gitVisibility, err := getGitVisibility(visibility)
		if err != nil {
			log.Fatal(err)
		}

		repo, err := gitsrht.CreateRepository(c.Client, ctx, args[0],
			gitVisibility, desc)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Created repository %s\n", repo.Name)

		if clone {
			ver, err := gitsrht.SshSettings(c.Client, ctx)
			if err != nil {
				log.Fatalf("failed to retrieve settings: %v", err)
			}

			cloneURL := fmt.Sprintf("%s@git.%s:%s/%s", ver.Settings.SshUser, c.Hostname,
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
	cmd.Flags().StringVarP(&visibility, "visibility", "v", "unlisted", "repo visibility")
	cmd.RegisterFlagCompletionFunc("visibility", completeVisibility)
	cmd.Flags().StringVarP(&desc, "description", "d", "", "repo description")
	cmd.RegisterFlagCompletionFunc("description", cobra.NoFileCompletions)
	cmd.Flags().BoolVarP(&clone, "clone", "c", false, "autoclone repo")
	return cmd
}

func newGitListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git", cmd)

		var repos *gitsrht.RepositoryCursor

		if len(args) > 0 {
			username := strings.TrimLeft(args[0], ownerPrefixes)
			user, err := gitsrht.RepositoriesByUser(c.Client, ctx, username)
			if err != nil {
				log.Fatal(err)
			} else if user == nil {
				log.Fatal("no such user")
			}
			repos = user.Repositories
		} else {
			var err error
			repos, err = gitsrht.Repositories(c.Client, ctx)
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

func newGitDeleteCommand() *cobra.Command {
	var autoConfirm bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git", cmd)

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			name = getRepoName(ctx, cmd, c)
		}

		repo, err := gitsrht.RepositoryByName(c.Client, ctx, name)
		if err != nil {
			log.Fatalf("failed to get repository ID: %v", err)
		} else if repo == nil {
			log.Fatalf("repository %s does not exist", name)
		}

		if !autoConfirm && !getConfirmation(fmt.Sprintf("Do you really want to delete the repo %s", name)) {
			fmt.Println("Aborted")
			return
		}
		repo, err = gitsrht.DeleteRepository(c.Client, ctx, repo.Id)
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
		c := createClient("git", cmd)
		repoName := getRepoName(ctx, cmd, c)

		if rev == "" {
			var err error
			rev, err = guessRev()
			if err != nil {
				log.Fatal(err)
			}
		}

		filename := args[0]

		f, err := os.Open(filename)
		if err != nil {
			log.Fatalf("failed to open input file: %v", err)
		}
		defer f.Close()

		file := gqlclient.Upload{Filename: filepath.Base(filename), Body: f}

		repo, err := gitsrht.RepositoryByName(c.Client, ctx, repoName)
		if err != nil {
			log.Fatalf("failed to get repository ID: %v", err)
		} else if repo == nil {
			log.Fatalf("repository %s does not exist", repoName)
		}

		artifact, err := gitsrht.UploadArtifact(c.Client, ctx, repo.Id, rev, file)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Uploaded %s\n", artifact.Filename)
	}

	cmd := &cobra.Command{
		Use:   "upload <filename>",
		Short: "Upload an artifact",
		Args:  cobra.ExactArgs(1),
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
		c := createClient("git", cmd)
		repoName := getRepoName(ctx, cmd, c)

		repo, err := gitsrht.ListArtifacts(c.Client, ctx, repoName)
		if err != nil {
			log.Fatal(err)
		} else if repo == nil {
			log.Fatalf("repository %s does not exist", repoName)
		}

		for _, ref := range repo.References.Results {
			if len(ref.Artifacts.Results) == 0 {
				continue
			}

			name := ref.Name[strings.LastIndex(ref.Name, "/")+1:]
			fmt.Printf("Tag %s:\n", name)
			for _, artifact := range ref.Artifacts.Results {
				fmt.Printf("  #%d: %s\n", artifact.Id, artifact.Filename)
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
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func getRepoName(ctx context.Context, cmd *cobra.Command, c *Client) string {
	if repoName, err := cmd.Flags().GetString("repo"); err != nil {
		log.Fatal(err)
	} else if repoName != "" {
		return repoName
	}

	repoName, err := guessGitRepoName(ctx, c)
	if err != nil {
		log.Fatal(err)
	}

	return repoName
}

func guessGitRepoName(ctx context.Context, c *Client) (string, error) {
	remoteURL, err := gitRemoteURL(ctx)
	if err != nil {
		return "", err
	}

	// TODO: ignore port in host
	if !strings.HasSuffix(remoteURL.Host, "."+c.Hostname) {
		return "", fmt.Errorf("Git URL %q doesn't match hostname %q", remoteURL, c.Hostname)
	}

	parts := strings.Split(strings.Trim(remoteURL.Path, "/"), "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("failed to parse Git URL %q: expected 2 path components", remoteURL)
	}
	repoName := parts[1]

	// TODO: handle repos not belonging to authenticated user
	return repoName, nil
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

func getGitVisibility(visibility string) (gitsrht.Visibility, error) {
	switch strings.ToLower(visibility) {
	case "unlisted":
		return gitsrht.VisibilityUnlisted, nil
	case "private":
		return gitsrht.VisibilityPrivate, nil
	case "public":
		return gitsrht.VisibilityPublic, nil
	default:
		return "", fmt.Errorf("invalid visibility: %s", visibility)
	}
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

		repo, err := gitsrht.RevsByRepoName(c.Client, ctx, repo)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return repo.References.Tags(), cobra.ShellCompDirectiveNoFileComp
	}

	output, err := exec.Command("git", "tag").Output()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	revs := strings.Split(string(output), "\n")
	return revs, cobra.ShellCompDirectiveNoFileComp
}
