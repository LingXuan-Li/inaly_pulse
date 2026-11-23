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
	"inaly_pulse/internal/dispatch"
	"inaly_pulse/internal/githubapp"
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
	if run {
		if err := commitToGitHub(ctx, dataDir); err != nil {
			fatal(err)
		}
		cfg, err := app.LoadConfig(configPath)
		if err != nil {
			fatal(err)
		}
		client := githubClientFromEnv()
		owner, repo := repositoryParts(os.Getenv("GITHUB_REPOSITORY"))
		if err := dispatch.Run(ctx, dispatch.Options{DataDir: dataDir, Owner: owner, Repo: repo, App: client, DiscordWebhook: os.Getenv("DISCORD_WEBHOOK_URL"), Settings: cfg.Settings.Notifications, Now: time.Now().UTC()}); err != nil {
			fatal(err)
		}
		if client != nil {
			if err := commitToGitHub(ctx, dataDir); err != nil {
				fatal(err)
			}
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
	files := []githubapp.FileChange{}
	err := filepath.WalkDir(dataDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dataDir, path)
		if err != nil {
			return err
		}
		files = append(files, githubapp.FileChange{Path: "data/" + filepath.ToSlash(rel), Content: content})
		return nil
	})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	client := githubClientFromEnv()
	return client.CommitFiles(ctx, parts[0], parts[1], "chore: update Inaly Pulse monitor data", files)
}

func githubClientFromEnv() *githubapp.Client {
	appID, key := os.Getenv("INALY_APP_ID"), os.Getenv("INALY_APP_PRIVATE_KEY")
	if appID == "" && key == "" {
		return nil
	}
	if appID == "" || key == "" {
		fatal(fmt.Errorf("both INALY_APP_ID and INALY_APP_PRIVATE_KEY are required"))
	}
	return &githubapp.Client{AppID: appID, PrivateKeyPEM: []byte(key)}
}
func repositoryParts(value string) (string, string) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
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
