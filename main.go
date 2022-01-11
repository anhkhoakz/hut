package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
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
	cmd.PersistentFlags().StringVar(&instanceName, "instance", "", "srht instance to use")
	cmd.RegisterFlagCompletionFunc("instance", cobra.NoFileCompletions)
	cmd.PersistentFlags().StringVar(&configFile, "config", "", "config file to use")

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

func completeVisibility(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"public", "unlisted", "private"}, cobra.ShellCompDirectiveNoFileComp
}

func getConfirmation(msg string) bool {
	fmt.Printf("%s [y/N]: ", msg)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}

	input = strings.ToLower(strings.TrimSpace(input))
	if input == "yes" || input == "y" {
		return true
	}

	return false
}
