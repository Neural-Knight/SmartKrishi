package chat

import "testing"

func TestGenerateChatTitle(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"short", "How do I grow rice?", "How do I grow rice?"},
		{"trim spaces", "   hello   ", "hello"},
		{"empty", "", "New Chat"},
		{"only spaces", "     ", "New Chat"},
		{"exactly 50", "01234567890123456789012345678901234567890123456789", "01234567890123456789012345678901234567890123456789"},
		{"truncated", "012345678901234567890123456789012345678901234567890123", "01234567890123456789012345678901234567890123456789..."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := GenerateChatTitle(c.in); got != c.want {
				t.Errorf("GenerateChatTitle(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
