package main

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := &cobra.Command{
		Use:               "hut",
		Short:             "hut is a CLI tool for sr.ht",
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	}
	cmd.AddCommand(newBuildsCommand())
	cmd.AddCommand(newGitCommand())
	cmd.AddCommand(newGraphqlCommand())
	cmd.AddCommand(newMetaCommand())
	cmd.AddCommand(newPasteCommand())
	cmd.AddCommand(newPagesCommand())

	if err := cmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
