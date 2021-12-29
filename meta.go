package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"git.sr.ht/~emersion/hut/srht/metasrht"
	"git.sr.ht/~emersion/hut/termfmt"
)

func newMetaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meta",
		Short: "Use the meta API",
	}
	cmd.AddCommand(newMetaShowCommand())
	cmd.AddCommand(newMetaSSHKeyCommand())
	return cmd
}

func newMetaShowCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		username := strings.TrimLeft(args[0], "~")

		ctx := cmd.Context()
		c := createClient("meta")

		user, err := metasrht.FetchUser(c.Client, ctx, username)
		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatal("no such user")
		}

		fmt.Printf("%v <%v>\n", termfmt.String(user.CanonicalName, termfmt.Bold), user.Email)
		if user.Url != nil {
			fmt.Println(*user.Url)
		}
		if user.Location != nil {
			fmt.Println(*user.Location)
		}
		if user.Bio != nil {
			fmt.Printf("\n%v\n", *user.Bio)
		}
	}

	cmd := &cobra.Command{
		Use:   "show <user>",
		Short: "Show a user profile",
		Args:  cobra.ExactArgs(1),
		Run:   run,
	}
	return cmd
}

func newMetaSSHKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh-key",
		Short: "Manage SSH keys",
	}
	cmd.AddCommand(newMetaSSHKeyCreateCommand())
	return cmd
}

func newMetaSSHKeyCreateCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta")

		var filename string
		if len(args) > 0 {
			filename = args[0]
		} else {
			var err error
			filename, err = guessSSHPubKeyFilename()
			if err != nil {
				log.Fatalf("failed to find default SSH public key: %v", err)
			}
		}

		b, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Fatal(err)
		}

		// Sanity check, mostly to avoid uploading private keys
		if !strings.HasPrefix(string(b), "ssh-") {
			log.Fatalf("%q doesn't look like an SSH public key file", filename)
		}

		key, err := metasrht.CreateSSHKey(c.Client, ctx, string(b))
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Uploaded SSH public key %v", key.Fingerprint)
		if key.Comment != nil {
			fmt.Printf(" (%v)", *key.Comment)
		}
		fmt.Println()
	}

	cmd := &cobra.Command{
		Use:   "create [path]",
		Short: "Create a new SSH key",
		Args:  cobra.MaximumNArgs(1),
		Run:   run,
	}
	return cmd
}

func guessSSHPubKeyFilename() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	var match string
	for _, name := range []string{"id_ed25519", "id_rsa"} {
		filename := filepath.Join(homeDir, ".ssh", name+".pub")
		if _, err := os.Stat(filename); err == nil {
			if match != "" {
				return "", fmt.Errorf("multiple SSH public keys found")
			}
			match = filename
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	if match == "" {
		return "", fmt.Errorf("no SSH public key found")
	}

	return match, nil
}
