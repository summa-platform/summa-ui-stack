package main

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {

	p := func(s string) (duration time.Duration) {
		duration, err := time.ParseDuration(s)
		if err != nil {
			t.Errorf("parseDuration() test parameter error: time.ParseDuration(): %q", err)
		}
		return
	}

	type ret struct {
		duration time.Duration
		years    int
		months   int
		days     int
		err      error
	}

	cases := []struct {
		in   string
		want ret
	}{
		{"1y2mo3w4d5h6m7s", ret{p("5h6m7s"), 1, 2, 3*7 + 4, nil}},
		{"1y2mo3w4d5h6m67s", ret{p("5h7m7s"), 1, 2, 3*7 + 4, nil}},
		{" 1y2M3w4d5.5h6m7s", ret{p("5h36m7s"), 1, 2, 3*7 + 4, nil}},
		{"1y 2mo3w4.5d5h6m67s", ret{p("17h7m7s"), 1, 2, 3*7 + 4, nil}},
		{"-1y2mo3w4d5h6m7s", ret{p("-5h6m7s"), -1, -2, -(3*7 + 4), nil}},
		{"- 1y2mo3w4d5h6m7s", ret{p("-5h6m7s"), -1, -2, -(3*7 + 4), nil}},
	}

	if t.Failed() {
		return
	}

	for _, c := range cases {
		duration, years, months, days, err := parseDuration(c.in)
		if err != nil && c.want.err == nil {
			t.Errorf("parseDuration(%q) => got unexpected error: %q", c.in, err)
			continue
		} else if err != nil && c.want.err != err {
			t.Errorf("parseDuration(%q) => got error %q, want error %q", c.in, err, c.want.err)
			continue
		}
		if years != c.want.years {
			t.Errorf("parseDuration(%q) => years=%v, want %v", c.in, years, c.want.years)
		}
		if months != c.want.months {
			t.Errorf("parseDuration(%q) => months=%v, want %v", c.in, months, c.want.months)
		}
		if days != c.want.days {
			t.Errorf("parseDuration(%q) => days=%v, want %v", c.in, days, c.want.days)
		}
		if duration != c.want.duration {
			t.Errorf("parseDuration(%q) => duration=%q, want %q", c.in, duration, c.want.duration)
		}
	}
}

func TestTimeAddAndSubtractDurationString(t *testing.T) {

	type input struct {
		since          time.Time
		durationString string
	}

	type output struct {
		result time.Time
		err    error
	}

	caseSets := [][]struct {
		in   input
		want output
	}{
		// add
		{
			{
				input{
					time.Date(2000, time.June, 30, 13, 55, 45, 0, time.UTC),
					"1d",
				},
				output{
					time.Date(2000, time.July, 1, 13, 55, 45, 0, time.UTC),
					nil,
				},
			},
			{
				input{
					time.Date(2000, time.June, 30, 13, 55, 45, 0, time.UTC),
					"-1mo1d2h",
				},
				output{
					time.Date(2000, time.May, 29, 11, 55, 45, 0, time.UTC),
					nil,
				},
			},
			{
				input{
					time.Date(2000, time.June, 30, 13, 55, 45, 0, time.UTC),
					"-24h",
				},
				output{
					time.Date(2000, time.June, 29, 13, 55, 45, 0, time.UTC),
					nil,
				},
			},
			{
				input{
					time.Date(2000, time.June, 30, 13, 55, 45, 0, time.UTC),
					"-2w",
				},
				output{
					time.Date(2000, time.June, 16, 13, 55, 45, 0, time.UTC),
					nil,
				},
			},
		},
		// subtract
		{
			{
				input{
					time.Date(2000, time.June, 30, 13, 55, 45, 0, time.UTC),
					"1d",
				},
				output{
					time.Date(2000, time.June, 29, 13, 55, 45, 0, time.UTC),
					nil,
				},
			},
			{
				input{
					time.Date(2000, time.June, 30, 13, 55, 45, 0, time.UTC),
					"1mo2h",
				},
				output{
					time.Date(2000, time.May, 30, 11, 55, 45, 0, time.UTC),
					nil,
				},
			},
			{
				input{
					time.Date(2000, time.June, 30, 13, 55, 45, 0, time.UTC),
					"-1mo1d2h",
				},
				output{
					time.Date(2000, time.July, 31, 15, 55, 45, 0, time.UTC),
					nil,
				},
			},
		},
	}

	for s, cases := range caseSets {
		for _, c := range cases {
			var (
				fn     string
				result time.Time
				err    error
			)
			if s == 0 {
				result, err = timeAddDurationString(c.in.since, c.in.durationString)
				fn = "timeAddDurationString"
			} else if s == 1 {
				result, err = timeSubtractDurationString(c.in.since, c.in.durationString)
				fn = "timeSubtractDurationString"
			}

			if err != nil && c.want.err == nil {
				t.Errorf("%s(%q) => got unexpected error: %q", fn, c.in, err)
			} else if err != nil && c.want.err != err {
				t.Errorf("%s(%q) => got error %q, want error %q", fn, c.in, err, c.want.err)
			}
			if !result.Equal(c.want.result) {
				t.Errorf("%s(%q) => result=%q, want %q", fn, c.in, result, c.want.result)
			}
		}
	}
}
