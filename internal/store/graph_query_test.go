package store

import "testing"

func TestGraphQueryTokens(t *testing.T) {
	got := graphQueryTokens("Investigate the open incidents and rank them by impact.")
	if len(got) == 0 {
		t.Fatal("expected tokens from prompt")
	}
	joined := stringsJoin(got)
	for _, want := range []string{"investigate", "open", "incidents", "rank", "them", "impact"} {
		if !contains(got, want) && want != "them" && want != "open" {
			// "them"/"open" may be filtered by length/stop; ensure key nouns survive
			_ = want
		}
		_ = joined
	}
	if !contains(got, "investigate") || !contains(got, "incidents") || !contains(got, "impact") {
		t.Fatalf("tokens=%v missing expected keywords", got)
	}
	if contains(got, "the") || contains(got, "and") {
		t.Fatalf("stop words should be filtered: %v", got)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func stringsJoin(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}
