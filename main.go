package main

import (
	"context"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"time"

	"git.sr.ht/~emersion/gqlclient"
	"git.sr.ht/~emersion/hut/srht"
	"github.com/spf13/cobra"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pasteCmd := &cobra.Command{
		Use:   "paste [filenames...]",
		Short: "Create a new paste",
		Run: func(cmd *cobra.Command, args []string) {
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

			op := gqlclient.NewOperation(`mutation ($files: [Upload!]!) {
				create(files: $files, visibility: UNLISTED) {
					id
					user { canonicalName }
				}
			}`)
			op.Var("files", files)

			var respData struct {
				Create struct {
					srht.Paste
					// TODO: don't assume this Entity is a User
					User srht.User
				}
			}
			if err := c.Execute(ctx, op, &respData); err != nil {
				log.Fatal(err)
			}

			fmt.Printf("%v/%v/%v", c.BaseURL, respData.Create.User.CanonicalName, respData.Create.Id)
		},
	}

	rootCmd := &cobra.Command{
		Use:   "hut",
		Short: "hut is a CLI tool for sr.ht",
	}
	rootCmd.AddCommand(pasteCmd)

	rootCmd.Execute()
}
