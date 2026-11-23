package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"inaly_pulse/internal/app"
)

func main() {
	var configPath string
	var run bool
	var force bool
	flag.StringVar(&configPath, "config", "config/monitors.json", "monitor configuration JSON")
	flag.BoolVar(&run, "run", false, "execute checks (otherwise validate configuration)")
	flag.BoolVar(&force, "force", false, "run enabled monitors regardless of interval (manual execution)")
	flag.Parse()

	if _, err := os.Stat(configPath); os.IsNotExist(err) && configPath == "config/monitors.json" {
		if err := writeSample(configPath); err != nil {
			fatal(err)
		}
		fmt.Printf("Created %s. Edit targets, then run `go run . -run`.\n", configPath)
		return
	}

	ctx := context.Background()
	result, err := app.Execute(ctx, app.Options{ConfigPath: configPath, Run: run, Force: force, Now: time.Now().UTC()})
	if err != nil {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fatal(err)
	}
}

func writeSample(path string) error {
	if err := os.MkdirAll("config", 0o755); err != nil {
		return err
	}
	data := []byte(`{
  "schema_version": 1,
  "monitors": [
    {"id":"example-http","name":"Example Website","type":"http","enabled":true,"target":"https://example.com","interval":"10m","timeout":"10s","http":{"method":"GET","expected_status":[200]}}
  ]
}
`)
	return os.WriteFile(path, data, 0o644)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "inaly-pulse:", err)
	os.Exit(1)
}
