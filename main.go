package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := &cobra.Command{
		Use:   "hut",
		Short: "hut is a CLI tool for sr.ht",
	}
	cmd.AddCommand(newBuildsCommand())
	cmd.AddCommand(newGitCommand())
	cmd.AddCommand(newPasteCommand())

	cmd.ExecuteContext(ctx)
}
