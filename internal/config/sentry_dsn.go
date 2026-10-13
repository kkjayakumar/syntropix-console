package config

import (
	"net/url"
	"strings"
)

// NormalizeSentryDSN rewrites Copilot ingest DSN hosts so the Sentry SDK inside
// AegisOps can reach Copilot. On same-host Docker stacks, prefer host.docker.internal
// over a public monitoring URL (EC2 hairpin NAT often blocks container -> public IP).
func NormalizeSentryDSN(dsn, copilotIngestPublicURL, copilotAPIURL string) string {
	if dsn == "" {
		return ""
	}
	internalHost := dockerInternalCopilotHost(copilotAPIURL)
	publicHost := ingestHost(copilotIngestPublicURL)

	targetHost := publicHost
	if internalHost != "" {
		targetHost = internalHost
	}
	if targetHost == "" {
		return dsn
	}

	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return strings.NewReplacer(
			"@127.0.0.1:5000/", "@"+targetHost+"/",
			"@localhost:5000/", "@"+targetHost+"/",
		).Replace(dsn)
	}

	host := strings.ToLower(u.Hostname())
	if shouldRewriteIngestHost(host, publicHost, internalHost != "") {
		u.Host = targetHost
		return u.String()
	}
	return dsn
}

func dockerInternalCopilotHost(copilotAPIURL string) string {
	if !strings.Contains(copilotAPIURL, "host.docker.internal") {
		return ""
	}
	u, err := url.Parse(copilotAPIURL)
	if err != nil || u.Host == "" {
		return "host.docker.internal:5000"
	}
	return u.Host
}

func shouldRewriteIngestHost(hostname, publicHost string, preferInternal bool) bool {
	if hostname == "127.0.0.1" || hostname == "localhost" {
		return true
	}
	if !preferInternal || publicHost == "" {
		return false
	}
	publicName := strings.ToLower(strings.Split(publicHost, ":")[0])
	return hostname == publicName
}

func ingestHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(raw), "https://"), "http://")
	}
	return u.Host
}
