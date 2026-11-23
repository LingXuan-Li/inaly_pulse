package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func validHTTPURL(s string) bool {
	u, e := url.ParseRequestURI(s)
	return e == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}
func splitHostPort(s string) (string, string, error) {
	h, p, e := net.SplitHostPort(s)
	if e != nil {
		return "", "", e
	}
	return h, p, nil
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
			r.Error = err.Error()
			break
		}
		for k, v := range m.HTTP.Headers {
			req.Header.Set(k, v)
		}
		if m.HTTP.ContentType != "" {
			req.Header.Set("Content-Type", m.HTTP.ContentType)
		}
		client := &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
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
		if method == http.MethodHead {
			r.Details = map[string]any{"status_code": resp.StatusCode, "response_bytes": 0}
		} else {
			body := make([]byte, 1<<20+1)
			n, e := resp.Body.Read(body)
			if e != nil && e.Error() != "EOF" {
				r.Status = "failure"
				r.Error = "read response body failed"
				break
			}
			body = body[:n]
			r.Details = map[string]any{"status_code": resp.StatusCode, "response_bytes": n}
			if m.HTTP.ExpectedText != "" && !strings.Contains(string(body), m.HTTP.ExpectedText) {
				r.Status = "failure"
				r.Error = "response did not contain expected_text"
				break
			}
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
		conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", m.Target)
		if err != nil {
			r.Status = statusForError(err)
			r.Error = safeError(err)
			break
		}
		_ = conn.Close()
		r.Status = "success"
	case "dns":
		host := strings.TrimSuffix(m.Target, ".")
		lookupType := m.DNS.RecordType
		if lookupType == "" {
			lookupType = "A"
		}
		resolver := net.DefaultResolver
		if len(m.DNS.Resolvers) > 0 && len(m.DNS.Resolvers) < 3 {
			r.Status = "failure"
			r.Error = "resolver selection must include all configured resolvers in this build"
			break
		}
		var values []string
		var err error
		switch lookupType {
		case "A", "AAAA":
			values, err = resolver.LookupHost(ctx, host)
		case "CNAME":
			var v string
			v, err = resolver.LookupCNAME(ctx, host)
			if err == nil {
				values = []string{v}
			}
		case "MX":
			var v []*net.MX
			v, err = resolver.LookupMX(ctx, host)
			for _, x := range v {
				values = append(values, fmt.Sprintf("%s %d", x.Host, x.Pref))
			}
		case "TXT":
			values, err = resolver.LookupTXT(ctx, host)
		case "NS":
			var v []*net.NS
			v, err = resolver.LookupNS(ctx, host)
			for _, x := range v {
				values = append(values, x.Host)
			}
		}
		if err != nil {
			r.Status = statusForError(err)
			r.Error = safeError(err)
		} else {
			r.Status = "success"
			r.Details = map[string]any{"query": host, "record_type": lookupType, "resolver": "system", "response": values}
		}
	}
	r.FinishedAt = time.Now().UTC()
	r.Duration = r.FinishedAt.Sub(started)
	return r
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
func safeError(err error) string { // Avoid logging URLs that may contain userinfo/query secrets.
	if ue, ok := err.(*url.Error); ok {
		return ue.Err.Error()
	}
	return err.Error()
}
