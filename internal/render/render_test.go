package render

import (
	"strings"
	"testing"
)

func TestUntrustedMarkdown(t *testing.T) {
	output, err := ToHTML("# Safe\n\n```go\nfunc main() {}\n```\n\n<script>fetch('/v1/account/token/reveal')</script><img src=x onerror=alert(1)><a href=javascript:alert(1)>bad</a>\n\n- [x] task")
	if err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"<script", "onerror", "javascript:", "fetch("} {
		if strings.Contains(output, unsafe) {
			t.Fatalf("unsafe HTML survived: %s", unsafe)
		}
	}
	for _, expected := range []string{"<h1", "language-go", "checkbox"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("Markdown feature removed: %s", expected)
		}
	}
}
