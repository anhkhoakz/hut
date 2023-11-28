package main

import (
	"github.com/spf13/cobra"
	"log"
	"os"
	"path/filepath"

	"git.sr.ht/~emersion/hut/export"
)

func newImportCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		importers := make(map[string]export.Exporter)

		mc := createClient("meta", cmd)
		meta := export.NewMetaExporter(mc.Client)
		importers["meta.sr.ht"] = meta

		gc := createClient("git", cmd)
		git := export.NewGitExporter(gc.Client, gc.BaseURL)
		importers["git.sr.ht"] = git

		hc := createClient("hg", cmd)
		hg := export.NewHgExporter(hc.Client, hc.BaseURL)
		importers["hg.sr.ht"] = hg

		pc := createClient("paste", cmd)
		paste := export.NewPasteExporter(pc.Client, pc.HTTP)
		importers["paste.sr.ht"] = paste

		lc := createClient("lists", cmd)
		lists := export.NewListsExporter(lc.Client, lc.HTTP)
		importers["lists.sr.ht"] = lists

		tc := createClient("todo", cmd)
		todo := export.NewTodoExporter(tc.Client, tc.HTTP)
		importers["todo.sr.ht"] = todo

		if _, ok := os.LookupEnv("SSH_AUTH_SOCK"); !ok {
			log.Println("Warning! SSH_AUTH_SOCK is not set in your environment.")
			log.Println("Using an SSH agent is advised to avoid unlocking your SSH keys repeatedly during the import.")
		}

		resources, err := export.FindDirResources(args[0])
		if err != nil {
			log.Fatalf("Failed to find resources to import: %v", err)
		} else if len(resources) == 0 {
			log.Fatal("No data found in directory")
		}

		ctx := cmd.Context()
		log.Println("Importing account data...")

		var lastService string
		for _, res := range resources {
			importer, ok := importers[res.Service]
			if !ok {
				continue // Some services are exported but never imported
			}

			if lastService != res.Service {
				log.Println(res.Service)
				lastService = res.Service
			}

			log.Printf("\t%s", res.Name)
			if err := importer.ImportResource(ctx, filepath.Dir(res.Path)); err != nil {
				log.Printf("Error importing %q: %v", res.Path, err)
			}
		}

		log.Println("Import complete.")
	}
	return &cobra.Command{
		Use:   "import <directory>",
		Short: "Imports your account data",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveFilterDirs
		},
		Run: run,
	}
}
