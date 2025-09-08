package flaky

import (
	"math/rand"
	"testing"
	"time"
)

// TestSometimesFails is intentionally flaky: it fails ~30% of the time.
func TestSometimesFails(t *testing.T) {
	// Seed with time so runs differ; in CI across multiple invocations this should surface.
	rand.Seed(time.Now().UnixNano())
	if rand.Float64() < 0.3 {
		t.Fatalf("flaky failure triggered")
	}
}
