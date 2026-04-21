package main

import (
	"log"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate-save-layout" {
		if err := runSaveLayoutMigration(os.Args[2:]); err != nil {
			log.Fatalf("save layout migration failed: %v", err)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "rescan-saves" {
		if err := runSaveRescan(os.Args[2:]); err != nil {
			log.Fatalf("save rescan failed: %v", err)
		}
		return
	}

	app := newApp()
	if err := app.initSaveStore(); err != nil {
		log.Fatalf("failed to initialize save storage: %v", err)
	}
	serve(newRouter(app))
}
