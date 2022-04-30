package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"git.sr.ht/~emersion/gqlclient"
	"github.com/spf13/cobra"

	"git.sr.ht/~emersion/hut/srht/pastesrht"
	"git.sr.ht/~emersion/hut/termfmt"
)

func newPasteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "paste",
		Short: "Use the paste API",
	}
	cmd.AddCommand(newPasteCreateCommand())
	cmd.AddCommand(newPasteDeleteCommand())
	cmd.AddCommand(newPasteListCommand())
	cmd.AddCommand(newPasteShowCommand())
	cmd.AddCommand(newPasteUpdateCommand())
	return cmd
}

func newPasteCreateCommand() *cobra.Command {
	var visibility string
	var name string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		pasteVisibility, err := pastesrht.ParseVisibility(visibility)
		if err != nil {
			log.Fatal(err)
		}

		c := createClient("paste", cmd)

		if name != "" && len(args) > 0 {
			log.Fatalln("--name is only supported when reading from stdin")
		}

		var files []gqlclient.Upload
		for _, filename := range args {
			f, err := os.Open(filename)
			if err != nil {
				log.Fatalf("failed to open input file: %v", err)
			}
			defer f.Close()

			t := mime.TypeByExtension(filename)
			if t == "" {
				t = "text/plain"
			}

			files = append(files, gqlclient.Upload{
				Filename: filepath.Base(filename),
				MIMEType: t,
				Body:     f,
			})
		}

		if len(args) == 0 {
			files = append(files, gqlclient.Upload{
				Filename: name,
				MIMEType: "text/plain",
				Body:     os.Stdin,
			})
		}

		paste, err := pastesrht.CreatePaste(c.Client, ctx, files, pasteVisibility)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("%v/%v/%v\n", c.BaseURL, paste.User.CanonicalName, paste.Id)
	}

	cmd := &cobra.Command{
		Use:   "create [filenames...]",
		Short: "Create a new paste",
		Run:   run,
	}
	cmd.Flags().StringVarP(&visibility, "visibility", "v", "unlisted", "paste visibility")
	cmd.RegisterFlagCompletionFunc("visibility", completeVisibility)
	cmd.Flags().StringVarP(&name, "name", "n", "", "paste name (when reading from stdin)")
	return cmd
}

func newPasteDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("paste", cmd)

		for _, id := range args {
			paste, err := pastesrht.Delete(c.Client, ctx, id)
			if err != nil {
				log.Fatalf("failed to delete paste %s: %v", id, err)
			}

			if paste == nil {
				fmt.Printf("Paste %s does not exist\n", id)
			} else {
				fmt.Printf("Deleted paste %s\n", paste.Id)
			}
		}
	}

	cmd := &cobra.Command{
		Use:               "delete <ID...>",
		Short:             "Delete pastes",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completePasteID,
		Run:               run,
	}
	return cmd
}

func newPasteListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("paste", cmd)

		pastes, err := pastesrht.Pastes(c.Client, ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, paste := range pastes.Results {
			fmt.Printf("%s %s %s ago\n", termfmt.DarkYellow.Sprint(paste.Id),
				paste.Visibility.TermString(), timeDelta(paste.Created))
			for _, file := range paste.Files {
				if file.Filename != nil && *file.Filename != "" {
					fmt.Printf("  %s\n", *file.Filename)
				}
			}

			fmt.Println()
		}
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pastes",
		Run:   run,
	}
	return cmd
}

func newPasteShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "show <id>",
		Short:             "Display a paste",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completePasteID,
	}
	cmd.Run = func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		id, _, instance := parseResourceName(args[0])
		c := createClientWithInstance("paste", cmd, instance)

		paste, err := pastesrht.ShowPaste(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if paste == nil {
			log.Fatalf("Paste %q does not exist", id)
		}

		fmt.Printf("%s %s %s ago\n", termfmt.DarkYellow.Sprint(paste.Id),
			paste.Visibility.TermString(), timeDelta(paste.Created))

		for _, file := range paste.Files {
			fmt.Print("\n■ ")
			if file.Filename != nil && *file.Filename != "" {
				fmt.Println(termfmt.Bold.String(*file.Filename))
			} else {
				fmt.Println(termfmt.Dim.String("(untitled)"))
			}
			fmt.Println()

			fetchPasteFile(ctx, c.HTTP, file)
		}
	}
	return cmd
}

func fetchPasteFile(ctx context.Context, c *http.Client, file *pastesrht.File) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(file.Contents), nil)
	if err != nil {
		log.Fatalf("Failed to create request to fetch file: %v", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		log.Fatalf("Failed to fetch file: %v", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		log.Fatalf("Failed to copy to stdout: %v", err)
	}
}

func newPasteUpdateCommand() *cobra.Command {
	var visibility string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("paste", cmd)

		pasteVisibility, err := pastesrht.ParseVisibility(visibility)
		if err != nil {
			log.Fatal(err)
		}

		paste, err := pastesrht.Update(c.Client, ctx, args[0], pasteVisibility)
		if err != nil {
			log.Fatal(err)
		}

		if paste == nil {
			log.Fatalf("Paste %s does not exist\n", args[0])
		}

		fmt.Printf("Updated paste %s visibility to %s\n", paste.Id, pasteVisibility)
	}

	cmd := &cobra.Command{
		Use:               "update <ID>",
		Short:             "Update a paste's visibility",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completePasteID,
		Run:               run,
	}
	cmd.Flags().StringVarP(&visibility, "visibility", "v", "", "paste visibility")
	cmd.MarkFlagRequired("visibility")
	cmd.RegisterFlagCompletionFunc("visibility", completeVisibility)
	return cmd
}

func completePasteID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("paste", cmd)
	var pasteList []string

	pastes, err := pastesrht.PasteCompletionList(c.Client, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, paste := range pastes.Results {
		str := paste.Id
		var files string

		for i, file := range paste.Files {
			if *file.Filename != "" {
				if i != 0 {
					files += ", "
				}
				files += *file.Filename
			}
		}

		if files != "" {
			str += fmt.Sprintf("\t%s", files)
		}

		pasteList = append(pasteList, str)
	}

	return pasteList, cobra.ShellCompDirectiveNoFileComp
}
