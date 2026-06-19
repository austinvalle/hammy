package dad

import (
	"math/rand"
	"time"
)

type dadRecord struct {
	UserID     string
	Year       int
	Month      int
	IsOverride bool
}

var estLocation *time.Location

func init() {
	var err error
	estLocation, err = time.LoadLocation("America/New_York")
	if err != nil {
		panic("failed to load America/New_York timezone: " + err.Error())
	}
}

// selectDad picks a dad from candidates using the fairness algorithm:
// - excludes last month's pick (unless they're the only candidate)
// - counts only non-override appearances in the last 12 months
// - picks randomly from the group tied for the fewest appearances
func selectDad(candidates []string, history []dadRecord, now time.Time) string {
	if len(candidates) == 0 {
		return ""
	}

	lastMonth := now.AddDate(0, -1, 0)
	lastMonthUserID := ""
	for _, r := range history {
		if r.Year == lastMonth.Year() && r.Month == int(lastMonth.Month()) {
			lastMonthUserID = r.UserID
			break
		}
	}

	eligible := candidates
	if lastMonthUserID != "" && len(candidates) > 1 {
		filtered := make([]string, 0, len(candidates)-1)
		for _, c := range candidates {
			if c != lastMonthUserID {
				filtered = append(filtered, c)
			}
		}
		eligible = filtered
	}

	windowStart := now.AddDate(-1, 0, 0)
	counts := make(map[string]int, len(eligible))
	for _, c := range eligible {
		counts[c] = 0
	}

	for _, r := range history {
		if r.IsOverride {
			continue
		}
		recordDate := time.Date(r.Year, time.Month(r.Month), 1, 0, 0, 0, 0, time.UTC)
		if !recordDate.After(windowStart) {
			continue
		}
		if _, ok := counts[r.UserID]; ok {
			counts[r.UserID]++
		}
	}

	minCount := -1
	for _, count := range counts {
		if minCount == -1 || count < minCount {
			minCount = count
		}
	}

	tied := make([]string, 0, len(eligible))
	for _, c := range eligible {
		if counts[c] == minCount {
			tied = append(tied, c)
		}
	}

	return tied[rand.Intn(len(tied))]
}

// nextPickTime returns the next scheduled pick time (1st of month at 10am EST).
// If now is the 1st and before 10am, returns today at 10am. Otherwise next month.
func nextPickTime(now time.Time) time.Time {
	nowEST := now.In(estLocation)
	pickTime := time.Date(nowEST.Year(), nowEST.Month(), 1, 10, 0, 0, 0, estLocation)

	if nowEST.Before(pickTime) {
		return pickTime
	}
	return time.Date(nowEST.Year(), nowEST.Month()+1, 1, 10, 0, 0, 0, estLocation)
}

// shouldCatchUp returns true when the bot missed the scheduled pick (past 10am on
// the 1st this month) and no dad has been set for the current month yet.
func shouldCatchUp(now time.Time, currentMonthHasDad bool) bool {
	if currentMonthHasDad {
		return false
	}
	nowEST := now.In(estLocation)
	pickTime := time.Date(nowEST.Year(), nowEST.Month(), 1, 10, 0, 0, 0, estLocation)
	return !nowEST.Before(pickTime)
}
