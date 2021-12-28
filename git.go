package main

import (
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
)

func newGitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "Use the git API",
	}
	cmd.AddCommand(newGitArtifactCommand())
	return cmd
}

func newGitArtifactCommand() *cobra.Command {
	var repoName, rev string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("git")

		if repoName == "" {
			var err error
			repoName, err = guessGitRepoName(c)
			if err != nil {
				log.Fatal(err)
			}
		}
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
		Use:   "artifact <filename>",
		Short: "Upload an artifact",
		Args:  cobra.ExactArgs(1),
		Run:   run,
	}
	cmd.Flags().StringVarP(&repoName, "repo", "r", "", "name of repository")
	cmd.Flags().StringVar(&rev, "rev", "", "revision tag")
	return cmd
}

func guessGitRepoName(c *Client) (string, error) {
	remoteURL, err := gitRemoteURL()
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

func gitRemoteURL() (*url.URL, error) {
	// TODO: iterate over all remotes, find one which matches the config file, etc
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
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
