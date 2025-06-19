package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/juju/ansiterm/tabwriter"
	"github.com/spf13/cobra"

	"git.sr.ht/~xenrox/hut/srht/metasrht"
	"git.sr.ht/~xenrox/hut/termfmt"
)

func newMetaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meta",
		Short: "Use the meta API",
	}
	cmd.AddCommand(newMetaShowCommand())
	cmd.AddCommand(newMetaAuditLogCommand())
	cmd.AddCommand(newMetaUpdateCommand())
	cmd.AddCommand(newMetaSSHKeyCommand())
	cmd.AddCommand(newMetaPGPKeyCommand())
	cmd.AddCommand(newMetaUserWebhookCommand())
	cmd.AddCommand(newMetaOAuthCommand())
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
	var count int
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)
		var cursor *metasrht.Cursor

		err := pagerify(func(p pager) error {
			logs, err := metasrht.AuditLog(c.Client, ctx, cursor)
			if err != nil {
				return err
			}

			for _, log := range logs.Results {
				printAuditLog(p, &log)
			}

			cursor = logs.Cursor
			if p.IsDone(cursor, len(logs.Results)) {
				return pagerDone
			}

			return nil
		}, count)
		if err != nil {
			log.Fatal(err)
		}
	}

	cmd := &cobra.Command{
		Use:   "audit-log",
		Short: "Display your audit log",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	cmd.Flags().IntVar(&count, "count", 0, "number of log entries to fetch")
	cmd.RegisterFlagCompletionFunc("count", cobra.NoFileCompletions)
	return cmd
}

func printAuditLog(w io.Writer, log *metasrht.AuditLogEntry) {
	s := log.IpAddress
	if log.Details != nil {
		s += fmt.Sprintf("\t%s\t", *log.Details)
	} else {
		s += fmt.Sprintf("\t%s\t", log.EventType)
	}
	s += termfmt.Dim.String(humanize.Time(log.Created.Time))

	fmt.Fprintln(w, s)
}

func newMetaUpdateCommand() *cobra.Command {
	var email, location, url string
	var bio bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)
		var input metasrht.UserInput

		if bio {
			if !isStdinTerminal {
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					log.Fatalf("failed to read bio: %v", err)
				}
				biography := string(b)
				input.Bio = &biography
			} else {
				me, err := metasrht.Bio(c.Client, ctx)
				if err != nil {
					log.Fatalf("failed to fetch bio: %v", err)
				}

				var prefill string
				if me.Bio != nil {
					prefill = *me.Bio
				}

				text, err := getInputWithEditor("hut_bio*.md", prefill)
				if err != nil {
					log.Fatalf("failed to read bio: %v", err)
				}

				if strings.TrimSpace(text) == "" {
					_, err := metasrht.ClearBio(c.Client, ctx)
					if err != nil {
						log.Fatalf("failed to clear bio: %v", err)
					}
				} else {
					input.Bio = &text
				}
			}
		}

		if cmd.Flags().Changed("email") {
			input.Email = &email
		}

		if cmd.Flags().Changed("location") {
			if location == "" {
				_, err := metasrht.ClearUserLocation(c.Client, ctx)
				if err != nil {
					log.Fatalf("failed to clear location: %v", err)
				}
			} else {
				input.Location = &location
			}
		}

		if cmd.Flags().Changed("url") {
			if url == "" {
				_, err := metasrht.ClearUserURL(c.Client, ctx)
				if err != nil {
					log.Fatalf("failed to clear URL: %v", err)
				}
			} else {
				input.Url = &url
			}
		}

		_, err := metasrht.UpdateUser(c.Client, ctx, &input)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Successfully updated account")
		if cmd.Flags().Changed("email") {
			log.Printf("An email has been sent to %q to confirm the change\n", email)
		}
	}
	cmd := &cobra.Command{
		Use:               "update",
		Short:             "Update account",
		Args:              cobra.ExactArgs(0),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().BoolVar(&bio, "bio", false, "edit biography")
	cmd.Flags().StringVar(&email, "email", "", "email")
	cmd.RegisterFlagCompletionFunc("email", cobra.NoFileCompletions)
	cmd.Flags().StringVar(&location, "location", "", "location")
	cmd.RegisterFlagCompletionFunc("location", cobra.NoFileCompletions)
	cmd.Flags().StringVar(&url, "url", "", "URL")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
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

		b, err := os.ReadFile(filename)
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

		log.Printf("Uploaded SSH public key %v", key.Fingerprint)
		if key.Comment != nil {
			log.Printf(" (%v)", *key.Comment)
		}
		log.Println()
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

		log.Printf("Deleted SSH key %s\n", key.Fingerprint)
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
	var count int
	var raw bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

		var (
			cursor   *metasrht.Cursor
			user     *metasrht.User
			username string
			err      error
		)
		if len(args) > 0 {
			username = strings.TrimLeft(args[0], ownerPrefixes)
		}

		err = pagerify(func(p pager) error {
			if username != "" {
				if raw {
					user, err = metasrht.ListRawSSHKeysByUser(c.Client, ctx, username, cursor)
				} else {
					user, err = metasrht.ListSSHKeysByUser(c.Client, ctx, username, cursor)
				}
			} else {
				if raw {
					user, err = metasrht.ListRawSSHKeys(c.Client, ctx, cursor)
				} else {
					user, err = metasrht.ListSSHKeys(c.Client, ctx, cursor)
				}
			}

			if err != nil {
				return err
			} else if user == nil {
				return fmt.Errorf("no such user %q", username)
			}

			if raw {
				for _, key := range user.SshKeys.Results {
					fmt.Fprintln(p, key.Key)
				}
			} else {
				for _, key := range user.SshKeys.Results {
					fmt.Fprintf(p, "%s %s\n", termfmt.DarkYellow.Sprintf("#%d", key.Id), key.Fingerprint)
					if key.Comment != nil {
						fmt.Fprintf(p, "  %s\n", *key.Comment)
					}
					fmt.Fprintln(p)
				}
			}

			cursor = user.SshKeys.Cursor
			if p.IsDone(cursor, len(user.SshKeys.Results)) {
				return pagerDone
			}

			return nil
		}, count)
		if err != nil {
			log.Fatal(err)
		}
	}

	cmd := &cobra.Command{
		Use:               "list [user]",
		Short:             "List SSH keys",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().IntVar(&count, "count", 0, "number of keys to fetch")
	cmd.RegisterFlagCompletionFunc("count", cobra.NoFileCompletions)
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
			keyBytes, err = os.ReadFile(args[0])
		} else {
			keyBytes, err = exportDefaultPGPKey()
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

		log.Printf("Uploaded PGP public key %v\n", key.Fingerprint)
	}

	return &cobra.Command{
		Use:   "create [path]",
		Short: "Create a new PGP key",
		Args:  cobra.MaximumNArgs(1),
		Run:   run,
	}
}

func exportDefaultPGPKey() ([]byte, error) {
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

		log.Printf("Deleted PGP key %s\n", key.Fingerprint)
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
	var count int
	var raw bool
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

		var (
			cursor   *metasrht.Cursor
			user     *metasrht.User
			username string
			err      error
		)
		if len(args) > 0 {
			username = strings.TrimLeft(args[0], ownerPrefixes)
		}

		err = pagerify(func(p pager) error {
			if username != "" {
				if raw {
					user, err = metasrht.ListRawPGPKeysByUser(c.Client, ctx, username, cursor)
				} else {
					user, err = metasrht.ListPGPKeysByUser(c.Client, ctx, username, cursor)
				}
			} else {
				if raw {
					user, err = metasrht.ListRawPGPKeys(c.Client, ctx, cursor)
				} else {
					user, err = metasrht.ListPGPKeys(c.Client, ctx, cursor)
				}
			}

			if err != nil {
				return err
			} else if user == nil {
				return fmt.Errorf("no such user %q", username)
			}

			if raw {
				for _, key := range user.PgpKeys.Results {
					fmt.Fprintln(p, key.Key)
				}
			} else {
				for _, key := range user.PgpKeys.Results {
					fmt.Fprintf(p, "%s %s\n", termfmt.DarkYellow.Sprintf("#%d", key.Id), key.Fingerprint)
					fmt.Fprintln(p)
				}
			}

			cursor = user.PgpKeys.Cursor
			if p.IsDone(cursor, len(user.PgpKeys.Results)) {
				return pagerDone
			}

			return nil
		}, count)
		if err != nil {
			log.Fatal(err)
		}
	}

	cmd := &cobra.Command{
		Use:               "list [user]",
		Short:             "List PGP keys",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().IntVar(&count, "count", 0, "number of keys to fetch")
	cmd.RegisterFlagCompletionFunc("count", cobra.NoFileCompletions)
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

		whEvents, err := metasrht.ParseUserEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents
		config.Query = readWebhookQuery(stdin)

		webhook, err := metasrht.CreateUserWebhook(c.Client, ctx, config)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Created user webhook with ID %d\n", webhook.Id)
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
	cmd.Flags().BoolVar(&stdin, "stdin", !isStdinTerminal, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newMetaUserWebhookListCommand() *cobra.Command {
	var count int
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)
		var cursor *metasrht.Cursor

		err := pagerify(func(p pager) error {
			webhooks, err := metasrht.UserWebhooks(c.Client, ctx, cursor)
			if err != nil {
				return err
			}

			for _, webhook := range webhooks.Results {
				fmt.Fprintf(p, "%s %s\n", termfmt.DarkYellow.Sprintf("#%d", webhook.Id), webhook.Url)
			}

			cursor = webhooks.Cursor
			if p.IsDone(cursor, len(webhooks.Results)) {
				return pagerDone
			}

			return nil
		}, count)
		if err != nil {
			log.Fatal(err)
		}
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List user webhooks",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	cmd.Flags().IntVar(&count, "count", 0, "number of webhooks to fetch")
	cmd.RegisterFlagCompletionFunc("count", cobra.NoFileCompletions)
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
		}

		log.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a user webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeMetaUserWebhookID,
		Run:               run,
	}
	return cmd
}

func newMetaOAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "oauth",
		Short: "List OAuth credentials",
	}
	cmd.AddCommand(newMetaOAuthTokensCommand())
	return cmd
}

func newMetaOAuthTokensCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("meta", cmd)

		tokens, err := metasrht.PersonalAccessTokens(c.Client, ctx)
		if err != nil {
			log.Fatal(err)
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer tw.Flush()

		fmt.Fprint(tw, termfmt.Bold.String("Comment\tIssued\tExpires\tGrant\n"))
		for _, token := range tokens {
			var s string
			if token.Comment != nil {
				s = fmt.Sprintf("%s\t", *token.Comment)
			} else {
				s = "\t"
			}

			issued := humanize.Time(token.Issued.Time)
			expires := humanize.Time(token.Expires.Time)

			s += fmt.Sprintf("%s\t%s\t", issued, expires)

			if token.Grants != nil {
				s += *token.Grants
			}

			fmt.Fprintln(tw, s)
		}
	}

	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "List personal access tokens",
		Args:  cobra.ExactArgs(0),
		Run:   run,
	}
	return cmd
}

func completeSSHKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("meta", cmd)
	var keyList []string

	user, err := metasrht.ListSSHKeys(c.Client, ctx, nil)
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

	user, err := metasrht.ListPGPKeys(c.Client, ctx, nil)
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

func completeMetaUserWebhookID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("meta", cmd)
	var webhookList []string

	webhooks, err := metasrht.UserWebhooks(c.Client, ctx, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, webhook := range webhooks.Results {
		s := fmt.Sprintf("%d\t%s", webhook.Id, webhook.Url)
		webhookList = append(webhookList, s)
	}

	return webhookList, cobra.ShellCompDirectiveNoFileComp
}
