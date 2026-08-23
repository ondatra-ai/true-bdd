package history

import (
	"encoding/json"
	"io"
)

// Event is a hook payload, kept as a decoded map rather than a struct: a
// struct's typed fields would fail the WHOLE decode on a type surprise
// (e.g. agent_id as a number), turning "skip this sub-agent turn" into "log it".
type Event map[string]any

// DecodeEvent reads a hook event. A malformed one is an empty event, not an
// error: the handlers all do nothing sensible with a missing field already,
// and a stack trace on stderr would surface as a broken hook.
func DecodeEvent(reader io.Reader) Event {
	var decoded Event

	err := json.NewDecoder(reader).Decode(&decoded)
	if err != nil {
		return Event{}
	}

	return decoded
}

// str reads a string field, or "" if it is absent or another type.
func (e Event) str(key string) string {
	text, _ := e[key].(string)

	return text
}

// truthy reports whether a field is set to something Python would call true.
func (e Event) truthy(key string) bool {
	return truthy(e[key])
}

// truthy is Python's notion of truth, for the JSON types a hook event carries.
func truthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	case float64:
		return typed != 0
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}
