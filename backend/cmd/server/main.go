package main

import "log"

func main() {
	app := newApp()
	if err := app.initSaveStore(); err != nil {
		log.Fatalf("failed to initialize save storage: %v", err)
	}
	serve(newRouter(app))
}
