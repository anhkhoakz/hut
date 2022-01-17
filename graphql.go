package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"

	"git.sr.ht/~emersion/gqlclient"
	"github.com/spf13/cobra"
)

func newGraphqlCommand() *cobra.Command {
	var stringVars []string
	run := func(cmd *cobra.Command, args []string) {
		service := args[0]

		ctx := cmd.Context()
		c := createClient(service, cmd)

		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("failed to read GraphQL query: %v", err)
		}
		query := string(b)

		op := gqlclient.NewOperation(query)

		for _, kv := range stringVars {
			op.Var(splitKeyValue(kv))
		}

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
		Use:               "graphql <service>",
		Short:             "Execute a GraphQL query",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringSliceVarP(&stringVars, "var", "v", nil, "set string variable")
	// TODO: JSON and file variables
	return cmd
}

func splitKeyValue(kv string) (string, string) {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		log.Fatalf("in variable definition %q: missing equal sign", kv)
	}
	return parts[0], parts[1]
}
