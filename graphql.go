package main

import (
	"encoding/json"
	"io"
	"log"
	"os"

	"git.sr.ht/~emersion/gqlclient"
	"github.com/spf13/cobra"
)

func newGraphqlCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		service := args[0]

		ctx := cmd.Context()
		c := createClient(service)

		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("failed to read GraphQL query: %v", err)
		}
		query := string(b)

		op := gqlclient.NewOperation(query)
		var data json.RawMessage
		if err := c.Execute(ctx, op, &data); err != nil {
			log.Fatal(err)
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			log.Fatalf("failed to write JSON response: %v", err)
		}
	}

	cmd := &cobra.Command{
		Use:   "graphql <service>",
		Short: "Execute a GraphQL query",
		Args:  cobra.ExactArgs(1),
		Run:   run,
	}
	// TODO: variables
	return cmd
}
