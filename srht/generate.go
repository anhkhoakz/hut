//go:build generate

package srht

import (
	_ "git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen"
)

//go:generate go run git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen -s pastesrht/schema.graphqls -q pastesrht/operations.graphql -n pastesrht -o pastesrht/gql.go
//go:generate go run git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen -s buildssrht/schema.graphqls -q buildssrht/operations.graphql -n buildssrht -o buildssrht/gql.go
