package main

import "testing"

// TestFormatDuration pins the scale switches. A step that renders as
// "0.0s" instead of "19ms" reads as "free", which is the opposite of
// what the report exists to show.
func TestFormatDuration(t *testing.T) {
	cases := []struct {
		name    string
		seconds float64
		present bool
		want    string
	}{
		{"absent", 0, false, emDash},
		{"sub-second stays in ms", 0.0136, true, "14ms"},
		{"zero is a real measurement", 0, true, "0ms"},
		{"seconds to one place", 10.02, true, "10.0s"},
		{"just under a minute", 59.94, true, "59.9s"},
		{"minute boundary", 60, true, "1m 00.0s"},
		{"minutes zero-pad the seconds", 122.623, true, "2m 02.6s"},
		{"long run", 205.88, true, "3m 25.9s"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := formatDuration(testCase.seconds, testCase.present)
			if got != testCase.want {
				t.Errorf("formatDuration(%v, %v) = %q, want %q",
					testCase.seconds, testCase.present, got, testCase.want)
			}
		})
	}
}

// TestFormatMoney pins four-decimal money. Rounding to cents would show
// a $0.0041 validation turn as free.
func TestFormatMoney(t *testing.T) {
	cases := []struct {
		name  string
		value float64
		want  string
	}{
		{"zero means unreported", 0, emDash},
		{"fraction of a cent survives", 0.0041, "$0.0041"},
		{"typical turn", 0.70119, "$0.7012"},
		{"suite total", 1.3265, "$1.3265"},
		{"thousands are grouped", 1234.5678, "$1,234.5678"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := formatMoney(testCase.value)
			if got != testCase.want {
				t.Errorf("formatMoney(%v) = %q, want %q", testCase.value, got, testCase.want)
			}
		})
	}
}

// TestFormatCount pins token grouping at every digit boundary, since an
// off-by-one in the grouping walk only shows up at specific widths.
func TestFormatCount(t *testing.T) {
	cases := []struct {
		value int
		want  string
	}{
		{0, emDash},
		{7, "7"},
		{999, "999"},
		{1000, "1,000"},
		{9984, "9,984"},
		{283766, "283,766"},
		{1000000, "1,000,000"},
		{12345678, "12,345,678"},
	}

	for _, testCase := range cases {
		got := formatCount(testCase.value)
		if got != testCase.want {
			t.Errorf("formatCount(%d) = %q, want %q", testCase.value, got, testCase.want)
		}
	}
}

// TestEscapeHTML pins the entity spellings. These are load-bearing for
// output stability, not just for safety.
func TestEscapeHTML(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{testPlain, testPlain},
		{"a & b", "a &amp; b"},
		{"<startup>", "&lt;startup&gt;"},
		{`say "hi"`, "say &quot;hi&quot;"},
		{"it's", "it&#x27;s"},
		{"<a href=\"x\">&", "&lt;a href=&quot;x&quot;&gt;&amp;"},
	}

	for _, testCase := range cases {
		got := escapeHTML(testCase.in)
		if got != testCase.want {
			t.Errorf("escapeHTML(%q) = %q, want %q", testCase.in, got, testCase.want)
		}
	}
}

// TestShareOfGuardsEmptyRun makes sure an empty run renders zeroes
// rather than NaN widths that would blow out the layout.
func TestShareOfGuardsEmptyRun(t *testing.T) {
	if got := shareOf(5, 0); got != 0 {
		t.Errorf("shareOf(5, 0) = %v, want 0", got)
	}

	if got := shareOf(1, 4); got != 25 {
		t.Errorf("shareOf(1, 4) = %v, want 25", got)
	}
}

// TestClampPercent keeps a rounding overshoot from drawing a bar wider
// than its track.
func TestClampPercent(t *testing.T) {
	cases := map[float64]float64{-3: 0, 0: 0, 42.5: 42.5, 100: 100, 100.7: 100}
	for in, want := range cases {
		if got := clampPercent(in); got != want {
			t.Errorf("clampPercent(%v) = %v, want %v", in, got, want)
		}
	}
}
