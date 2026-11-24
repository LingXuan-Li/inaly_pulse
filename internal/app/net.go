package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// maxBodyBytes は読み取るレスポンス本文の上限。超過しても監視失敗にはしない。
	maxBodyBytes = 1 << 20
	// defaultUserAgent は Headers で User-Agent が未指定のときに使う。
	// Go 既定の UA は Wikipedia などに 403 で拒否されるため、識別可能な UA を送る。
	defaultUserAgent = "InalyPulse/1.0 (+https://github.com/LingXuan-Li/inaly_pulse)"
)

func validHTTPURL(s string) bool {
	u, e := url.ParseRequestURI(s)
	return e == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

func splitHostPort(s string) (string, string, error) { return net.SplitHostPort(s) }

func validateMonitorURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("invalid URL host")
	}
	if u.User != nil {
		return fmt.Errorf("URL userinfo is not allowed")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("localhost targets are not allowed")
	}
	if ip, err := netip.ParseAddr(host); err == nil && !publicAddress(ip) {
		return fmt.Errorf("private or special-use IP targets are not allowed")
	}
	return nil
}

func publicAddress(ip netip.Addr) bool {
	return ip.IsValid() && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !ip.IsMulticast()
}

func check(ctx context.Context, m MonitorConfig, timeout time.Duration) CheckResult {
	started := time.Now().UTC()
	r := CheckResult{MonitorID: m.ID, Type: m.Type, StartedAt: started}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	switch m.Type {
	case "http":
		method := m.HTTP.Method
		if method == "" {
			method = http.MethodGet
		}
		req, err := http.NewRequestWithContext(ctx, method, m.Target, strings.NewReader(m.HTTP.Body))
		if err != nil {
			r.Status = "failure"
			r.Error = "invalid HTTP request"
			break
		}
		for k, v := range m.HTTP.Headers {
			req.Header.Set(k, v)
		}
		if m.HTTP.ContentType != "" {
			req.Header.Set("Content-Type", m.HTTP.ContentType)
		}
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", defaultUserAgent)
		}
		if err := applyAuthentication(req, m); err != nil {
			r.Status = "failure"
			r.Error = err.Error()
			break
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil // Direct dialing is required for the resolved-IP safety check.
		transport.DialContext = secureDialContext
		client := &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if err := validateMonitorURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect rejected: %w", err)
			}
			if m.HTTP.FollowRedirects != nil && !*m.HTTP.FollowRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		}}
		resp, err := client.Do(req)
		if err != nil {
			r.Status = statusForError(err)
			r.Error = safeError(err)
			break
		}
		defer resp.Body.Close()
		var body []byte
		bodyBytes := 0
		truncated := false
		if method != http.MethodHead {
			if m.HTTP.ExpectedText != "" {
				// expected_text がある場合のみ本文を保持して検索する(先頭 maxBodyBytes まで)。
				body, err = io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
				if err != nil {
					r.Status = "failure"
					r.Error = "read response body failed"
					break
				}
				if len(body) > maxBodyBytes {
					body = body[:maxBodyBytes]
					truncated = true
				}
				bodyBytes = len(body)
			} else {
				// 本文を使わない場合は上限まで読み捨てるだけ。大きなページでも失敗にしない。
				n, copyErr := io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyBytes+1))
				if copyErr != nil {
					r.Status = "failure"
					r.Error = "read response body failed"
					break
				}
				if n > maxBodyBytes {
					n = maxBodyBytes
					truncated = true
				}
				bodyBytes = int(n)
			}
		}
		details := map[string]any{"status_code": resp.StatusCode, "response_bytes": bodyBytes}
		if truncated {
			details["body_truncated"] = true
		}
		r.Details = details
		if m.HTTP.ExpectedText != "" && !strings.Contains(string(body), m.HTTP.ExpectedText) {
			r.Status = "failure"
			if truncated {
				r.Error = "expected_text not found in the first 1 MiB of the response"
			} else {
				r.Error = "response did not contain expected_text"
			}
			break
		}
		ok := len(m.HTTP.ExpectedStatus) == 0 && resp.StatusCode >= 200 && resp.StatusCode < 400
		for _, s := range m.HTTP.ExpectedStatus {
			if resp.StatusCode == s {
				ok = true
			}
		}
		if ok {
			r.Status = "success"
		} else {
			r.Status = "failure"
			r.Error = fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode)
		}
	case "tcp":
		if err := validateResolvedTarget(ctx, m.Target); err != nil {
			r.Status = "failure"
			r.Error = err.Error()
			break
		}
		dialCtx, dialCancel := context.WithTimeout(ctx, timeout)
		conn, err := secureDialContext(dialCtx, "tcp", m.Target)
		dialCancel()
		if err != nil {
			r.Status = statusForError(err)
			r.Error = safeError(err)
			break
		}
		_ = conn.Close()
		r.Status = "success"
	case "dns":
		host := strings.TrimSuffix(m.Target, ".")
		record := m.DNS.RecordType
		if record == "" {
			record = "A"
		}
		resolvers := m.DNS.Resolvers
		if len(resolvers) == 0 {
			resolvers = []string{"system", "google", "cloudflare"}
		}
		type answer struct {
			Resolver string        `json:"resolver"`
			Response []string      `json:"response,omitempty"`
			Error    string        `json:"error,omitempty"`
			Duration time.Duration `json:"duration"`
		}
		answers := make([]answer, 0, len(resolvers))
		successes := 0
		for _, name := range resolvers {
			begin := time.Now()
			resolver := makeResolver(name)
			values, err := lookupDNS(ctx, resolver, host, record)
			a := answer{Resolver: name, Response: values, Duration: time.Since(begin)}
			if err != nil {
				a.Error = err.Error()
			} else {
				successes++
			}
			answers = append(answers, a)
		}
		if successes == 0 {
			r.Status = "failure"
			r.Error = "all DNS resolvers failed"
		} else {
			r.Status = "success"
			r.Details = map[string]any{"query": host, "record_type": record, "resolver_results": answers, "warning": successes < len(answers)}
		}
	}
	r.FinishedAt = time.Now().UTC()
	r.Duration = r.FinishedAt.Sub(started)
	return r
}

func makeResolver(name string) *net.Resolver {
	if name == "system" {
		return net.DefaultResolver
	}
	address := "8.8.8.8:53"
	if name == "cloudflare" {
		address = "1.1.1.1:53"
	}
	return &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", address)
	}}
}

func lookupDNS(ctx context.Context, resolver *net.Resolver, host, record string) ([]string, error) {
	switch record {
	case "A":
		return resolver.LookupHost(ctx, host)
	case "AAAA":
		all, err := resolver.LookupNetIP(ctx, "ip6", host)
		values := make([]string, 0, len(all))
		for _, ip := range all {
			if ip.Is6() {
				values = append(values, ip.String())
			}
		}
		return values, err
	case "CNAME":
		v, e := resolver.LookupCNAME(ctx, host)
		if e != nil {
			return nil, e
		}
		return []string{v}, nil
	case "MX":
		v, e := resolver.LookupMX(ctx, host)
		if e != nil {
			return nil, e
		}
		values := make([]string, 0, len(v))
		for _, x := range v {
			values = append(values, fmt.Sprintf("%s %d", x.Host, x.Pref))
		}
		return values, nil
	case "TXT":
		return resolver.LookupTXT(ctx, host)
	case "NS":
		v, e := resolver.LookupNS(ctx, host)
		if e != nil {
			return nil, e
		}
		values := make([]string, 0, len(v))
		for _, x := range v {
			values = append(values, x.Host)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("unsupported record type")
	}
}

func validateResolvedTarget(ctx context.Context, target string) error {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		return fmt.Errorf("invalid TCP target")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicAddress(ip) {
			return fmt.Errorf("private or special-use IP targets are not allowed")
		}
		return nil
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return fmt.Errorf("localhost targets are not allowed")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolve TCP host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("TCP host has no addresses")
	}
	for _, ip := range ips {
		if !publicAddress(ip) {
			return fmt.Errorf("private or special-use IP targets are not allowed")
		}
	}
	return nil
}

func secureDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid network address")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicAddress(ip) {
			return nil, fmt.Errorf("private or special-use IP targets are not allowed")
		}
		return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return nil, fmt.Errorf("localhost targets are not allowed")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve target: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("target has no addresses")
	}
	var last error
	for _, ip := range ips {
		if !publicAddress(ip) {
			return nil, fmt.Errorf("private or special-use IP targets are not allowed")
		}
		conn, e := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if e == nil {
			return conn, nil
		}
		last = e
	}
	return nil, last
}

func applyAuthentication(req *http.Request, m MonitorConfig) error {
	if m.Authentication == nil {
		return nil
	}
	raw := os.Getenv("INALY_MONITOR_SECRETS")
	if raw == "" {
		return fmt.Errorf("INALY_MONITOR_SECRETS is required for configured authentication")
	}
	var secrets map[string]struct {
		Type     string `json:"type"`
		Token    string `json:"token"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal([]byte(raw), &secrets); err != nil {
		return fmt.Errorf("INALY_MONITOR_SECRETS is invalid JSON")
	}
	secret, ok := secrets[m.Authentication.SecretRef]
	if !ok {
		return fmt.Errorf("authentication secret_ref was not found")
	}
	switch secret.Type {
	case "bearer":
		if secret.Token == "" {
			return fmt.Errorf("bearer token is empty")
		}
		req.Header.Set("Authorization", "Bearer "+secret.Token)
	case "basic":
		req.SetBasicAuth(secret.Username, secret.Password)
	default:
		return fmt.Errorf("authentication type must be bearer or basic")
	}
	return nil
}

func statusForError(err error) string {
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return "timeout"
	}
	if err == context.DeadlineExceeded || strings.Contains(strings.ToLower(err.Error()), "deadline exceeded") {
		return "timeout"
	}
	return "failure"
}
func safeError(err error) string {
	if ue, ok := err.(*url.Error); ok {
		return ue.Err.Error()
	}
	return err.Error()
}
