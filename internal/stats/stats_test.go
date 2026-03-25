package stats

import (
	"testing"
	"time"
)

func TestGetDailyDetailsFrom(t *testing.T) {
	t.Run("returns recent week start when range is longer than seven days", func(t *testing.T) {
		from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 3, 25, 23, 59, 59, 0, time.UTC)

		got := getDailyDetailsFrom(from, to)

		want := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("getDailyDetailsFrom() = %v, want %v", got, want)
		}
	})

	t.Run("returns original from when range is shorter than seven days", func(t *testing.T) {
		from := time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 3, 25, 23, 59, 59, 0, time.UTC)

		got := getDailyDetailsFrom(from, to)

		if !got.Equal(from) {
			t.Fatalf("getDailyDetailsFrom() = %v, want %v", got, from)
		}
	})
}
