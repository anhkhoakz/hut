package main

import (
	"encoding/json"
	"log"
	"os"
	"path"
	"time"

	"github.com/spf13/cobra"

	"git.sr.ht/~emersion/hut/export"
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

		// TODO: Allow exporting a subset of all services (maybe meta should
		// provide a list of services configured for that instance?)
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

		if _, ok := os.LookupEnv("SSH_AUTH_SOCK"); !ok {
			log.Println("Warning! SSH_AUTH_SOCK is not set in your environment.")
			log.Println("Using an SSH agent is advised to avoid unlocking your SSH keys repeatedly during the export.")
		}

		ctx := cmd.Context()
		log.Println("Exporting account data...")

		for _, ex := range exporters {
			log.Println(ex.Name)

			base := path.Join(args[0], ex.Name)
			if err := os.MkdirAll(base, 0o755); err != nil {
				log.Fatalf("Failed to create export directory: %s", err.Error())
			}

			stamp := path.Join(base, "export-stamp.json")
			if _, err := os.Stat(stamp); err == nil {
				log.Printf("Skipping %s (already exported)", ex.Name)
				continue
			}

			if err := ex.Export(ctx, base); err != nil {
				log.Printf("Error exporting %s: %s", ex.Name, err.Error())
				continue
			}

			info := ExportInfo{
				Instance: ex.BaseURL,
				Service:  ex.Name,
				Date:     time.Now().UTC(),
			}
			if err := writeExportStamp(stamp, &info); err != nil {
				log.Printf("Error writing stamp for %s: %s", ex.Name, err.Error())
			}
		}
		log.Println("Export complete.")
	}
	return &cobra.Command{
		Use:   "export <directory>",
		Short: "Exports your account data",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveFilterDirs
		},
		Run: run,
	}
}

func writeExportStamp(path string, info *ExportInfo) error {
	file, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to create export info: %s", err.Error())
	}
	defer file.Close()

	err = json.NewEncoder(file).Encode(info)
	if err != nil {
		log.Fatalf("Failed to marshal export info: %s", err.Error())
	}
	return nil
}
