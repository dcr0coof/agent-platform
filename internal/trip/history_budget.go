package trip

import (
	"encoding/json"

	"github.com/dcr0coof/agent-platform/internal/llm"
)

// Initial history only: constraints, current input, schemas and tool-loop growth
// are separately covered by the client's whole-request limit. This is not tokens.
const historyByteLimit = 128 * 1024

// historyWithinBytes takes persisted complete user turns and keeps a contiguous
// newest suffix. Never skip a large recent turn to revive older context, or split
// a tool call from its results. Each candidate turn is serialized at most once.
func historyWithinBytes(messages []llm.Message, limit int) ([]llm.Message, int, error) {
	start, size := len(messages), 2 // JSON []
	for end := len(messages); end > 0; {
		begin := end - 1
		for begin > 0 && messages[begin].Role != llm.RoleUser {
			begin--
		}
		encoded, err := json.Marshal(messages[begin:end])
		if err != nil {
			return nil, 0, err
		}
		candidate := size + len(encoded) - 2
		if start < len(messages) {
			candidate++ // comma between retained turns
		}
		if candidate > limit {
			break
		}
		start, size, end = begin, candidate, begin
	}
	return messages[start:], size, nil
}
