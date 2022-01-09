package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
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
	cmd.AddCommand(newMetaPGPKeyCommand())
	return cmd
}

func newMetaShowCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {

		ctx := cmd.Context()
		c := createClient("meta")

		var (
			user *metasrht.User
			err  error
		)
		if len(args) > 0 {
			username := strings.TrimLeft(args[0], "~")
			user, err = metasrht.FetchUser(c.Client, ctx, username)
		} else {
			user, err = metasrht.FetchMe(c.Client, ctx)
		}
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
		Use:               "show [user]",
		Short:             "Show a user profile",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
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

func newMetaPGPKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pgp-key",
		Short: "Manage PGP keys",
	}
	cmd.AddCommand(newMetaPGPKeyCreateCommand())
	return cmd
}

func newMetaPGPKeyCreateCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta")

		var (
			keyBytes []byte
			err      error
		)
		if len(args) > 0 {
			keyBytes, err = ioutil.ReadFile(args[0])
		} else {
			keyBytes, err = exportDefaultPGPKey(ctx)
		}
		if err != nil {
			log.Fatal(err)
		}

		// Sanity check, mostly to avoid uploading private keys
		if !strings.HasPrefix(string(keyBytes), "-----BEGIN PGP PUBLIC KEY BLOCK-----") {
			log.Fatalf("input doesn't look like a PGP public key file")
		}

		key, err := metasrht.CreatePGPKey(c.Client, ctx, string(keyBytes))
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Uploaded PGP public key %v\n", key.Fingerprint)
	}

	return &cobra.Command{
		Use:   "create [path]",
		Short: "Create a new PGP key",
		Args:  cobra.MaximumNArgs(1),
		Run:   run,
	}
}

func exportDefaultPGPKey(ctx context.Context) ([]byte, error) {
	out, err := exec.Command("gpg", "--list-secret-keys", "--with-colons").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list keys in GPG keyring: %v", err)
	}

	var keyID string
	for _, l := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(l, "sec:") {
			continue
		}

		if keyID != "" {
			return nil, fmt.Errorf("multiple keys found in GPG keyring")
		}

		fields := strings.Split(l, ":")
		if len(fields) <= 4 {
			continue
		}
		keyID = fields[4]
	}
	if keyID == "" {
		return nil, fmt.Errorf("no key found in GPG keyring")
	}

	out, err = exec.Command("gpg", "--export", "--armor", "--export-options=export-minimal", keyID).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to export key from GPG kerying: %v", err)
	}

	return out, nil
}
