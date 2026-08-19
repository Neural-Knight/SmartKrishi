package tools

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/domain"
)

// MessageReader is the narrow read-only surface the chat_history tool needs
// from the chat store. The existing *postgres.ChatRepository satisfies it, so
// the tool reads from Postgres per the locked decision — not the Python SQLite.
// A fake implementation is used in tests.
type MessageReader interface {
	// GetByID returns a non-deleted chat scoped to the user (used to verify
	// chat ownership before reading its messages). Implementations return a
	// not-found error when the chat does not belong to the user.
	GetByID(ctx context.Context, chatID uuid.UUID, userID int32) (*domain.Chat, error)
	// GetMessages returns all messages for a chat, oldest-first.
	GetMessages(ctx context.Context, chatID uuid.UUID) ([]domain.ChatMessage, error)
	// SearchMessages returns the user's messages matching any term (recent
	// first, capped at limit). chatID nil = across all the user's chats.
	SearchMessages(ctx context.Context, userID int32, terms []string, chatID *uuid.UUID, limit int32) ([]domain.ChatMessage, error)
	// ListSummaries returns the user's chats (used when neither a query nor a
	// chat id is given).
	ListSummaries(ctx context.Context, userID int32, skip, limit int32) ([]domain.ChatSummary, error)
}

// ChatHistory fetches prior conversation context from Postgres. It ports the
// Python chat_history_tool semantics:
//   - query non-empty + chat_id -> keyword search within that chat
//   - query non-empty + no chat_id -> keyword search across ALL the user's
//     chats (Python search_messages with chat_id=None), chat_id "all_chats"
//   - query empty + chat_id -> recent messages from that chat
//   - query empty + no chat_id -> the user's chat list
//
// IDs arrive as strings from agent.State. chat_id is a UUID and user_id is the
// integer user PK (as a string), matching the Go chat schema. Every message
// read is user-scoped: a specific chat_id is verified to belong to the user
// (via GetByID) before its messages are read. Any error is returned inside the
// result map (never panics), mirroring the Python tool.
func (r *Registry) ChatHistory(ctx context.Context, args ChatHistoryArgs) map[string]any {
	if strings.TrimSpace(args.UserID) == "" {
		return map[string]any{"error": "user_id is required"}
	}
	if r.reader == nil {
		return map[string]any{"error": "chat history unavailable (no data store configured)"}
	}
	userID64, err := strconv.ParseInt(args.UserID, 10, 32)
	if err != nil {
		return map[string]any{"error": "invalid user_id"}
	}
	userID := int32(userID64)

	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}
	query := strings.TrimSpace(args.Query)

	// Resolve an optional chat id and verify it belongs to the user.
	var chatID *uuid.UUID
	if args.ChatID != "" {
		parsed, err := uuid.Parse(args.ChatID)
		if err != nil {
			return map[string]any{"error": "invalid chat_id"}
		}
		if _, err := r.reader.GetByID(ctx, parsed, userID); err != nil {
			// Chat does not exist or is not this user's.
			return map[string]any{"error": "chat not found for user"}
		}
		chatID = &parsed
	}

	// Path 1 & 2: keyword search (in-chat or cross-chat).
	if query != "" {
		msgs, err := r.reader.SearchMessages(ctx, userID, strings.Fields(strings.ToLower(query)), chatID, int32(limit))
		if err != nil {
			return map[string]any{"error": "failed to fetch chat history: " + err.Error()}
		}
		scope := "all_chats"
		if chatID != nil {
			scope = args.ChatID
		}
		return map[string]any{
			"type":     "search_results",
			"query":    query,
			"chat_id":  scope,
			"messages": toHistoryItems(msgs),
			"count":    len(msgs),
		}
	}

	// Path 3: recent messages from a specific chat.
	if chatID != nil {
		msgs, err := r.reader.GetMessages(ctx, *chatID)
		if err != nil {
			return map[string]any{"error": "failed to fetch chat history: " + err.Error()}
		}
		items := capItems(toHistoryItems(msgs), limit)
		return map[string]any{
			"type":     "recent_messages",
			"chat_id":  args.ChatID,
			"messages": items,
			"count":    len(items),
		}
	}

	// Path 4: no query, no chat id -> list the user's chats.
	summaries, err := r.reader.ListSummaries(ctx, userID, 0, int32(limit))
	if err != nil {
		return map[string]any{"error": "failed to fetch chat history: " + err.Error()}
	}
	chats := make([]map[string]any, 0, len(summaries))
	for _, s := range summaries {
		chats = append(chats, map[string]any{
			"chat_id":       s.ID.String(),
			"chat_name":     s.Title,
			"message_count": s.MessageCount,
		})
	}
	return map[string]any{
		"type":  "user_chats",
		"chats": chats,
		"count": len(chats),
	}
}

type historyItem struct {
	Role string `json:"role"`
	Msg  string `json:"msg"`
}

func toHistoryItems(msgs []domain.ChatMessage) []historyItem {
	items := make([]historyItem, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, historyItem{Role: m.Role, Msg: m.Content})
	}
	return items
}

// capItems returns the most recent n items (messages are ordered oldest-first).
func capItems(items []historyItem, n int) []historyItem {
	if n <= 0 || len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}
