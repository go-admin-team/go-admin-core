package runtime

import "testing"

// A phase in a log line has to be readable as itself. "10" sends the reader
// to the source to find out which phase failed; the name does not.
func TestPhaseStringNamesEachPhase(t *testing.T) {
	for _, tc := range []struct {
		phase Phase
		want  string
	}{
		{AfterResource, "AfterResource"},
		{BeforeRouter, "BeforeRouter"},
		{AfterListen, "AfterListen"},
		{BeforeExit, "BeforeExit"},
	} {
		if got := tc.phase.String(); got != tc.want {
			t.Errorf("Phase(%d).String() = %q, want %q", int(tc.phase), got, tc.want)
		}
	}
}

// An out-of-range value must not borrow a name it does not have: the
// out-of-range reports in SetPhase and RunPhase carry this string, and a
// report naming a real phase would send the reader after the wrong one.
func TestPhaseStringOfAnUnknownValueIsNotAName(t *testing.T) {
	for _, p := range []Phase{0, 1, 42, -1, 11} {
		got := p.String()
		for _, known := range []string{"AfterResource", "BeforeRouter", "AfterListen", "BeforeExit"} {
			if got == known {
				t.Errorf("Phase(%d).String() = %q: an unknown value took a known phase's name", int(p), got)
			}
		}
	}
	if got := Phase(42).String(); got != "Phase(42)" {
		t.Errorf("Phase(42).String() = %q, want %q", got, "Phase(42)")
	}
}

// The phases are ordered as the application runs, and comparing them with <
// is meant to keep meaning "happens earlier".
func TestPhasesAreOrderedByWhenTheyHappen(t *testing.T) {
	ordered := []Phase{AfterResource, BeforeRouter, AfterListen, BeforeExit}
	for i := 1; i < len(ordered); i++ {
		if !(ordered[i-1] < ordered[i]) {
			t.Errorf("%v is not before %v", ordered[i-1], ordered[i])
		}
	}
}

// The gaps are the reason the values are written out instead of coming from
// iota: a phase added between two existing ones must be able to take a number
// of its own rather than renumber the ones already compiled into somebody
// else's binary. Switching this block to iota is what this test is here to
// catch.
func TestPhasesLeaveRoomForOneInBetween(t *testing.T) {
	ordered := []Phase{AfterResource, BeforeRouter, AfterListen, BeforeExit}
	for i := 1; i < len(ordered); i++ {
		if ordered[i]-ordered[i-1] < 2 {
			t.Errorf("no room between %v (%d) and %v (%d) for a phase added later",
				ordered[i-1], int(ordered[i-1]), ordered[i], int(ordered[i]))
		}
	}
}
