package main

import (
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"

	"git.sr.ht/~emersion/gqlclient"
	"github.com/spf13/cobra"

	"git.sr.ht/~emersion/hut/srht/pastesrht"
)

func newPasteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "paste",
		Short: "Use the paste API",
	}
	cmd.AddCommand(newPasteCreateCommand())
	return cmd
}

func newPasteCreateCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("paste")

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
				Filename: "-",
				MIMEType: "text/plain",
				Body:     os.Stdin,
			})
		}

		paste, err := pastesrht.CreatePaste(c.Client, ctx, files)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("%v/%v/%v\n", c.BaseURL, paste.User.CanonicalName, paste.Id)
	}

	return &cobra.Command{
		Use:   "create [filenames...]",
		Short: "Create a new paste",
		Run:   run,
	}
}
