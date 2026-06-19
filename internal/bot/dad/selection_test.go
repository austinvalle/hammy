package dad

import (
	"testing"
	"time"
)

// selectDad tests

func TestSelectDad_ReturnsEmptyWhenNoCandidates(t *testing.T) {
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	result := selectDad([]string{}, []dadRecord{}, now)
	if result != "" {
		t.Errorf("expected empty string for no candidates, got %s", result)
	}
}

func TestSelectDad_ExcludesLastMonthPick(t *testing.T) {
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	candidates := []string{"alice", "bob", "carol"}
	history := []dadRecord{
		{UserID: "alice", Year: 2026, Month: 5, IsOverride: false},
	}

	for i := 0; i < 100; i++ {
		result := selectDad(candidates, history, now)
		if result == "alice" {
			t.Fatal("selectDad should not pick alice who was dad last month")
		}
	}
}

func TestSelectDad_PicksLeastFrequentCandidate(t *testing.T) {
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	candidates := []string{"alice", "bob", "carol"}
	history := []dadRecord{
		{UserID: "alice", Year: 2026, Month: 1, IsOverride: false},
		{UserID: "alice", Year: 2026, Month: 3, IsOverride: false},
		{UserID: "bob", Year: 2026, Month: 2, IsOverride: false},
		// carol has never been picked
	}

	for i := 0; i < 100; i++ {
		result := selectDad(candidates, history, now)
		if result != "carol" {
			t.Fatalf("expected carol (0 picks) to be selected, got %s", result)
		}
	}
}

func TestSelectDad_OverrideMonthsDoNotCountAgainstFairness(t *testing.T) {
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	candidates := []string{"alice", "bob"}
	history := []dadRecord{
		// alice has 2 override (baby) months — should not count
		{UserID: "alice", Year: 2026, Month: 1, IsOverride: true},
		{UserID: "alice", Year: 2026, Month: 2, IsOverride: true},
		// bob has 2 normal months
		{UserID: "bob", Year: 2026, Month: 3, IsOverride: false},
		{UserID: "bob", Year: 2026, Month: 4, IsOverride: false},
	}

	// alice has 0 non-override picks, bob has 2 — alice should always win
	for i := 0; i < 100; i++ {
		result := selectDad(candidates, history, now)
		if result != "alice" {
			t.Fatalf("expected alice (0 non-override picks) to be selected, got %s", result)
		}
	}
}

func TestSelectDad_FallsBackToLastMonthPickWhenOnlyCandidate(t *testing.T) {
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	candidates := []string{"alice"}
	history := []dadRecord{
		{UserID: "alice", Year: 2026, Month: 5, IsOverride: false},
	}

	result := selectDad(candidates, history, now)
	if result != "alice" {
		t.Errorf("expected alice to be picked when she's the only candidate, got %s", result)
	}
}

func TestSelectDad_IgnoresHistoryOutside12MonthWindow(t *testing.T) {
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	candidates := []string{"alice", "bob"}
	history := []dadRecord{
		// alice was picked many times but all >12 months ago
		{UserID: "alice", Year: 2025, Month: 1, IsOverride: false},
		{UserID: "alice", Year: 2025, Month: 2, IsOverride: false},
		{UserID: "alice", Year: 2025, Month: 3, IsOverride: false},
		// bob was picked once recently
		{UserID: "bob", Year: 2026, Month: 3, IsOverride: false},
	}

	// alice's old picks don't count, so alice (0) < bob (1) — alice should always win
	for i := 0; i < 100; i++ {
		result := selectDad(candidates, history, now)
		if result != "alice" {
			t.Fatalf("expected alice (0 picks in window) to be selected, got %s", result)
		}
	}
}

func TestSelectDad_RandomlySelectsAmongTiedCandidates(t *testing.T) {
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	candidates := []string{"alice", "bob", "carol"}

	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		result := selectDad(candidates, []dadRecord{}, now)
		if result == "" {
			t.Fatal("selectDad returned empty string with valid candidates")
		}
		seen[result] = true
	}

	for _, c := range candidates {
		if !seen[c] {
			t.Errorf("candidate %s was never picked in 500 iterations — selection is not random", c)
		}
	}
}

// nextPickTime tests

func TestNextPickTime_ReturnsCurrentMonthWhenBeforePickTimeOnFirst(t *testing.T) {
	est, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, time.June, 1, 8, 0, 0, 0, est)

	next := nextPickTime(now)

	expected := time.Date(2026, time.June, 1, 10, 0, 0, 0, est)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextPickTime_ReturnsNextMonthWhenAfterPickTime(t *testing.T) {
	est, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, time.June, 5, 12, 0, 0, 0, est)

	next := nextPickTime(now)

	expected := time.Date(2026, time.July, 1, 10, 0, 0, 0, est)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextPickTime_ReturnsNextMonthWhenExactlyAtPickTime(t *testing.T) {
	est, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, est)

	next := nextPickTime(now)

	expected := time.Date(2026, time.July, 1, 10, 0, 0, 0, est)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextPickTime_HandlesDecemberToJanuary(t *testing.T) {
	est, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, time.December, 15, 12, 0, 0, 0, est)

	next := nextPickTime(now)

	expected := time.Date(2027, time.January, 1, 10, 0, 0, 0, est)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

// shouldCatchUp tests

func TestShouldCatchUp_TrueWhenPastPickTimeAndNoDadSet(t *testing.T) {
	est, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, time.June, 5, 12, 0, 0, 0, est)

	if !shouldCatchUp(now, false) {
		t.Error("expected shouldCatchUp true when past pick time and no dad set")
	}
}

func TestShouldCatchUp_FalseWhenDadAlreadySet(t *testing.T) {
	est, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, time.June, 5, 12, 0, 0, 0, est)

	if shouldCatchUp(now, true) {
		t.Error("expected shouldCatchUp false when dad already set for month")
	}
}

func TestShouldCatchUp_FalseWhenBeforePickTimeOnFirst(t *testing.T) {
	est, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, time.June, 1, 8, 0, 0, 0, est)

	if shouldCatchUp(now, false) {
		t.Error("expected shouldCatchUp false when before 10am on the 1st")
	}
}

func TestShouldCatchUp_TrueWhenJustAfterPickTime(t *testing.T) {
	est, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, time.June, 1, 10, 1, 0, 0, est)

	if !shouldCatchUp(now, false) {
		t.Error("expected shouldCatchUp true when just after 10am on the 1st")
	}
}
