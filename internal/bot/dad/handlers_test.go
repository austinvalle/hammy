package dad

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestWhosDadHandler_NoDadSet(t *testing.T) {
	repo := &fakeRepository{}
	cmd := NewWhosDadCommand(nil, repo)
	m := &discordgo.MessageCreate{Message: &discordgo.Message{GuildID: "g1", Content: "!whosdad"}}

	resp, err := cmd.Handler(context.Background(), nil, m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "No Dad of the Month") {
		t.Errorf("expected no-dad message, got %q", resp.Content)
	}
}

func TestWhosDadHandler_DadIsSet(t *testing.T) {
	now := time.Now()
	repo := &fakeRepository{entries: []dadEntry{
		{GuildID: "g1", UserID: "bob", Year: now.Year(), Month: int(now.Month())},
	}}
	cmd := NewWhosDadCommand(nil, repo)
	m := &discordgo.MessageCreate{Message: &discordgo.Message{GuildID: "g1", Content: "!whosdad"}}

	resp, err := cmd.Handler(context.Background(), nil, m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "<@bob>") {
		t.Errorf("expected mention of bob, got %q", resp.Content)
	}
}

func TestWhosDadHandler_RepoError(t *testing.T) {
	repo := &fakeRepository{getErr: errors.New("db down")}
	cmd := NewWhosDadCommand(nil, repo)
	m := &discordgo.MessageCreate{Message: &discordgo.Message{GuildID: "g1", Content: "!whosdad"}}

	if _, err := cmd.Handler(context.Background(), nil, m); err == nil {
		t.Error("expected error to propagate from repo, got nil")
	}
}

func TestSetDadOverride_WritesCurrentAndNextMonth(t *testing.T) {
	repo := &fakeRepository{}
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)

	cur, next, err := setDadOverride(context.Background(), repo, "g1", "alice", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cur != "June 2026" {
		t.Errorf("expected current month 'June 2026', got %q", cur)
	}
	if next != "July 2026" {
		t.Errorf("expected next month 'July 2026', got %q", next)
	}

	if len(repo.entries) != 2 {
		t.Fatalf("expected 2 entries written, got %d", len(repo.entries))
	}

	for _, e := range repo.entries {
		if e.UserID != "alice" {
			t.Errorf("expected alice, got %s", e.UserID)
		}
		if !e.IsOverride {
			t.Errorf("expected IsOverride true for entry %+v", e)
		}
	}

	months := map[int]bool{}
	for _, e := range repo.entries {
		if e.Year != 2026 {
			t.Errorf("expected year 2026, got %d", e.Year)
		}
		months[e.Month] = true
	}
	if !months[6] || !months[7] {
		t.Errorf("expected June(6) and July(7) entries, got %+v", months)
	}
}

func TestSetDadOverride_HandlesDecemberToJanuaryRollover(t *testing.T) {
	repo := &fakeRepository{}
	now := time.Date(2026, time.December, 10, 12, 0, 0, 0, time.UTC)

	cur, next, err := setDadOverride(context.Background(), repo, "g1", "carol", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cur != "December 2026" {
		t.Errorf("expected 'December 2026', got %q", cur)
	}
	if next != "January 2027" {
		t.Errorf("expected 'January 2027', got %q", next)
	}

	var hasDec2026, hasJan2027 bool
	for _, e := range repo.entries {
		if e.Year == 2026 && e.Month == 12 {
			hasDec2026 = true
		}
		if e.Year == 2027 && e.Month == 1 {
			hasJan2027 = true
		}
	}
	if !hasDec2026 || !hasJan2027 {
		t.Errorf("expected Dec 2026 and Jan 2027 entries, got %+v", repo.entries)
	}
}

func TestSetDadOverride_PropagatesInsertError(t *testing.T) {
	repo := &fakeRepository{insertErr: errors.New("insert failed")}
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)

	if _, _, err := setDadOverride(context.Background(), repo, "g1", "alice", now); err == nil {
		t.Error("expected insert error to propagate, got nil")
	}
}

func TestSetDadOverride_PropagatesSecondInsertError(t *testing.T) {
	repo := &fakeRepository{insertErr: errors.New("next month insert failed"), failInsertOnCall: 2}
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)

	_, _, err := setDadOverride(context.Background(), repo, "g1", "alice", now)
	if err == nil {
		t.Fatal("expected second-insert error to propagate, got nil")
	}
	if !strings.Contains(err.Error(), "next month") {
		t.Errorf("expected next-month error context, got %v", err)
	}
}
