package dad

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func msgWithMentions(content string, mentionCount int) discordgo.Message {
	mentions := make([]*discordgo.User, mentionCount)
	for i := range mentions {
		mentions[i] = &discordgo.User{ID: "user123"}
	}
	return discordgo.Message{Content: content, Mentions: mentions}
}

func TestAddDadCommand_CanActivate(t *testing.T) {
	cmd := NewAddDadCommand(nil, "role1")

	tests := []struct {
		name     string
		content  string
		mentions int
		want     bool
	}{
		{"valid with mention", "!adddad @user", 1, true},
		{"no mention", "!adddad", 0, false},
		{"two mentions", "!adddad @a @b", 2, false},
		{"wrong prefix", "!addmom @user", 1, false},
		{"not a command", "hello @user", 1, false},
		{"prefix without space", "!adddad@user", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := msgWithMentions(tt.content, tt.mentions)
			if got := cmd.CanActivate(nil, m); got != tt.want {
				t.Errorf("CanActivate(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestWhosDadCommand_CanActivate(t *testing.T) {
	cmd := NewWhosDadCommand(nil, nil)

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"exact match", "!whosdad", true},
		{"with trailing text", "!whosdad now", false},
		{"wrong command", "!whodad", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := discordgo.Message{Content: tt.content}
			if got := cmd.CanActivate(nil, m); got != tt.want {
				t.Errorf("CanActivate(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestSetDadCommand_CanActivate_RequiresAdmin(t *testing.T) {
	cmd := NewSetDadCommand(nil, nil, "chan1", "role1")

	// m.Member == nil → isAdmin returns false → CanActivate false even with valid content
	m := msgWithMentions("!setdad @user", 1)
	if cmd.CanActivate(nil, m) {
		t.Error("expected CanActivate false for non-admin (nil member)")
	}
}

func TestPickDadCommand_CanActivate_RequiresAdmin(t *testing.T) {
	cmd := NewPickDadCommand(nil, nil, "chan1", "role1")

	m := discordgo.Message{Content: "!pickdad"}
	if cmd.CanActivate(nil, m) {
		t.Error("expected CanActivate false for non-admin (nil member)")
	}
}

func TestIsAdmin_FalseWhenNoMember(t *testing.T) {
	m := discordgo.Message{Content: "!pickdad"}
	if isAdmin(nil, m) {
		t.Error("expected isAdmin false when Member is nil")
	}
}

func TestIsAdmin_FalseWhenNoRoles(t *testing.T) {
	m := discordgo.Message{
		Content: "!pickdad",
		Member:  &discordgo.Member{Roles: nil},
	}
	if isAdmin(nil, m) {
		t.Error("expected isAdmin false when Member.Roles is nil")
	}
}
