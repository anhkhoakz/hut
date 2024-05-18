package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"git.sr.ht/~xenrox/hut/export"
)

type ExportInfo struct {
	Instance string    `json:"instance"`
	Service  string    `json:"service"`
	Date     time.Time `json:"date"`
}

type exporter struct {
	export.Exporter
	Name    string
	BaseURL string
}

func newExportCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		var exporters []exporter

		mc := createClient("meta", cmd)
		meta := export.NewMetaExporter(mc.Client)
		exporters = append(exporters, exporter{meta, "meta.sr.ht", mc.BaseURL})

		gc := createClient("git", cmd)
		git := export.NewGitExporter(gc.Client, gc.BaseURL)
		exporters = append(exporters, exporter{git, "git.sr.ht", gc.BaseURL})

		hc := createClient("hg", cmd)
		hg := export.NewHgExporter(hc.Client, hc.BaseURL)
		exporters = append(exporters, exporter{hg, "hg.sr.ht", hc.BaseURL})

		bc := createClient("builds", cmd)
		builds := export.NewBuildsExporter(bc.Client, bc.HTTP)
		exporters = append(exporters, exporter{builds, "builds.sr.ht", bc.BaseURL})

		pc := createClient("paste", cmd)
		paste := export.NewPasteExporter(pc.Client, pc.HTTP)
		exporters = append(exporters, exporter{paste, "paste.sr.ht", pc.BaseURL})

		lc := createClient("lists", cmd)
		lists := export.NewListsExporter(lc.Client, lc.HTTP)
		exporters = append(exporters, exporter{lists, "lists.sr.ht", lc.BaseURL})

		tc := createClient("todo", cmd)
		todo := export.NewTodoExporter(tc.Client, tc.HTTP)
		exporters = append(exporters, exporter{todo, "todo.sr.ht", tc.BaseURL})

		if _, ok := os.LookupEnv("SSH_AUTH_SOCK"); !ok {
			log.Println("Warning! SSH_AUTH_SOCK is not set in your environment.")
			log.Println("Using an SSH agent is advised to avoid unlocking your SSH keys repeatedly during the export.")
		}

		ctx := cmd.Context()
		log.Println("Exporting account data...")

		out := args[0]
		resources := args[1:]

		// Export all services by default
		if len(resources) == 0 {
			for _, ex := range exporters {
				resources = append(resources, ex.BaseURL)
			}
		}

		for _, resource := range resources {
			log.Println(resource)

			var name, owner, instance string
			if res := stripProtocol(resource); !strings.Contains(res, "/") {
				instance = res
			} else {
				name, owner, instance = parseResourceName(resource)
				owner = strings.TrimLeft(owner, ownerPrefixes)
			}

			var ex *exporter
			for _, e := range exporters {
				if stripProtocol(e.BaseURL) == instance {
					ex = &e
					break
				}
			}
			if ex == nil {
				log.Fatalf("Unknown resource instance: %s", resource)
			}

			var err error
			if name == "" && owner == "" {
				err = exportService(ctx, out, ex)
			} else if name != "" && owner != "" {
				err = exportResource(ctx, out, ex, owner, name)
			} else {
				err = fmt.Errorf("unknown resource")
			}
			if err != nil {
				log.Printf("Failed to export %q: %v", resource, err)
			}
		}

		log.Println("Export complete.")
	}
	return &cobra.Command{
		Use:   "export <directory> [resource|service...]",
		Short: "Exports your account data",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) <= 1 {
				return nil, cobra.ShellCompDirectiveFilterDirs
			}
			// TODO: completion on export resources
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		Run: run,
	}
}

func exportService(ctx context.Context, out string, ex *exporter) error {
	base := path.Join(out, ex.Name)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return fmt.Errorf("failed to create export directory: %v", err)
	}

	stamp := path.Join(base, "service.json")
	if _, err := os.Stat(stamp); err == nil {
		log.Printf("Skipping %s (already exported)", ex.Name)
		return nil
	}

	if err := ex.Export(ctx, base); err != nil {
		return err
	}

	info := ExportInfo{
		Instance: ex.BaseURL,
		Service:  ex.Name,
		Date:     time.Now().UTC(),
	}
	if err := writeExportStamp(stamp, &info); err != nil {
		return fmt.Errorf("failed writing stamp: %v", err)
	}

	return nil
}

func writeExportStamp(path string, info *ExportInfo) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(info)
}

func exportResource(ctx context.Context, out string, ex *exporter, owner, name string) error {
	base := path.Join(out, ex.Name, name)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return fmt.Errorf("failed to create export directory: %v", err)
	}

	return ex.ExportResource(ctx, base, owner, name)
}
