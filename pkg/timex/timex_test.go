package timex

import (
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	if got := Format(time.Time{}); got != "" {
		t.Fatalf("零值应输出空串，got %q", got)
	}
	tm := time.Date(2026, 10, 12, 8, 30, 5, 0, time.Local)
	if got, want := Format(tm), "2026-10-12 08:30:05"; got != want {
		t.Fatalf("Format = %q, want %q", got, want)
	}
}
