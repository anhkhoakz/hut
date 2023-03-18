package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~emersion/go-scfg"
	"github.com/spf13/cobra"

	"git.sr.ht/~emersion/hut/srht/metasrht"
	"git.sr.ht/~emersion/hut/termfmt"
)

type Config struct {
	Instances []*InstanceConfig
}

type InstanceConfig struct {
	Name string

	AccessToken    string
	AccessTokenCmd []string

	Origins map[string]string
}

func (instance InstanceConfig) match(name string) bool {
	if instancesEqual(name, instance.Name) {
		return true
	}

	for _, origin := range instance.Origins {
		if stripProtocol(origin) == name {
			return true
		}
	}
	return false
}

func instancesEqual(a, b string) bool {
	return a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a)
}

func loadConfig(filename string) (*Config, error) {
	rootBlock, err := scfg.Load(filename)
	if err != nil {
		return nil, err
	}

	cfg := new(Config)
	instanceNames := make(map[string]struct{})
	for _, instanceDir := range rootBlock.GetAll("instance") {
		instance := &InstanceConfig{
			Origins: make(map[string]string),
		}

		if err := instanceDir.ParseParams(&instance.Name); err != nil {
			return nil, err
		}

		if _, ok := instanceNames[instance.Name]; ok {
			return nil, fmt.Errorf("duplicate instance name %q", instance.Name)
		}
		instanceNames[instance.Name] = struct{}{}

		if dir := instanceDir.Children.Get("access-token"); dir != nil {
			if err := dir.ParseParams(&instance.AccessToken); err != nil {
				return nil, err
			}
		}
		if dir := instanceDir.Children.Get("access-token-cmd"); dir != nil {
			if len(dir.Params) == 0 {
				return nil, fmt.Errorf("instance %q: missing command name in access-token-cmd directive", instance.Name)
			}
			instance.AccessTokenCmd = dir.Params
		}
		if instance.AccessToken == "" && len(instance.AccessTokenCmd) == 0 {
			return nil, fmt.Errorf("instance %q: missing access-token or access-token-cmd", instance.Name)
		}
		if instance.AccessToken != "" && len(instance.AccessTokenCmd) > 0 {
			return nil, fmt.Errorf("instance %q: access-token and access-token-cmd can't be both specified", instance.Name)
		}

		for _, service := range []string{"builds", "git", "hg", "lists", "meta", "pages", "paste", "todo"} {
			serviceDir := instanceDir.Children.Get(service)
			if serviceDir == nil {
				continue
			}

			originDir := serviceDir.Children.Get("origin")
			if originDir == nil {
				continue
			}

			var origin string
			if err := originDir.ParseParams(&origin); err != nil {
				return nil, err
			}

			instance.Origins[service] = origin
		}

		cfg.Instances = append(cfg.Instances, instance)
	}

	return cfg, nil
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
		token := strings.TrimSpace(scanner.Text())
		if err := scanner.Err(); err != nil {
			log.Fatalf("failed to read token from stdin: %v", err)
		} else if token == "" {
			log.Fatal("no token provided")
		}

		config := fmt.Sprintf("instance %q {\n	access-token %q\n}\n", instance, token)

		c := createClientWithToken(baseURL, token)
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
