package correlation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kkjayakumar/AegIsOps/internal/store"
)

// FingerprintFromEvents returns a stable SHA-256 key for deduplicating incidents.
// Prefers Copilot/Sentry fingerprint when present; otherwise hashes title + culprit.
func FingerprintFromEvents(events []store.WebhookEvent) string {
	for _, ev := range events {
		if ev.Source != "copilot" {
			continue
		}
		var data map[string]any
		if json.Unmarshal(ev.Payload, &data) != nil {
			continue
		}
		issue, ok := data["issue"].(map[string]any)
		if !ok {
			continue
		}
		if fp, ok := issue["fingerprint"].(string); ok && fp != "" {
			return normalizeFingerprint(fp)
		}
		title, _ := issue["title"].(string)
		culprit, _ := issue["culprit"].(string)
		if title != "" {
			return hashParts(title, culprit)
		}
	}
	for _, ev := range events {
		if ev.Source != "spotting" {
			continue
		}
		var data map[string]any
		if json.Unmarshal(ev.Payload, &data) != nil {
			continue
		}
		if failures, ok := data["networkFailures"].([]any); ok && len(failures) > 0 {
			if f, ok := failures[0].(map[string]any); ok {
				return hashParts(fmt.Sprint(f["method"]), fmt.Sprint(f["url"]), fmt.Sprint(f["status"]))
			}
		}
	}
	return ""
}

func normalizeFingerprint(fp string) string {
	fp = strings.TrimSpace(fp)
	if len(fp) == 64 && isHex(fp) {
		return fp
	}
	sum := sha256.Sum256([]byte(fp))
	return hex.EncodeToString(sum[:])
}

func hashParts(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		if p == "" {
			continue
		}
		h.Write([]byte(p))
		h.Write([]byte("|"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
