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

// ownerPrefixes is the set of characters used to prefix sr.ht owners. "~" is
// used to indicate users.
const ownerPrefixes = "~"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := &cobra.Command{
		Use:               "hut",
		Short:             "hut is a CLI tool for sr.ht",
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	}
	cmd.PersistentFlags().String("instance", "", "srht instance to use")
	cmd.RegisterFlagCompletionFunc("instance", cobra.NoFileCompletions)
	cmd.PersistentFlags().String("config", "", "config file to use")

	cmd.AddCommand(newBuildsCommand())
	cmd.AddCommand(newGitCommand())
	cmd.AddCommand(newGraphqlCommand())
	cmd.AddCommand(newHgCommand())
	cmd.AddCommand(newListsCommand())
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
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [y/n]: ", msg)

		input, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		switch strings.ToLower(strings.TrimSpace(input)) {
		case "yes", "y":
			return true
		case "no", "n":
			return false
		default:
			fmt.Println(`Expected "yes" or "no"`)
		}
	}
}

func timeDelta(d time.Duration) string {
	switch {
	case d > time.Hour*24*30:
		return fmt.Sprintf("%.f months", d.Hours()/(24*30))
	case d > time.Hour*24:
		return fmt.Sprintf("%.f days", d.Hours()/24)
	case d > time.Hour:
		return fmt.Sprintf("%.f hours", d.Hours())
	case d > time.Minute:
		return fmt.Sprintf("%.f minutes", d.Minutes())
	}

	return fmt.Sprintf("%.f seconds", d.Seconds())
}
