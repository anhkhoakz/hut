package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.sr.ht/~emersion/go-scfg"
	"git.sr.ht/~emersion/gqlclient"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"git.sr.ht/~emersion/hut/srht/metasrht"
	"git.sr.ht/~emersion/hut/termfmt"
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

	cfg, err := scfg.Load(configFile)
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
	return createClientWithToken(hostname, baseURL, token)
}

func createClientWithToken(hostname, baseURL, token string) *Client {
	gqlEndpoint := baseURL + "/query"
	tokenSrc := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(context.Background(), tokenSrc)
	return &Client{
		Client:   gqlclient.New(gqlEndpoint, httpClient),
		Hostname: hostname,
		BaseURL:  baseURL,
		HTTP:     httpClient,
	}
}

func instancesEqual(a, b string) bool {
	return a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a)
}

func defaultConfigFilename() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("failed to get user config dir: %v", err)
	}
	return filepath.Join(configDir, "hut", "config")
}

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize hut",
		Args:  cobra.ExactArgs(0),
	}
	cmd.Run = func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		filename, err := cmd.Flags().GetString("config")
		if err != nil {
			log.Fatal(err)
		} else if filename == "" {
			filename = defaultConfigFilename()
		}

		instance, err := cmd.Flags().GetString("instance")
		if err != nil {
			log.Fatal(err)
		} else if instance == "" {
			instance = "sr.ht"
		}

		baseURL := "https://meta." + instance
		fmt.Printf("Generate a new OAuth2 access token at:\n")
		fmt.Printf("%s/oauth2/personal-token\n", baseURL)
		fmt.Printf("Then copy-paste it here: ")

		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		token := scanner.Text()
		if err := scanner.Err(); err != nil {
			log.Fatalf("failed to read token from stdin: %v", err)
		} else if token == "" {
			log.Fatal("no token provided")
		}

		config := fmt.Sprintf("instance %q {\n	access-token %q\n}\n", instance, token)

		c := createClientWithToken(instance, baseURL, token)
		user, err := metasrht.FetchMe(c.Client, ctx)
		if err != nil {
			log.Fatalf("failed to check OAuth2 token: %v", err)
		}

		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			log.Fatalf("failed to create config file parent directory: %v", err)
		}

		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			log.Fatalf("config file %q already exists (delete it if you want to overwrite it)", filename)
		} else if err != nil {
			log.Fatalf("failed to create config file: %v", err)
		}
		defer f.Close()

		if _, err := f.WriteString(config); err != nil {
			log.Fatalf("failed to write config file: %v", err)
		}
		if err := f.Close(); err != nil {
			log.Fatalf("failed to close config file: %v", err)
		}

		fmt.Printf("hut initialized for user %v\n", termfmt.Bold.String(user.CanonicalName))
	}
	return cmd
}
