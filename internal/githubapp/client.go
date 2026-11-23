// Package githubapp authenticates as an installed GitHub App using only the
// standard library. The installation token is scoped to the target repository.
package githubapp

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiBase = "https://api.github.com"

type Client struct {
	AppID         string
	PrivateKeyPEM []byte
	HTTP          *http.Client
	Now           func() time.Time
}
type Token struct {
	Value     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type FileChange struct {
	Path    string
	Content []byte
}

// CommitFiles applies all changes in one Git Data API commit to the default branch.
func (c *Client) CommitFiles(ctx context.Context, owner, repo, message string, files []FileChange) error {
	token, err := c.InstallationToken(ctx, owner, repo)
	if err != nil {
		return err
	}
	h := c.HTTP
	if h == nil {
		h = &http.Client{Timeout: 20 * time.Second}
	}
	var repository struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := c.request(ctx, h, http.MethodGet, apiBase+"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo), token.Value, nil, &repository); err != nil {
		return err
	}
	if repository.DefaultBranch == "" {
		return fmt.Errorf("repository default branch was not returned")
	}
	refURL := apiBase + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/git/ref/heads/" + url.PathEscape(repository.DefaultBranch)
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := c.request(ctx, h, http.MethodGet, refURL, token.Value, nil, &ref); err != nil {
		return err
	}
	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := c.request(ctx, h, http.MethodGet, apiBase+"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/git/commits/"+ref.Object.SHA, token.Value, nil, &commit); err != nil {
		return err
	}
	type treeEntry struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
		Type string `json:"type"`
		SHA  string `json:"sha"`
	}
	entries := make([]treeEntry, 0, len(files))
	for _, f := range files {
		if f.Path == "" || strings.HasPrefix(f.Path, "/") || strings.Contains(f.Path, "..") || strings.Contains(f.Path, "\\") {
			return fmt.Errorf("invalid repository path %q", f.Path)
		}
		var blob struct {
			SHA string `json:"sha"`
		}
		blobBody, _ := json.Marshal(map[string]string{"content": string(f.Content), "encoding": "utf-8"})
		if err := c.request(ctx, h, http.MethodPost, apiBase+"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/git/blobs", token.Value, blobBody, &blob); err != nil {
			return err
		}
		entries = append(entries, treeEntry{Path: f.Path, Mode: "100644", Type: "blob", SHA: blob.SHA})
	}
	treeBody, _ := json.Marshal(map[string]any{"base_tree": commit.Tree.SHA, "tree": entries})
	var tree struct {
		SHA string `json:"sha"`
	}
	if err := c.request(ctx, h, http.MethodPost, apiBase+"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/git/trees", token.Value, treeBody, &tree); err != nil {
		return err
	}
	commitBody, _ := json.Marshal(map[string]any{"message": message, "tree": tree.SHA, "parents": []string{ref.Object.SHA}})
	var newCommit struct {
		SHA string `json:"sha"`
	}
	if err := c.request(ctx, h, http.MethodPost, apiBase+"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/git/commits", token.Value, commitBody, &newCommit); err != nil {
		return err
	}
	updateBody, _ := json.Marshal(map[string]any{"sha": newCommit.SHA, "force": false})
	return c.request(ctx, h, http.MethodPatch, refURL, token.Value, updateBody, nil)
}

func (c *Client) InstallationToken(ctx context.Context, owner, repo string) (Token, error) {
	if c.AppID == "" || len(c.PrivateKeyPEM) == 0 {
		return Token{}, fmt.Errorf("github app id and private key are required")
	}
	if owner == "" || repo == "" || strings.ContainsAny(owner+repo, "/\\") {
		return Token{}, fmt.Errorf("invalid target repository")
	}
	jwt, err := c.appJWT()
	if err != nil {
		return Token{}, err
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	base := apiBase + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/installation"
	var installation struct {
		ID int64 `json:"id"`
	}
	if err := c.request(ctx, httpClient, http.MethodGet, base, jwt, nil, &installation); err != nil {
		return Token{}, fmt.Errorf("find app installation: %w", err)
	}
	if installation.ID == 0 {
		return Token{}, fmt.Errorf("github app is not installed on %s/%s", owner, repo)
	}
	var token Token
	body := []byte(fmt.Sprintf(`{"repositories":[%q]}`, repo))
	if err := c.request(ctx, httpClient, http.MethodPost, fmt.Sprintf("%s/app/installations/%d/access_tokens", apiBase, installation.ID), jwt, body, &token); err != nil {
		return Token{}, fmt.Errorf("create installation token: %w", err)
	}
	if token.Value == "" {
		return Token{}, fmt.Errorf("github returned an empty installation token")
	}
	return token, nil
}

func (c *Client) appJWT() (string, error) {
	block, _ := pem.Decode(c.PrivateKeyPEM)
	if block == nil {
		return "", fmt.Errorf("private key is not valid PEM")
	}
	var key *rsa.PrivateKey
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = k
	} else if parsed, e := x509.ParsePKCS8PrivateKey(block.Bytes); e == nil {
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("private key must be RSA")
		}
	} else {
		return "", fmt.Errorf("parse private key")
	}
	now := time.Now().UTC()
	if c.Now != nil {
		now = c.Now().UTC()
	}
	iat := now.Add(-time.Minute).Unix()
	claims := map[string]any{"iat": iat, "exp": now.Add(9 * time.Minute).Unix(), "iss": c.AppID}
	enc := base64.RawURLEncoding
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	unsigned := enc.EncodeToString(header) + "." + enc.EncodeToString(payload)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("sign app JWT: %w", err)
	}
	return unsigned + "." + enc.EncodeToString(sig), nil
}

func (c *Client) request(ctx context.Context, h *http.Client, method, endpoint, jwt string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+jwt)
	if method == http.MethodPost || method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("decode GitHub API response: %w", err)
		}
	}
	return nil
}
