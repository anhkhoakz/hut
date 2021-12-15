package main

import (
	"context"
	"log"
	"os"

	"git.sr.ht/~emersion/gqlclient"
	"git.sr.ht/~emersion/hut/srht"
	"github.com/spf13/cobra"
)

func main() {
	ctx := context.Background()

	pasteCmd := &cobra.Command{
		Use:   "paste",
		Short: "Create a new paste",
		Run: func(cmd *cobra.Command, args []string) {
			c := createClient("paste")

			op := gqlclient.NewOperation(`mutation ($upload: Upload!) {
				create(files: [$upload], visibility: UNLISTED) { id }
			}`)
			op.Var("upload", gqlclient.Upload{
				Filename: "-",
				MIMEType: "text/plain",
				Body:     os.Stdin,
			})

			var respData struct {
				Create srht.Paste
			}
			if err := c.Execute(ctx, op, &respData); err != nil {
				log.Fatal(err)
			}

			log.Println(respData.Create.Id)
		},
	}

	rootCmd := &cobra.Command{
		Use:   "hut",
		Short: "hut is a CLI tool for sr.ht",
	}
	rootCmd.AddCommand(pasteCmd)

	rootCmd.Execute()
}
