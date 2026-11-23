package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"inaly_pulse/internal/app"
)

func main() {
	var configPath string
	var run bool
	var force bool
	var dataDir string
	flag.StringVar(&configPath, "config", "config/monitors.json", "monitor configuration JSON")
	flag.BoolVar(&run, "run", false, "execute checks (otherwise validate configuration)")
	flag.BoolVar(&force, "force", false, "run enabled monitors regardless of interval (manual execution)")
	flag.StringVar(&dataDir, "data-dir", "data", "directory for JSON state and history")
	flag.Parse()

	if _, err := os.Stat(configPath); os.IsNotExist(err) && configPath == "config/monitors.json" {
		if err := writeSample(configPath); err != nil {
			fatal(err)
		}
		fmt.Printf("Created %s. Edit targets, then run `go run . -run`.\n", configPath)
		return
	}

	ctx := context.Background()
	result, err := app.Execute(ctx, app.Options{ConfigPath: configPath, DataDir: dataDir, Run: run, Force: force, Now: time.Now().UTC()})
	if err != nil {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fatal(err)
	}
	if run && len(result.Results) > 0 {
		if err := commitToGitHub(ctx, dataDir); err != nil {
			fatal(err)
		}
	}
}

func commitToGitHub(ctx context.Context, dataDir string) error {
	appID := os.Getenv("INALY_APP_ID")
	key := os.Getenv("INALY_APP_PRIVATE_KEY")
	repository := os.Getenv("GITHUB_REPOSITORY")
	if appID == "" && key == "" {
		return nil
	}
	if appID == "" || key == "" || repository == "" {
		return fmt.Errorf("GitHub storage requires INALY_APP_ID, INALY_APP_PRIVATE_KEY, and GITHUB_REPOSITORY")
	}
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return fmt.Errorf("GITHUB_REPOSITORY must be owner/repository")
	}
	paths := []string{"state/monitor_states.json", "state/current.json"}
	today := time.Now().UTC().Format("2006/01/02")
	paths = append(paths, "history/"+today+".json")
	files := make([]githubapp.FileChange, 0, len(paths))
	for _, rel := range paths {
		local := filepath.Join(dataDir, filepath.FromSlash(rel))
		content, err := os.ReadFile(local)
		if err != nil {
			return fmt.Errorf("read generated data %s: %w", rel, err)
		}
		files = append(files, githubapp.FileChange{Path: "data/" + rel, Content: content})
	}
	client := githubapp.Client{AppID: appID, PrivateKeyPEM: []byte(key)}
	return client.CommitFiles(ctx, parts[0], parts[1], "chore: update Inaly Pulse monitor data", files)
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
