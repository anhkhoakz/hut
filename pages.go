package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~emersion/gqlclient"
	"github.com/spf13/cobra"

	"git.sr.ht/~emersion/hut/srht/pagessrht"
	"git.sr.ht/~emersion/hut/termfmt"
)

func newPagesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pages",
		Short: "Use the pages API",
	}
	cmd.AddCommand(newPagesPublishCommand())
	cmd.AddCommand(newPagesUnpublishCommand())
	cmd.AddCommand(newPagesListCommand())
	return cmd
}

func newPagesPublishCommand() *cobra.Command {
	var domain, protocol string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		filename := args[0]

		if domain == "" {
			log.Fatal("enter a domain with --domain")
		}

		pagesProtocol, err := pagessrht.ParseProtocol(protocol)
		if err != nil {
			log.Fatal(err)
		}

		c := createClient("pages", cmd)

		f, err := os.Open(filename)
		if err != nil {
			log.Fatalf("failed to open input file: %v", err)
		}
		defer f.Close()

		file := gqlclient.Upload{Body: f, Filename: filepath.Base(filename)}

		site, err := pagessrht.Publish(c.Client, ctx, domain, file, pagesProtocol)
		if err != nil {
			log.Fatalf("failed to publish site: %v", err)
		}

		fmt.Printf("Published site at %s\n", site.Domain)
	}

	cmd := &cobra.Command{
		Use:   "publish <archive>",
		Short: "Publish a website",
		Args:  cobra.ExactArgs(1),
		Run:   run,
	}
	cmd.Flags().StringVarP(&domain, "domain", "d", "", "domain name")
	cmd.RegisterFlagCompletionFunc("domain", completeDomain)
	cmd.Flags().StringVarP(&protocol, "protocol", "p", "HTTPS",
		"protocol (HTTPS or GEMINI)")
	cmd.RegisterFlagCompletionFunc("protocol", completeProtocol)
	return cmd
}

func newPagesUnpublishCommand() *cobra.Command {
	var domain, protocol string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		if domain == "" {
			log.Fatal("enter a domain with --domain")
		}

		pagesProtocol, err := pagessrht.ParseProtocol(protocol)
		if err != nil {
			log.Fatal(err)
		}

		c := createClient("pages", cmd)

		site, err := pagessrht.Unpublish(c.Client, ctx, domain, pagesProtocol)
		if err != nil {
			log.Fatalf("failed to unpublish site: %v", err)
		}

		fmt.Printf("Unpublished site at %s\n", site.Domain)
	}

	cmd := &cobra.Command{
		Use:   "unpublish",
		Short: "Unpublish a website",
		Run:   run,
	}
	cmd.Flags().StringVarP(&domain, "domain", "d", "", "domain name")
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

		sites, err := pagessrht.Sites(c.Client, ctx)
		if err != nil {
			log.Fatalf("failed to list sites: %v", err)
		}

		for _, site := range sites.Results {
			fmt.Printf("%s (%s)\n", termfmt.Bold.Sprintf(site.Domain), site.Protocol)
		}
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered sites",
		Run:   run,
	}
	return cmd
}

func completeProtocol(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"https", "gemini"}, cobra.ShellCompDirectiveNoFileComp
}

func completeDomain(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()
	c := createClient("pages", cmd)
	var domainList []string

	protocol, err := cmd.Flags().GetString("protocol")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	sites, err := pagessrht.Sites(c.Client, ctx)
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
