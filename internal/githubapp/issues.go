package githubapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Issue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
}

func (c *Client) FindIncidentIssue(ctx context.Context, owner, repo, id string) (*Issue, error) {
	token, err := c.InstallationToken(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	h := c.httpClient()
	marker := "<!-- inaly-pulse-incident:" + id + " -->"
	base := apiBase + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/issues?state=all&per_page=100"
	for page := 1; page <= 10; page++ {
		endpoint := base + fmt.Sprintf("&page=%d", page)
		var items []Issue
		if err := c.request(ctx, h, http.MethodGet, endpoint, token.Value, nil, &items); err != nil {
			return nil, err
		}
		for i := range items {
			if strings.Contains(items[i].Body, marker) {
				return &items[i], nil
			}
		}
		if len(items) < 100 {
			break
		}
	}
	return nil, nil
}

func (c *Client) CreateIssue(ctx context.Context, owner, repo, title, body string) (Issue, error) {
	token, err := c.InstallationToken(ctx, owner, repo)
	if err != nil {
		return Issue{}, err
	}
	payload, _ := json.Marshal(map[string]string{"title": title, "body": body})
	endpoint := apiBase + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/issues"
	var issue Issue
	err = c.request(ctx, c.httpClient(), http.MethodPost, endpoint, token.Value, payload, &issue)
	return issue, err
}
func (c *Client) CreateIncidentIssue(ctx context.Context, owner, repo, id, title, body string) (Issue, error) {
	marker := "<!-- inaly-pulse-incident:" + id + " -->"
	body = marker + "\n\n" + body
	return c.createIssue(ctx, owner, repo, title, body)
}
func (c *Client) createIssue(ctx context.Context, owner, repo, title, body string) (Issue, error) {
	token, err := c.InstallationToken(ctx, owner, repo)
	if err != nil {
		return Issue{}, err
	}
	payload, _ := json.Marshal(map[string]string{"title": title, "body": body})
	var issue Issue
	endpoint := apiBase + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/issues"
	err = c.request(ctx, c.httpClient(), http.MethodPost, endpoint, token.Value, payload, &issue)
	return issue, err
}
func (c *Client) CommentIssue(ctx context.Context, owner, repo string, number int, body string) error {
	token, err := c.InstallationToken(ctx, owner, repo)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"body": body})
	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", apiBase, url.PathEscape(owner), url.PathEscape(repo), number)
	return c.request(ctx, c.httpClient(), http.MethodPost, endpoint, token.Value, payload, nil)
}
func (c *Client) GetIssue(ctx context.Context, owner, repo string, number int) (Issue, error) {
	token, err := c.InstallationToken(ctx, owner, repo)
	if err != nil {
		return Issue{}, err
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%d", apiBase, url.PathEscape(owner), url.PathEscape(repo), number)
	var issue Issue
	err = c.request(ctx, c.httpClient(), http.MethodGet, endpoint, token.Value, nil, &issue)
	return issue, err
}
func (c *Client) CloseIssue(ctx context.Context, owner, repo string, number int) error {
	token, err := c.InstallationToken(ctx, owner, repo)
	if err != nil {
		return err
	}
	payload := []byte(`{"state":"closed"}`)
	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%d", apiBase, url.PathEscape(owner), url.PathEscape(repo), number)
	return c.request(ctx, c.httpClient(), http.MethodPatch, endpoint, token.Value, payload, nil)
}
func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}
