//go:build generate

package srht

import (
	_ "git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen"
)

//go:generate go run git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen -s pastesrht/schema.graphqls -n pastesrht -o pastesrht/gql.go
