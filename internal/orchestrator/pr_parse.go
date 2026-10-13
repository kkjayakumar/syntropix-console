package orchestrator

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	aegisBranchPattern = regexp.MustCompile(`^aegis/(fix|feat)-(\d+)$`)
	fixesIssuePattern  = regexp.MustCompile(`(?i)(?:fixes|closes|resolves)\s+#(\d+)`)
)

func parseAegisBranch(branch string) int64 {
	branch = strings.TrimSpace(branch)
	m := aegisBranchPattern.FindStringSubmatch(branch)
	if len(m) < 3 {
		return 0
	}
	id, err := strconv.ParseInt(m[2], 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func ParseAegisBranchFromRef(ref string) int64 {
	return parseAegisBranch(strings.TrimPrefix(ref, "refs/heads/"))
}

func parseFixesIssueReference(body string) int {
	m := fixesIssuePattern.FindStringSubmatch(body)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}
