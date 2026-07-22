package episode

import "testing"

func TestStringArrayAndNameFallback(t *testing.T) {
	if got := stringArray([]any{"top_of_head", 1, "neck"}); len(got) != 2 || got[0] != "top_of_head" || got[1] != "neck" {
		t.Fatalf("stringArray = %#v", got)
	}
	if got := firstNonEmpty("", "цитрамон", "ibuprofen"); got != "цитрамон" {
		t.Fatalf("firstNonEmpty = %q", got)
	}
	if value, ok := asFloat(float64(6)); !ok || value != 6 {
		t.Fatalf("asFloat failed: %v %v", value, ok)
	}
}
