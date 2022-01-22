package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ownerPrefixes is the set of characters used to prefix sr.ht owners. "~" is
// used to indicate users.
const ownerPrefixes = "~"

const dateLayout = "Mon, 02 Jan 2006 15:04:05 -0700"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := &cobra.Command{
		Use:               "hut",
		Short:             "hut is a CLI tool for sr.ht",
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	}
	cmd.PersistentFlags().String("instance", "", "sr.ht instance to use")
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

func timeDelta(t time.Time) string {
	d := time.Since(t)
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

func parseResourceName(name string) (resource, owner, instance string) {
	i := strings.Index(name, "://")
	if i != -1 {
		name = name[i+3:]
	}

	parsed := strings.Split(name, "/")
	if len(parsed) == 1 {
		return strings.TrimLeft(parsed[0], "#"), owner, instance
	}

	if len(parsed) > 2 && strings.IndexAny(parsed[1], ownerPrefixes) == 0 {
		instance = parsed[0]
		owner = parsed[1]
		resource = strings.Join(parsed[2:], "/")
	} else if strings.IndexAny(parsed[0], ownerPrefixes) == 0 {
		owner = parsed[0]
		resource = strings.Join(parsed[1:], "/")
	} else {
		resource = strings.Join(parsed, "/")
	}

	return resource, owner, instance
}

func parseInt32(s string) (int32, error) {
	i, err := strconv.ParseInt(s, 10, 32)
	return int32(i), err
}
