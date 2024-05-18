package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"

	"git.sr.ht/~xenrox/hut/termfmt"
	"github.com/google/shlex"
	"github.com/spf13/cobra"
)

// ownerPrefixes is the set of characters used to prefix sr.ht owners. "~" is
// used to indicate users.
const ownerPrefixes = "~"

const dateLayout = "Mon, 02 Jan 2006 15:04:05 -0700"

const fileTransferTimeout = 10 * time.Minute

func main() {
	log.SetFlags(0) // disable date/time prefix

	ctx := context.Background()

	cmd := &cobra.Command{
		Use:               "hut",
		Short:             "hut is a CLI tool for sr.ht",
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	}
	cmd.PersistentFlags().String("instance", "", "sr.ht instance to use")
	cmd.RegisterFlagCompletionFunc("instance", cobra.NoFileCompletions)
	cmd.PersistentFlags().String("config", "", "config file to use")

	cmd.AddCommand(newBuildsCommand())
	cmd.AddCommand(newExportCommand())
	cmd.AddCommand(newGitCommand())
	cmd.AddCommand(newGraphqlCommand())
	cmd.AddCommand(newHgCommand())
	cmd.AddCommand(newImportCommand())
	cmd.AddCommand(newInitCommand())
	cmd.AddCommand(newListsCommand())
	cmd.AddCommand(newMetaCommand())
	cmd.AddCommand(newPagesCommand())
	cmd.AddCommand(newPasteCommand())
	cmd.AddCommand(newTodoCommand())

	if err := cmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

var completeVisibility = cobra.FixedCompletions([]string{"public", "unlisted", "private"}, cobra.ShellCompDirectiveNoFileComp)

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

func parseOwnerName(name string) (owner, instance string) {
	name = stripProtocol(name)
	parsed := strings.Split(name, "/")
	switch len(parsed) {
	case 1:
		owner = name
	case 2:
		instance = parsed[0]
		owner = parsed[1]

		if strings.IndexAny(owner, ownerPrefixes) != 0 {
			log.Fatalf("Invalid owner name %q: must start with %q", owner, ownerPrefixes)
		}
	default:
		log.Fatalf("Invalid owner name %q", name)
	}

	return owner, instance
}

func parseResourceName(name string) (resource, owner, instance string) {
	name = stripProtocol(name)
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

func getInputWithEditor(pattern, initialText string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return "", errors.New("EDITOR not set")
	}

	commandSplit, err := shlex.Split(editor)
	if err != nil {
		return "", err
	}

	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())

	if initialText != "" {
		_, err = file.WriteString(initialText)
		if err != nil {
			return "", err
		}
	}

	err = file.Close()
	if err != nil {
		return "", err
	}

	commandSplit = append(commandSplit, file.Name())
	cmd := exec.Command(commandSplit[0], commandSplit[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(file.Name())
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func dropComment(text, comment string) string {
	// Drop our prefilled comment, but without stripping leading
	// whitespace
	text = strings.TrimRightFunc(text, unicode.IsSpace)
	text = strings.TrimSuffix(text, comment)
	text = strings.TrimRightFunc(text, unicode.IsSpace)
	return text
}

func stripProtocol(s string) string {
	i := strings.Index(s, "://")
	if i != -1 {
		s = s[i+3:]
	}

	return s
}

func hasCmdArg(cmd *cobra.Command, arg string) bool {
	for _, v := range cmd.Flags().Args() {
		if v == arg {
			return true
		}
	}

	return false
}

func readWebhookQuery(stdin bool) string {
	var query string

	// Disable $EDITOR support when not in interactive terminal
	if !termfmt.IsTerminal() {
		stdin = true
	}

	if stdin {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("failed to read webhook query: %v", err)
		}
		query = string(b)
	} else {
		var err error
		query, err = getInputWithEditor("hut_query*.graphql", "")
		if err != nil {
			log.Fatalf("failed to read webhook query: %v", err)
		}
	}

	if query == "" {
		log.Println("Aborting due to empty query.")
		os.Exit(1)
	}
	return query
}
