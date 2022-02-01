//go:build generate
// +build generate

package srht

import (
	_ "git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen"
)

//go:generate go run git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen -s pastesrht/schema.graphqls -q pastesrht/operations.graphql -o pastesrht/gql.go
//go:generate go run git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen -s buildssrht/schema.graphqls -q buildssrht/operations.graphql -o buildssrht/gql.go
//go:generate go run git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen -s gitsrht/schema.graphqls -q gitsrht/operations.graphql -o gitsrht/gql.go
//go:generate go run git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen -s pagessrht/schema.graphqls -q pagessrht/operations.graphql -o pagessrht/gql.go
//go:generate go run git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen -s metasrht/schema.graphqls -q metasrht/operations.graphql -o metasrht/gql.go
//go:generate go run git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen -s listssrht/schema.graphqls -q listssrht/operations.graphql -o listssrht/gql.go
//go:generate go run git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen -s hgsrht/schema.graphqls -q hgsrht/operations.graphql -o hgsrht/gql.go
//go:generate go run git.sr.ht/~emersion/gqlclient/cmd/gqlclientgen -s todosrht/schema.graphqls -q todosrht/operations.graphql -o todosrht/gql.go
