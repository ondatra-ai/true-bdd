package queryserver

import (
	"math"
	"math/rand"
	"net/http"
	"time"
)

// Reconnect signals — the CLI's typed reaction to an HTTP status or a poll
// body that reports a stale session/epoch (plan §2).
const (
	signalOK             = ""
	signalSessionGone    = "session_gone"
	signalStaleEpoch     = "stale_epoch"
	signalReplyTooLarge  = "reply_too_large"
	signalInvalidReply   = "invalid_reply"
	signalCapacity       = "capacity"
	signalTransport      = "transport"
)

// ClassifyStatus maps an HTTP status to the CLI's typed reconnect signal
// (plan §2): a 409 CLI domain conflict is NOT a reconnect signal.
func ClassifyStatus(status int) string {
	switch status {
	case http.StatusOK, http.StatusNoContent:
		return signalOK
	case http.StatusConflict:
		return signalOK
	case http.StatusNotFound:
		return signalSessionGone
	case http.StatusRequestEntityTooLarge:
		return signalReplyTooLarge
	case http.StatusBadGateway:
		return signalInvalidReply
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return signalCapacity
	default:
		return signalTransport
	}
}

// ReconnectPlan is the client's reaction to a classified failure. It is
// field-identical to the seam ReconnectDecision.
type ReconnectPlan struct {
	ReRegister    bool
	SameSessionID bool
	Backoff       bool
}

// ReconnectFor maps a signal to the reconnect plan (plan §2): a poll 404 /
// stale-epoch re-registers with the SAME client-owned session id; a transport
// failure re-registers with capped full-jitter backoff; a normal completion or
// a domain conflict never re-registers.
func ReconnectFor(signal string) ReconnectPlan {
	switch signal {
	case signalSessionGone, signalStaleEpoch:
		return ReconnectPlan{ReRegister: true, SameSessionID: true, Backoff: false}
	case signalTransport, signalCapacity, signalInvalidReply, signalReplyTooLarge:
		return ReconnectPlan{ReRegister: true, SameSessionID: true, Backoff: true}
	default:
		return ReconnectPlan{}
	}
}

// NextEpoch returns the connection_epoch a fresh register mints given the
// previous one — strictly monotonic (plan §2: a new registration invalidates
// the old one).
func NextEpoch(prev int) int {
	if prev < 0 {
		prev = 0
	}

	return prev + 1
}

// Backoff bounds — capped exponential full-jitter (plan §2).
const (
	baseBackoff = 200 * time.Millisecond
	maxBackoff  = 30 * time.Second
	// backoffGrowth is the exponential base for capped full-jitter backoff.
	backoffGrowth = 2
)

// FullJitterBackoff returns a capped, full-jitter backoff for the given
// attempt (0-based): a uniform sample in [0, min(max, base*2^attempt)).
func FullJitterBackoff(attempt int, rng *rand.Rand) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	ceiling := float64(baseBackoff) * math.Pow(backoffGrowth, float64(attempt))
	if ceiling > float64(maxBackoff) {
		ceiling = float64(maxBackoff)
	}

	if rng == nil {
		return time.Duration(ceiling)
	}

	return time.Duration(rng.Int63n(int64(ceiling) + 1))
}
