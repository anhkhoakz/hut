package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~emersion/gqlclient"
	"github.com/spf13/cobra"

	"git.sr.ht/~xenrox/hut/srht/pagessrht"
	"git.sr.ht/~xenrox/hut/termfmt"
)

func newPagesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pages",
		Short: "Use the pages API",
	}
	cmd.AddCommand(newPagesPublishCommand())
	cmd.AddCommand(newPagesUnpublishCommand())
	cmd.AddCommand(newPagesListCommand())
	cmd.AddCommand(newPagesUserWebhookCommand())
	cmd.AddCommand(newPagesACLCommand())
	return cmd
}

func newPagesPublishCommand() *cobra.Command {
	var domain, protocol, subdirectory, siteConfigFile string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		var filename string
		if len(args) > 0 {
			filename = args[0]
		}

		pagesProtocol, err := pagessrht.ParseProtocol(protocol)
		if err != nil {
			log.Fatal(err)
		}

		siteConfig := pagessrht.SiteConfig{}
		if siteConfigFile != "" {
			config, err := readSiteConfig(siteConfigFile)
			if err != nil {
				log.Fatalf("failed to read site-config: %v", err)
			}
			siteConfig = *config
		}

		c := createClient("pages", cmd)
		c.HTTP.Timeout = fileTransferTimeout

		var f *os.File
		if filename == "" {
			f = os.Stdin
		} else {
			f, err = os.Open(filename)
			if err != nil {
				log.Fatalf("failed to open input file: %v", err)
			}
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			log.Fatalf("failed to stat input file: %v", err)
		}

		var upload gqlclient.Upload
		if fi.IsDir() {
			pr, pw := io.Pipe()
			defer pr.Close()

			go func() {
				pw.CloseWithError(writeSiteArchive(pw, filename))
			}()

			upload = gqlclient.Upload{Body: pr}
		} else {
			upload = gqlclient.Upload{Body: f, Filename: filepath.Base(filename)}
		}

		upload.MIMEType = "application/gzip"

		site, err := pagessrht.Publish(c.Client, ctx, domain, upload, pagesProtocol, subdirectory, siteConfig)
		if err != nil {
			log.Fatalf("failed to publish site: %v", err)
		}

		log.Printf("Published site at %s\n", site.Domain)
	}

	cmd := &cobra.Command{
		Use:   "publish [file]",
		Short: "Publish a website",
		Args:  cobra.MaximumNArgs(1),
		Run:   run,
	}
	cmd.Flags().StringVarP(&domain, "domain", "d", "", "domain name")
	cmd.MarkFlagRequired("domain")
	cmd.RegisterFlagCompletionFunc("domain", completeDomain)
	cmd.Flags().StringVarP(&protocol, "protocol", "p", "HTTPS",
		"protocol (HTTPS or GEMINI)")
	cmd.RegisterFlagCompletionFunc("protocol", completeProtocol)
	cmd.Flags().StringVarP(&subdirectory, "subdirectory", "s", "/", "subdirectory")
	cmd.Flags().StringVar(&siteConfigFile, "site-config", "", "path to site configuration file (for e.g. cache-control)")
	cmd.RegisterFlagCompletionFunc("site-config", cobra.FixedCompletions([]string{"json"}, cobra.ShellCompDirectiveFilterFileExt))
	return cmd
}

func writeSiteArchive(w io.Writer, dir string) error {
	gzipWriter := gzip.NewWriter(w)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	err := filepath.WalkDir(dir, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if de.IsDir() {
			return nil
		}
		if t := de.Type(); t != 0 {
			// Symlink, pipe, socket, device, etc
			return fmt.Errorf("unsupported file %q type (%v)", path, t)
		}

		fi, err := de.Info()
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		header := tar.Header{
			Typeflag: tar.TypeReg,
			Name:     filepath.ToSlash(rel),
			ModTime:  fi.ModTime(),
			Mode:     0600,
			Size:     fi.Size(),
		}
		if err := tarWriter.WriteHeader(&header); err != nil {
			return err
		}
		_, err = io.Copy(tarWriter, f)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to walk directory: %v", err)
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %v", err)
	}
	return nil
}

func readSiteConfig(filename string) (*pagessrht.SiteConfig, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config pagessrht.SiteConfig
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func newPagesUnpublishCommand() *cobra.Command {
	var domain, protocol string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		pagesProtocol, err := pagessrht.ParseProtocol(protocol)
		if err != nil {
			log.Fatal(err)
		}

		c := createClient("pages", cmd)

		site, err := pagessrht.Unpublish(c.Client, ctx, domain, pagesProtocol)
		if err != nil {
			log.Fatalf("failed to unpublish site: %v", err)
		}

		if site == nil {
			log.Printf("This site does not exist\n")
		} else {
			log.Printf("Unpublished site at %s\n", site.Domain)
		}
	}

	cmd := &cobra.Command{
		Use:   "unpublish",
		Short: "Unpublish a website",
		Run:   run,
	}
	cmd.Flags().StringVarP(&domain, "domain", "d", "", "domain name")
	cmd.MarkFlagRequired("domain")
	cmd.RegisterFlagCompletionFunc("domain", completeDomain)
	cmd.Flags().StringVarP(&protocol, "protocol", "p", "HTTPS",
		"protocol (HTTPS or GEMINI)")
	cmd.RegisterFlagCompletionFunc("protocol", completeProtocol)
	return cmd
}

func newPagesListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("pages", cmd)
		var cursor *pagessrht.Cursor

		err := pagerify(func(p pager) error {
			sites, err := pagessrht.Sites(c.Client, ctx, cursor)
			if err != nil {
				return fmt.Errorf("failed to list sites: %v", err)
			}

			for _, site := range sites.Results {
				fmt.Fprintf(p, "%s %s (%s)\n", termfmt.DarkYellow.Sprintf("#%d", site.Id), termfmt.Bold.Sprintf(site.Domain), site.Protocol)
			}

			cursor = sites.Cursor
			if cursor == nil {
				return pagerDone
			}

			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered sites",
		Run:   run,
	}
	return cmd
}

func newPagesUserWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user-webhook",
		Short: "Manage user webhooks",
	}
	cmd.AddCommand(newPagesUserWebhookCreateCommand())
	cmd.AddCommand(newPagesUserWebhookListCommand())
	cmd.AddCommand(newPagesUserWebhookDeleteCommand())
	return cmd
}

func newPagesUserWebhookCreateCommand() *cobra.Command {
	var events []string
	var stdin bool
	var url string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("pages", cmd)

		var config pagessrht.UserWebhookInput
		config.Url = url

		whEvents, err := pagessrht.ParseEvents(events)
		if err != nil {
			log.Fatal(err)
		}
		config.Events = whEvents
		config.Query = readWebhookQuery(stdin)

		webhook, err := pagessrht.CreateUserWebhook(c.Client, ctx, config)
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
	cmd.RegisterFlagCompletionFunc("events", completePagesUserWebhookEvents)
	cmd.MarkFlagRequired("events")
	cmd.Flags().BoolVar(&stdin, "stdin", !isStdinTerminal, "read webhook query from stdin")
	cmd.Flags().StringVarP(&url, "url", "u", "", "payload url")
	cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	cmd.MarkFlagRequired("url")
	return cmd
}

func newPagesUserWebhookListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("pages", cmd)
		var cursor *pagessrht.Cursor

		err := pagerify(func(p pager) error {
			webhooks, err := pagessrht.UserWebhooks(c.Client, ctx, cursor)
			if err != nil {
				return err
			}

			for _, webhook := range webhooks.Results {
				fmt.Fprintf(p, "%s %s\n", termfmt.DarkYellow.Sprintf("#%d", webhook.Id), webhook.Url)
			}

			cursor = webhooks.Cursor
			if cursor == nil {
				return pagerDone
			}

			return nil
		})
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
	return cmd
}

func newPagesUserWebhookDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("pages", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		webhook, err := pagessrht.DeleteUserWebhook(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted webhook %d\n", webhook.Id)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete a user webhook",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completePagesUserWebhookID,
		Run:               run,
	}
	return cmd
}

func newPagesACLCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acl",
		Short: "Manage access-control lists",
	}
	cmd.AddCommand(newPagesACLUpdateCommand())
	cmd.AddCommand(newPagesACLDeleteCommand())
	return cmd
}

func newPagesACLUpdateCommand() *cobra.Command {
	var publish bool
	var siteID int
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("pages", cmd)

		user, err := pagessrht.UserID(c.Client, ctx, args[0])
		if err != nil {
			log.Fatalf("failed to get user ID: %v", err)
		} else if user == nil {
			log.Fatal("no such user")
		}

		var input pagessrht.ACLInput
		input.Publish = publish

		acl, err := pagessrht.UpdateSiteACL(c.Client, ctx, int32(siteID), user.Id, input)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Updated access rights for %q\n", acl.Entity.CanonicalName)
	}

	cmd := &cobra.Command{
		Use:               "update <user>",
		Short:             "Update ACL entries",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	cmd.Flags().BoolVar(&publish, "publish", false, "permission to publish the site")
	cmd.Flags().IntVar(&siteID, "id", 0, "ID of the site")
	cmd.MarkFlagRequired("id")
	cmd.RegisterFlagCompletionFunc("id", cobra.NoFileCompletions)
	return cmd
}

func newPagesACLDeleteCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := createClient("pages", cmd)

		id, err := parseInt32(args[0])
		if err != nil {
			log.Fatal(err)
		}

		acl, err := pagessrht.DeleteSiteACL(c.Client, ctx, id)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Deleted ACL entry for %q\n", acl.Entity.CanonicalName)
	}

	cmd := &cobra.Command{
		Use:               "delete <ID>",
		Short:             "Delete an ACL entry",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run:               run,
	}
	return cmd
}

var completeProtocol = cobra.FixedCompletions([]string{"https", "gemini"}, cobra.ShellCompDirectiveNoFileComp)

func completeDomain(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("pages", cmd)
	var domainList []string

	protocol, err := cmd.Flags().GetString("protocol")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	sites, err := pagessrht.Sites(c.Client, ctx, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, site := range sites.Results {
		if strings.EqualFold(protocol, string(site.Protocol)) {
			domainList = append(domainList, site.Domain)
		}
	}

	return domainList, cobra.ShellCompDirectiveNoFileComp
}

func completePagesUserWebhookEvents(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var eventList []string
	events := [2]string{"site_published", "site_unpublished"}
	set := strings.ToLower(cmd.Flag("events").Value.String())
	for _, event := range events {
		if !strings.Contains(set, event) {
			eventList = append(eventList, event)
		}
	}
	return eventList, cobra.ShellCompDirectiveNoFileComp
}

func completePagesUserWebhookID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("pages", cmd)
	var webhookList []string

	webhooks, err := pagessrht.UserWebhooks(c.Client, ctx, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, webhook := range webhooks.Results {
		s := fmt.Sprintf("%d\t%s", webhook.Id, webhook.Url)
		webhookList = append(webhookList, s)
	}

	return webhookList, cobra.ShellCompDirectiveNoFileComp
}
