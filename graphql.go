package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"git.sr.ht/~emersion/gqlclient"
	"git.sr.ht/~emersion/hut/termfmt"
	"github.com/spf13/cobra"
)

const graphqlPrefill = `
# Please write the GraphQL query you want to execute above. The GraphQL schema
# for %v.sr.ht is available at:
# %v`

func newGraphqlCommand() *cobra.Command {
	var stringVars []string
	var stdin bool
	run := func(cmd *cobra.Command, args []string) {
		service := args[0]

		ctx := cmd.Context()
		c := createClient(service, cmd)

		// Disable $EDITOR support when not in interactive terminal
		if !termfmt.IsTerminal() {
			stdin = true
		}

		var query string
		if stdin {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				log.Fatalf("failed to read GraphQL query: %v", err)
			}
			query = string(b)
		} else {
			prefill := fmt.Sprintf(graphqlPrefill, service, graphqlSchemaURL(service))

			var err error
			query, err = getInputWithEditor("hut_query*.graphql", prefill)
			if err != nil {
				log.Fatalf("failed to read GraphQL query: %v", err)
			}

			query = dropComment(query, prefill)
		}

		if strings.TrimSpace(query) == "" {
			fmt.Fprintf(os.Stderr, "Aborting due to empty query")
			os.Exit(1)
		}

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
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read query from stdin")
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

func graphqlSchemaURL(service string) string {
	var filename string
	switch service {
	case "pages":
		filename = "graph/schema.graphqls"
	default:
		filename = "api/graph/schema.graphqls"
	}
	return fmt.Sprintf("https://git.sr.ht/~sircmpwn/%v.sr.ht/tree/master/item/%v", service, filename)
}
