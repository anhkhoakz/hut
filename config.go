package main

import (
	"bufio"
	"context"
	"errors"
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
	Instances []*InstanceConfig `scfg:"instance"`
}

type InstanceConfig struct {
	Name string `scfg:",param"`

	AccessToken    string   `scfg:"access-token"`
	AccessTokenCmd []string `scfg:"access-token-cmd"`

	Builds *ServiceConfig `scfg:"builds"`
	Git    *ServiceConfig `scfg:"git"`
	Hg     *ServiceConfig `scfg:"hg"`
	Lists  *ServiceConfig `scfg:"lists"`
	Meta   *ServiceConfig `scfg:"meta"`
	Pages  *ServiceConfig `scfg:"pages"`
	Paste  *ServiceConfig `scfg:"paste"`
	Todo   *ServiceConfig `scfg:"todo"`
}

func (instance *InstanceConfig) match(name string) bool {
	if instancesEqual(name, instance.Name) {
		return true
	}

	for _, service := range instance.Services() {
		if service.Origin != "" && stripProtocol(service.Origin) == name {
			return true
		}
	}
	return false
}

func (instance *InstanceConfig) Services() map[string]*ServiceConfig {
	all := map[string]*ServiceConfig{
		"builds": instance.Builds,
		"git":    instance.Git,
		"hg":     instance.Hg,
		"lists":  instance.Lists,
		"meta":   instance.Meta,
		"pages":  instance.Pages,
		"paste":  instance.Paste,
		"todo":   instance.Todo,
	}

	m := make(map[string]*ServiceConfig)
	for name, service := range all {
		if service != nil {
			m[name] = service
		}
	}
	return m
}

type ServiceConfig struct {
	Origin string `scfg:"origin"`
}

func instancesEqual(a, b string) bool {
	return a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a)
}

func loadConfigFile(filename string) (*Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := new(Config)
	if err := scfg.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}

	instanceNames := make(map[string]struct{})
	for _, instance := range cfg.Instances {
		if _, ok := instanceNames[instance.Name]; ok {
			return nil, fmt.Errorf("duplicate instance name %q", instance.Name)
		}
		instanceNames[instance.Name] = struct{}{}

		if instance.AccessTokenCmd != nil && len(instance.AccessTokenCmd) == 0 {
			return nil, fmt.Errorf("instance %q: missing command name in access-token-cmd directive", instance.Name)
		}
		if instance.AccessToken == "" && len(instance.AccessTokenCmd) == 0 {
			return nil, fmt.Errorf("instance %q: missing access-token or access-token-cmd", instance.Name)
		}
		if instance.AccessToken != "" && len(instance.AccessTokenCmd) > 0 {
			return nil, fmt.Errorf("instance %q: access-token and access-token-cmd can't be both specified", instance.Name)
		}
	}

	return cfg, nil
}

func loadConfig(cmd *cobra.Command) *Config {
	type configContextKey struct{}
	if v := cmd.Context().Value(configContextKey{}); v != nil {
		return v.(*Config)
	}

	customConfigFile := true
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		log.Fatal(err)
	} else if configFile == "" {
		configFile = defaultConfigFilename()
		customConfigFile = false
	}

	cfg, err := loadConfigFile(configFile)
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

	ctx := cmd.Context()
	ctx = context.WithValue(ctx, configContextKey{}, cfg)
	cmd.SetContext(ctx)

	return cfg
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

		// Perform an early sanity check to avoid asking the user to login if
		// the config file already exists
		if _, err := os.Stat(filename); err == nil {
			log.Fatalf("config file %q already exists (delete it if you want to overwrite it)", filename)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Fatal(err)
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

		log.Printf("hut initialized for user %v\n", termfmt.Bold.String(user.CanonicalName))
	}
	return cmd
}
