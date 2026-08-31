package dad

import "context"

// fakeRepository is an in-memory Repository for tests. It never touches pgx/sqlx.
type fakeRepository struct {
	entries   []dadEntry
	insertErr error
	getErr    error

	// failInsertOnCall, when > 0, makes insertDad return insertErr only on the
	// Nth call (1-based), allowing tests to target a specific insert.
	failInsertOnCall int
	insertCalls      int
}

func (f *fakeRepository) getDadForMonth(_ context.Context, guildID string, year, month int) (*dadEntry, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	for i := range f.entries {
		e := f.entries[i]
		if e.GuildID == guildID && e.Year == year && e.Month == month {
			return &e, nil
		}
	}
	return nil, nil
}

func (f *fakeRepository) insertDad(_ context.Context, guildID, userID string, year, month int, isOverride bool) error {
	f.insertCalls++
	if f.failInsertOnCall > 0 {
		if f.insertCalls == f.failInsertOnCall {
			return f.insertErr
		}
	} else if f.insertErr != nil {
		return f.insertErr
	}
	// emulate upsert on (guildID, year, month)
	for i := range f.entries {
		if f.entries[i].GuildID == guildID && f.entries[i].Year == year && f.entries[i].Month == month {
			f.entries[i].UserID = userID
			f.entries[i].IsOverride = isOverride
			return nil
		}
	}
	f.entries = append(f.entries, dadEntry{
		GuildID: guildID, UserID: userID, Year: year, Month: month, IsOverride: isOverride,
	})
	return nil
}

func (f *fakeRepository) getHistory(_ context.Context, guildID string) ([]dadRecord, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	var records []dadRecord
	for _, e := range f.entries {
		if e.GuildID == guildID {
			records = append(records, dadRecord{
				UserID: e.UserID, Year: e.Year, Month: e.Month, IsOverride: e.IsOverride,
			})
		}
	}
	return records, nil
}
