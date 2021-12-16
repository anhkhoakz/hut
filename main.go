package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"time"

	"git.sr.ht/~emersion/gqlclient"
	"git.sr.ht/~emersion/hut/srht/pastesrht"
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

			paste, err := pastesrht.CreatePaste(c.Client, ctx, files)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("%v/%v/%v\n", c.BaseURL, paste.User.CanonicalName, paste.Id)
		},
	}

	buildCmd := &cobra.Command{
		Use:   "build [manifest...]",
		Short: "Submit a build manifest",
		Run: func(cmd *cobra.Command, args []string) {
			c := createClient("builds")

			filenames := args
			if len(args) == 0 {
				if _, err := os.Stat(".build.yml"); err == nil {
					filenames = append(filenames, ".build.yml")
				}
				if matches, err := filepath.Glob(".build/*.yml"); err == nil {
					filenames = append(filenames, matches...)
				}
			}

			if len(filenames) == 0 {
				log.Fatal("no build manifest found")
			}

			for _, name := range filenames {
				var b []byte
				var err error
				if name == "-" {
					b, err = io.ReadAll(os.Stdin)
				} else {
					b, err = os.ReadFile(name)
				}
				if err != nil {
					log.Fatalf("failed to read manifest from %q: %v", name, err)
				}

				op := gqlclient.NewOperation(`mutation ($manifest: String!) {
					submit(manifest: $manifest) {
						id
						owner { canonicalName }
					}
				}`)
				op.Var("manifest", string(b))

				// TODO: use generated types
				var respData struct {
					Submit struct {
						Id    int
						Owner struct {
							CanonicalName string
						}
					}
				}
				if err := c.Execute(ctx, op, &respData); err != nil {
					log.Fatal(err)
				}

				fmt.Printf("%v/%v/job/%v\n", c.BaseURL, respData.Submit.Owner.CanonicalName, respData.Submit.Id)
			}
		},
	}

	rootCmd := &cobra.Command{
		Use:   "hut",
		Short: "hut is a CLI tool for sr.ht",
	}
	rootCmd.AddCommand(pasteCmd)
	rootCmd.AddCommand(buildCmd)

	rootCmd.Execute()
}
