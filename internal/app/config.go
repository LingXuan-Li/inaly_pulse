package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	SchemaVersion int             `json:"schema_version"`
	Monitors      []MonitorConfig `json:"monitors"`
	Settings      Settings        `json:"settings,omitempty"`
}

type Settings struct {
	Tolerance         string `json:"tolerance,omitempty"`
	FailureThreshold  int    `json:"failure_threshold,omitempty"`
	RecoveryThreshold int    `json:"recovery_threshold,omitempty"`
}

type MonitorConfig struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Type           string                `json:"type"`
	Enabled        bool                  `json:"enabled"`
	Target         string                `json:"target"`
	Interval       string                `json:"interval"`
	Timeout        string                `json:"timeout,omitempty"`
	HTTP           HTTPConfig            `json:"http,omitempty"`
	DNS            DNSConfig             `json:"dns,omitempty"`
	Authentication *AuthenticationConfig `json:"authentication,omitempty"`
}

type AuthenticationConfig struct {
	SecretRef string `json:"secret_ref"`
}

type HTTPConfig struct {
	Method          string            `json:"method,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Body            string            `json:"body,omitempty"`
	ContentType     string            `json:"content_type,omitempty"`
	ExpectedStatus  []int             `json:"expected_status,omitempty"`
	ExpectedText    string            `json:"expected_text,omitempty"`
	FollowRedirects *bool             `json:"follow_redirects,omitempty"`
}

type DNSConfig struct {
	RecordType string   `json:"record_type,omitempty"`
	Resolvers  []string `json:"resolvers,omitempty"`
}

func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var c Config
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return Config{}, fmt.Errorf("decode config: multiple JSON values")
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) Validate() error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("schema_version must be 1 (got %d)", c.SchemaVersion)
	}
	if len(c.Monitors) == 0 {
		return fmt.Errorf("monitors must contain at least one monitor")
	}
	seen := map[string]bool{}
	for i, m := range c.Monitors {
		p := fmt.Sprintf("monitors[%d]", i)
		if m.ID == "" || m.Name == "" || m.Target == "" {
			return fmt.Errorf("%s: id, name, and target are required", p)
		}
		if strings.ContainsAny(m.ID, "/\\") || m.ID == "." || m.ID == ".." {
			return fmt.Errorf("%s: id must not contain path separators", p)
		}
		if seen[m.ID] {
			return fmt.Errorf("%s: duplicate monitor id %q", p, m.ID)
		}
		seen[m.ID] = true
		interval, err := time.ParseDuration(m.Interval)
		if err != nil || interval < 10*time.Minute || interval%(10*time.Minute) != 0 {
			return fmt.Errorf("%s: interval must be at least 10m and a multiple of 10m", p)
		}
		if m.Timeout != "" {
			d, e := time.ParseDuration(m.Timeout)
			if e != nil || d <= 0 {
				return fmt.Errorf("%s: timeout must be a positive duration", p)
			}
		}
		switch m.Type {
		case "http":
			if !validHTTPURL(m.Target) {
				return fmt.Errorf("%s: target must be an absolute http or https URL", p)
			}
			if err := validateMonitorURL(m.Target); err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			if m.HTTP.Method != "" && m.HTTP.Method != "GET" && m.HTTP.Method != "POST" && m.HTTP.Method != "HEAD" {
				return fmt.Errorf("%s: http.method must be GET, POST, or HEAD", p)
			}
			for _, s := range m.HTTP.ExpectedStatus {
				if s < 100 || s > 599 {
					return fmt.Errorf("%s: expected HTTP status must be 100..599", p)
				}
			}
		case "tcp":
			host, port, err := splitHostPort(m.Target)
			if err != nil {
				return fmt.Errorf("%s: tcp target must be host:port", p)
			}
			if host == "" {
				return fmt.Errorf("%s: tcp host is required", p)
			}
			n, e := strconv.Atoi(port)
			if e != nil || n < 1 || n > 65535 {
				return fmt.Errorf("%s: tcp port must be between 1 and 65535", p)
			}
		case "dns":
			if m.DNS.RecordType != "" && !oneOf(m.DNS.RecordType, "A", "AAAA", "CNAME", "MX", "TXT", "NS") {
				return fmt.Errorf("%s: unsupported DNS record type", p)
			}
			for _, r := range m.DNS.Resolvers {
				if !oneOf(r, "system", "google", "cloudflare") {
					return fmt.Errorf("%s: unsupported DNS resolver %q", p, r)
				}
			}
		case "ping":
			return fmt.Errorf("%s: ping checker is not available in this local runner", p)
		default:
			return fmt.Errorf("%s: unsupported monitor type %q", p, m.Type)
		}
		if m.Authentication != nil && m.Authentication.SecretRef != "" && !secretRefRE.MatchString(m.Authentication.SecretRef) {
			return fmt.Errorf("%s: invalid authentication.secret_ref", p)
		}
	}
	if c.Settings.Tolerance != "" {
		d, e := time.ParseDuration(c.Settings.Tolerance)
		if e != nil || d < 0 || d >= 10*time.Minute {
			return fmt.Errorf("settings.tolerance must be in [0,10m)")
		}
	}
	return nil
}

var secretRefRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func oneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
