package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"git.sr.ht/~emersion/gqlclient"
	"github.com/spf13/cobra"
)

type Client struct {
	*gqlclient.Client

	BaseURL string
	HTTP    *http.Client
}

func createClient(service string, cmd *cobra.Command) *Client {
	return createClientWithInstance(service, cmd, "")
}

func createClientWithInstance(service string, cmd *cobra.Command, instanceName string) *Client {
	cfg := loadConfig(cmd)
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

		fields := strings.Fields(string(output))
		if len(fields) == 0 {
			log.Fatalf("access-token-cmd did not return a token")
		}

		token = fields[0]
	} else {
		token = inst.AccessToken
	}

	var baseURL string
	if serviceCfg := inst.Services()[service]; serviceCfg != nil {
		baseURL = serviceCfg.Origin
	}
	if baseURL == "" && strings.Contains(inst.Name, ".") && net.ParseIP(inst.Name) == nil {
		baseURL = fmt.Sprintf("https://%s.%s", service, inst.Name)
	}
	if baseURL == "" {
		log.Fatalf("failed to get origin for service %q in instance %q", service, inst.Name)
	}

	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		log.Fatal(err)
	}

	return createClientWithToken(baseURL, token, debug)
}

func createClientWithToken(baseURL, token string, debug bool) *Client {
	gqlEndpoint := baseURL + "/query"
	httpClient := &http.Client{
		Transport: &httpTransport{accessToken: token, logRequest: debug},
		Timeout:   30 * time.Second,
	}
	return &Client{
		Client:  gqlclient.New(gqlEndpoint, httpClient),
		BaseURL: baseURL,
		HTTP:    httpClient,
	}
}

type httpTransport struct {
	accessToken string
	logRequest  bool
	count       int
}

func (tr *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "hut/"+version)
	req.Header.Set("Authorization", "Bearer "+tr.accessToken)

	// Add delay to consecutive API requests to keep hut from DoSing the server
	if tr.count > 0 {
		time.Sleep(time.Second)
	}
	tr.count++

	if tr.logRequest {
		log.Println(req.Body)
	}
	return http.DefaultTransport.RoundTrip(req)
}
