package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.sr.ht/~emersion/go-scfg"
	"git.sr.ht/~emersion/gqlclient"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

type Client struct {
	*gqlclient.Client

	Hostname string
	BaseURL  string
}

func createClient(service string, cmd *cobra.Command) *Client {
	return createClientWithInstance(service, cmd, "")
}

func createClientWithInstance(service string, cmd *cobra.Command, instanceName string) *Client {
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		log.Fatal(err)
	}

	customConfigFile := true
	if configFile == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			log.Fatalf("failed to get user config dir: %v", err)
		}

		configFile = filepath.Join(configDir, "hut", "config")
		customConfigFile = false
	}

	cfg, err := scfg.Load(configFile)
	if err != nil {
		// This error message doesn't make sense if a config was
		// provided with "--config". In that case, the normal log
		// message is always desired.
		if !customConfigFile && errors.Is(err, os.ErrNotExist) {
			os.Stderr.WriteString("Looks like you haven't created a config file yet.\nSee `man hut` for an example that you can copy.\n")
			os.Exit(1)
		}
		log.Fatalf("failed to load config file: %v", err)
	}

	instances := cfg.GetAll("instance")
	if len(instances) == 0 {
		log.Fatalf("no sr.ht instance configured")
	}

	if instanceFlag, err := cmd.Flags().GetString("instance"); err != nil {
		log.Fatal(err)
	} else if instanceFlag != "" {
		if instanceName != "" && !instancesEqual(instanceName, instanceFlag) {
			log.Fatalf("conflicting instances: %v and --instance=%v", instanceName, instanceFlag)
		}
		instanceName = instanceFlag
	}

	var inst *scfg.Directive
	if instanceName != "" {
		for _, instance := range instances {
			if instancesEqual(instanceName, instance.Params[0]) {
				inst = instance
				break
			}
		}

		if inst == nil {
			log.Fatalf("no instance for %s found", instanceName)
		}
	} else {
		inst = instances[0]
	}

	if len(inst.Params) == 0 {
		log.Fatalf("missing instance hostname")
	}

	var token string
	accessToken := inst.Children.Get("access-token")
	if accessToken == nil || len(accessToken.Params) == 0 {
		tokenCmd := inst.Children.Get("access-token-cmd")
		if tokenCmd == nil || len(tokenCmd.Params) == 0 {
			log.Fatalf("missing instance access-token or access-token-cmd")
		}

		cmd := exec.Command(tokenCmd.Params[0], tokenCmd.Params[1:]...)
		output, err := cmd.Output()
		if err != nil {
			log.Fatalf("could not execute access-token-cmd: %v", err)
		}

		token = strings.Fields(string(output))[0]
	} else {
		token = accessToken.Params[0]
	}

	hostname := inst.Params[0]
	baseURL := fmt.Sprintf("https://%s.%s", service, hostname)
	gqlEndpoint := baseURL + "/query"

	tokenSrc := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(context.Background(), tokenSrc)
	return &Client{
		Client:   gqlclient.New(gqlEndpoint, httpClient),
		Hostname: hostname,
		BaseURL:  baseURL,
	}
}

func instancesEqual(a, b string) bool {
	return a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a)
}
