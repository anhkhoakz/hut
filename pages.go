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

		pagesProtocol, err := getProtocol(protocol)
		if err != nil {
			log.Fatal(err)
		}

		c := createClient("pages")

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
	cmd.Flags().StringVarP(&protocol, "protocol", "p", "HTTPS",
		"protocol (HTTPS or GEMINI)")
	return cmd
}

func newPagesUnpublishCommand() *cobra.Command {
	var domain, protocol string
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		if domain == "" {
			log.Fatal("enter a domain with --domain")
		}

		pagesProtocol, err := getProtocol(protocol)
		if err != nil {
			log.Fatal(err)
		}

		c := createClient("pages")

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
	cmd.Flags().StringVarP(&protocol, "protocol", "p", "HTTPS",
		"protocol (HTTPS or GEMINI)")
	return cmd
}

func newPagesListCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		c := createClient("pages")

		sites, err := pagessrht.Sites(c.Client, ctx)
		if err != nil {
			log.Fatalf("failed to list sites: %v", err)
		}

		for _, site := range sites.Results {
			fmt.Printf("%s (%s)\n", termfmt.String(site.Domain, termfmt.Bold), site.Protocol)
		}
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered sites",
		Run:   run,
	}
	return cmd
}

func getProtocol(protocol string) (pagessrht.Protocol, error) {
	switch strings.ToLower(protocol) {
	case "https":
		return pagessrht.ProtocolHttps, nil
	case "gemini":
		return pagessrht.ProtocolGemini, nil
	default:
		return "", fmt.Errorf("invalid protocol: %s", protocol)
	}
}
