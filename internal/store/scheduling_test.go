package store

import (
	"testing"
	"time"
)

func TestDailyTimezoneTransitions(t *testing.T) {
	cases := []struct{ now, clock, zone, want string }{
		{"2026-03-29T00:00:00Z", "01:30", "Europe/London", "2026-03-29T01:00:00Z"},
		{"2026-10-25T00:45:00Z", "01:30", "Europe/London", "2026-10-26T01:30:00Z"},
		{"2026-09-08T03:00:00Z", "02:00", "UTC", "2026-09-09T02:00:00Z"},
	}
	for _, c := range cases {
		now, _ := time.Parse(time.RFC3339, c.now)
		got, err := nextDaily(now, c.clock, c.zone)
		if err != nil || got.UTC().Format(time.RFC3339) != c.want {
			t.Fatalf("%+v: %s %v", c, got, err)
		}
	}
}
func TestScheduleValidation(t *testing.T) {
	for _, c := range []CompilationSchedule{{Mode: "invalid"}, {Mode: "daily", Time: "27:00"}, {Mode: "daily", Timezone: "unknown/zone"}} {
		if c.NormalizeAndValidate() == nil {
			t.Fatal("invalid schedule accepted", c)
		}
	}
	c := CompilationSchedule{Mode: "daily"}
	if err := c.NormalizeAndValidate(); err != nil || c.Time != "02:00" || c.Timezone != "UTC" {
		t.Fatal(c, err)
	}
}
