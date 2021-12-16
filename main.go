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
	"git.sr.ht/~emersion/hut/srht/buildssrht"
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

	var follow bool
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
			if len(filenames) > 1 && follow {
				log.Fatal("--follow cannot be used when submitting multiple jobs")
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

				job, err := buildssrht.Submit(c.Client, ctx, string(b))
				if err != nil {
					log.Fatal(err)
				}

				fmt.Printf("%v/%v/job/%v\n", c.BaseURL, job.Owner.CanonicalName, job.Id)

				if follow {
					job, err := c.followJob(context.Background(), job.Id)
					if err != nil {
						log.Fatal(err)
					}
					if job.Status != buildssrht.JobStatusSuccess {
						os.Exit(1)
					}
				}
			}
		},
	}
	buildCmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow build logs")

	rootCmd := &cobra.Command{
		Use:   "hut",
		Short: "hut is a CLI tool for sr.ht",
	}
	rootCmd.AddCommand(pasteCmd)
	rootCmd.AddCommand(buildCmd)

	rootCmd.Execute()
}
