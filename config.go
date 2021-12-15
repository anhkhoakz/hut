package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"git.sr.ht/~emersion/go-scfg"
	"git.sr.ht/~emersion/gqlclient"
	"golang.org/x/oauth2"
)

func createClient(service string) *gqlclient.Client {
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("failed to get user config dir: %v", err)
	}

	cfg, err := scfg.Load(filepath.Join(configDir, "hut", "config"))
	if err != nil {
		log.Fatalf("failed to load config file: %v", err)
	}

	instances := cfg.GetAll("instance")
	if len(instances) == 0 {
		log.Fatalf("no sr.ht instance configured")
	}
	inst := instances[0]

	if len(inst.Params) == 0 {
		log.Fatalf("missing instance hostname")
	}
	accessToken := inst.Children.Get("access-token")
	if accessToken == nil || len(accessToken.Params) == 0 {
		log.Fatalf("missing instance access-token")
	}

	hostname := inst.Params[0]
	endpoint := fmt.Sprintf("https://%s.%s/query", service, hostname)

	tokenSrc := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken.Params[0]})
	httpClient := oauth2.NewClient(context.Background(), tokenSrc)
	return gqlclient.New(endpoint, httpClient)
}
