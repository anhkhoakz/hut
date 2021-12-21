package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

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

		if repoName == "" {
			log.Fatal("enter a repository name with --repo")
		}
		if rev == "" {
			log.Fatal("enter a revision name with --rev")
		}

		c := createClient("git")

		if len(args) == 0 {
			log.Fatal("enter a file to upload")
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
		Run:   run,
	}
	cmd.Flags().StringVarP(&repoName, "repo", "r", "", "name of repository")
	cmd.Flags().StringVar(&rev, "rev", "", "revision tag")
	return cmd
}
