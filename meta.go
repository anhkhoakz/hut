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
	cmd.AddCommand(newMetaAuditLogCommand())
	cmd.AddCommand(newMetaSSHKeyCommand())
	cmd.AddCommand(newMetaPGPKeyCommand())
	cmd.AddCommand(newMetaUserWebhookCommand())
	return cmd
}

func newMetaShowCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var (
			user *metasrht.User
			err  error
		)
		if len(args) > 0 {
			owner, instance := parseOwnerName(args[0])
			c := createClientWithInstance("meta", cmd, instance)
			username := strings.TrimLeft(owner, ownerPrefixes)
			user, err = metasrht.FetchUser(c.Client, ctx, username)
		} else {
			c := createClient("meta", cmd)
			user, err = metasrht.FetchMe(c.Client, ctx)
		}
		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatal("no such user")
		}

		fmt.Printf("%v <%v>\n", termfmt.Bold.String(user.CanonicalName), user.Email)
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

func newMetaAuditLogCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

		logs, err := metasrht.AuditLog(c.Client, ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, log := range logs.Results {
			entry := log.IpAddress
			if log.Details != nil {
				entry += fmt.Sprintf(" %s ", *log.Details)
			} else {
				entry += fmt.Sprintf(" %s ", log.EventType)
			}
			entry += fmt.Sprintf("%s ago", timeDelta(log.Created))
			fmt.Println(entry)
		}
	}

	cmd := &cobra.Command{
		Use:   "audit-log",
		Short: "Display your audit log",
		Args:  cobra.ExactArgs(0),
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
	cmd.AddCommand(newMetaSSHKeyDeleteCommand())
	cmd.AddCommand(newMetaSSHKeyListCommand())
	return cmd
}

func newMetaSSHKeyCreateCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

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

func newMetaSSHKeyDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		key, err := metasrht.DeleteSSHKey(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Deleted SSH key %s\n", key.Fingerprint)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete an SSH key",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSSHKeys,
		Run:               run,
	}
	return cmd
}

func newMetaSSHKeyListCommand() *cobra.Command {
	var raw bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

		var (
			user *metasrht.User
			err  error
		)

		if len(args) > 0 {
			username := strings.TrimLeft(args[0], ownerPrefixes)
			if raw {
				user, err = metasrht.ListRawSSHKeysByUser(c.Client, ctx, username)
			} else {
				user, err = metasrht.ListSSHKeysByUser(c.Client, ctx, username)
			}
		} else {
			if raw {
				user, err = metasrht.ListRawSSHKeys(c.Client, ctx)
			} else {
				user, err = metasrht.ListSSHKeys(c.Client, ctx)
			}
		}
		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatal("no such user")
		}

		if raw {
			for _, key := range user.SshKeys.Results {
				fmt.Println(key.Key)
			}
		} else {
			for _, key := range user.SshKeys.Results {
				fmt.Printf("#%d: %s\n", key.Id, key.Fingerprint)
				if key.Comment != nil {
					fmt.Printf("  %s\n", *key.Comment)
				}
				fmt.Println()
			}
		}
	}

	cmd := &cobra.Command{
		Use:               "list [user]",
		Short:             "List SSH keys",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&raw, "raw", "r", false, "print raw public keys")
	return cmd
}

func newMetaPGPKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pgp-key",
		Short: "Manage PGP keys",
	}
	cmd.AddCommand(newMetaPGPKeyCreateCommand())
	cmd.AddCommand(newMetaPGPKeyDeleteCommand())
	cmd.AddCommand(newMetaPGPKeyListCommand())
	return cmd
}

func newMetaPGPKeyCreateCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

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

func newMetaPGPKeyDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		key, err := metasrht.DeletePGPKey(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Deleted PGP key %s\n", key.Fingerprint)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a PGP key",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completePGPKeys,
		Run:               run,
	}
	return cmd
}

func newMetaPGPKeyListCommand() *cobra.Command {
	var raw bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

		var (
			user *metasrht.User
			err  error
		)

		if len(args) > 0 {
			username := strings.TrimLeft(args[0], ownerPrefixes)
			if raw {
				user, err = metasrht.ListRawPGPKeysByUser(c.Client, ctx, username)
			} else {
				user, err = metasrht.ListPGPKeysByUser(c.Client, ctx, username)
			}
		} else {
			if raw {
				user, err = metasrht.ListRawPGPKeys(c.Client, ctx)
			} else {
				user, err = metasrht.ListPGPKeys(c.Client, ctx)
			}
		}
		if err != nil {
			log.Fatal(err)
		} else if user == nil {
			log.Fatal("no such user")
		}

		if raw {
			for _, key := range user.PgpKeys.Results {
				fmt.Println(key.Key)
			}
		} else {
			for _, key := range user.PgpKeys.Results {
				fmt.Printf("#%d: %s\n", key.Id, key.Fingerprint)
				fmt.Println()
			}
		}
	}

	cmd := &cobra.Command{
		Use:               "list [user]",
		Short:             "List PGP keys",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().BoolVarP(&raw, "raw", "r", false, "print raw public keys")
	return cmd

}

func newMetaUserWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user-webhook",
		Short: "Manage user webhooks",
	}
	cmd.AddCommand(newMetaUserWebhookCreateCommand())
	cmd.AddCommand(newMetaUserWebhookListCommand())
	cmd.AddCommand(newMetaUserWebhookDeleteCommand())
	return cmd
}

func newMetaUserWebhookCreateCommand() *cobra.Command {
	var events []string
	var stdin bool
	var url string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

		var config metasrht.ProfileWebhookInput
		config.Url = url
		config.Query = readWebhookQuery(stdin)

		whEvents, err := metasrht.ParseUserEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents

		webhook, err := metasrht.CreateUserWebhook(c.Client, ctx, config)
		if err != nil {
			log.Fatal(err)
		} else if webhook == nil {
			log.Fatal("failed to create webhook")
		}

		fmt.Printf("Created user webhook with ID %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "create",
		Short:             "Create a user webhook",
		Args:              cobra.ExactArgs(0),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().StringSliceVarP(&events, "events", "e", nil, "webhook events")
	cmd.RegisterFlagCompletionFunc("events", completeMetaUserWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newMetaUserWebhookListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

		webhooks, err := metasrht.UserWebhooks(c.Client, ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, webhook := range webhooks.Results {
			fmt.Printf("%s %s %s\n", termfmt.DarkYellow.Sprintf("#%d", webhook.Id),
				webhook.Url, webhook.Events)
			fmt.Println(indent(webhook.Query, "  "))
		}
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List user webhooks",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	return cmd
}

func newMetaUserWebhookDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		webhook, err := metasrht.DeleteUserWebhook(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		} else if webhook == nil {
			log.Fatal("failed to delete webhook")
		}

		fmt.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a user webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

func completeSSHKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("meta", cmd)
	var keyList []string

	user, err := metasrht.ListSSHKeys(c.Client, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, key := range user.SshKeys.Results {
		str := fmt.Sprintf("%d\t%s", key.Id, key.Fingerprint)
		if key.Comment != nil {
			str += fmt.Sprintf(" %s", *key.Comment)
		}
		keyList = append(keyList, str)
	}

	return keyList, cobra.ShellCompDirectiveNoFileComp
}

func completePGPKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("meta", cmd)
	var keyList []string

	user, err := metasrht.ListPGPKeys(c.Client, ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, key := range user.PgpKeys.Results {
		str := fmt.Sprintf("%d\t%s", key.Id, key.Fingerprint)
		keyList = append(keyList, str)
	}

	return keyList, cobra.ShellCompDirectiveNoFileComp
}

func completeMetaUserWebhookEvents(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var eventList []string
	events := [5]string{"profile_update", "pgp_key_added", "pgp_key_removed", "ssh_key_added", "ssh_key_removed"}
	set := strings.ToLower(cmd.Flag("events").Value.String())
	for _, event := range events {
		if !strings.Contains(set, event) {
			eventList = append(eventList, event)
		}
	}
	return eventList, cobra.ShellCompDirectiveNoFileComp
}
