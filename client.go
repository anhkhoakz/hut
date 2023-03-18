package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"git.sr.ht/~emersion/gqlclient"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

type Client struct {
	*gqlclient.Client

	Hostname string
	BaseURL  string
	HTTP     *http.Client
}

func createClient(service string, cmd *cobra.Command) *Client {
	return createClientWithInstance(service, cmd, "")
}

func createClientWithInstance(service string, cmd *cobra.Command, instanceName string) *Client {
	customConfigFile := true
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		log.Fatal(err)
	} else if configFile == "" {
		configFile = defaultConfigFilename()
		customConfigFile = false
	}

	cfg, err := loadConfig(configFile)
	if err != nil {
		// This error message doesn't make sense if a config was
		// provided with "--config". In that case, the normal log
		// message is always desired.
		if !customConfigFile && errors.Is(err, os.ErrNotExist) {
			os.Stderr.WriteString("Looks like hut's config file hasn't been set up yet.\nRun `hut init` to configure it.\n")
			os.Exit(1)
		}
		log.Fatalf("failed to load config file: %v", err)
	}

	if len(cfg.Instances) == 0 {
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

	var inst *InstanceConfig
	if instanceName != "" {
		for _, instance := range cfg.Instances {
			if instance.match(instanceName) {
				inst = instance
				break
			}
		}

		if inst == nil {
			log.Fatalf("no instance for %s found", instanceName)
		}
	} else {
		inst = cfg.Instances[0]
	}

	var token string
	if len(inst.AccessTokenCmd) > 0 {
		cmd := exec.Command(inst.AccessTokenCmd[0], inst.AccessTokenCmd[1:]...)
		output, err := cmd.Output()
		if err != nil {
			log.Fatalf("could not execute access-token-cmd: %v", err)
		}

		token = strings.Fields(string(output))[0]
	} else {
		token = inst.AccessToken
	}

	baseURL := inst.Origins[service]
	if baseURL == "" && strings.Contains(inst.Name, ".") && net.ParseIP(inst.Name) == nil {
		baseURL = fmt.Sprintf("https://%s.%s", service, inst.Name)
	}
	if baseURL == "" {
		log.Fatalf("failed to get origin for service %q in instance %q", service, inst.Name)
	}
	return createClientWithToken(baseURL, token)
}

func createClientWithToken(baseURL, token string) *Client {
	gqlEndpoint := baseURL + "/query"
	httpTransport := &oauth2.Transport{
		Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}),
		Base:   userAgentHTTPTransport("hut"),
	}
	httpClient := &http.Client{
		Transport: httpTransport,
		Timeout:   30 * time.Second,
	}
	return &Client{
		Client:  gqlclient.New(gqlEndpoint, httpClient),
		BaseURL: baseURL,
		HTTP:    httpClient,
	}
}

type userAgentHTTPTransport string

func (ua userAgentHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", string(ua))
	return http.DefaultTransport.RoundTrip(req)
}
