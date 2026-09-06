package steps

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// ErrRedisUnreachable is returned when the backend the relays coordinate
// through did not answer.
var ErrRedisUnreachable = errors.New(
	"redis://127.0.0.1:6379 did not answer; start it with `docker compose up -d redis`")

// ErrRedisProtocol is returned when what came back is not RESP this client reads.
var ErrRedisProtocol = errors.New("unreadable RESP reply")

// ErrMalformedRedisKey is returned when a step names a key that is not
// "<kind>:<work label>".
var ErrMalformedRedisKey = errors.New("not a \"<kind>:<work label>\" key")

// ErrNoLateReply is returned when a clause is about the late reply and no
// earlier step sent one.
var ErrNoLateReply = errors.New("no earlier step replied to that work")

const (
	// redisAddr is where the compose file publishes the coordination backend:
	// loopback-only, no password, one instance for the whole suite.
	redisAddr = "127.0.0.1:6379"
	// redisURL is that address as a relay is told it.
	redisURL = "redis://" + redisAddr
	// redisTimeout caps one dial and one command.
	redisTimeout = 5 * time.Second
	// redisGoneTimeout is how long a key is watched for leaving: an orphan
	// record leaves when its TTL expires, not when the clause is read.
	redisGoneTimeout = 20 * time.Second
	// redisPollInterval is how often a key is re-read.
	redisPollInterval = 250 * time.Millisecond
	// workStateTimeout is how long a work record is watched for reaching a state.
	workStateTimeout = 15 * time.Second
	// nilBulk is the RESP length of an absent value, and crlfLen the terminator
	// that follows every bulk body.
	nilBulk = -1
	crlfLen = 2
	// workKeyKind is the key kind the relay files a work record under.
	workKeyKind = "work"
)

// redisConn is one connection to that backend. Inline commands and three
// reply shapes are the whole protocol this suite needs: it READS the relay's
// coordination state and never writes it.
type redisConn struct {
	conn   net.Conn
	reader *bufio.Reader
}

// dialRedis opens the connection and proves it answers, so a scenario that
// cannot reach the backend fails naming how to start it.
func dialRedis(state *State) (*redisConn, error) {
	dialer := net.Dialer{Timeout: redisTimeout}

	conn, err := dialer.DialContext(context.Background(), "tcp", redisAddr)
	if err != nil {
		return nil, state.fail("%w: %w", ErrRedisUnreachable, err)
	}

	client := &redisConn{conn: conn, reader: bufio.NewReader(conn)}

	pong, err := client.command(state, "PING")
	if err != nil {
		client.close()

		return nil, err
	}

	if !strings.EqualFold(pong, "PONG") {
		client.close()

		return nil, state.fail("%w: PING answered %q, want PONG", ErrRedisUnreachable, pong)
	}

	return client, nil
}

// command sends one inline command and reads the single reply it answers
// with. Inline is enough because no key this suite names carries a space.
func (c *redisConn) command(state *State, parts ...string) (string, error) {
	sent := strings.Join(parts, " ")

	err := c.conn.SetDeadline(time.Now().Add(redisTimeout))
	if err != nil {
		return "", state.fail("set the redis deadline for %q: %w", sent, err)
	}

	_, err = c.conn.Write([]byte(sent + "\r\n"))
	if err != nil {
		return "", state.fail("send %q to redis: %w", sent, err)
	}

	return c.reply(state, sent)
}

// reply reads one status, integer or bulk reply. An error reply is the
// command's own failure and comes back as one.
func (c *redisConn) reply(state *State, sent string) (string, error) {
	line, err := c.line(state, sent)
	if err != nil {
		return "", err
	}

	if line == "" {
		return "", state.fail("%w: %q answered an empty line", ErrRedisProtocol, sent)
	}

	switch line[0] {
	case '+', ':':
		return line[1:], nil
	case '$':
		return c.bulk(state, sent, line[1:])
	case '-':
		return "", state.fail("%w: %q answered %q", ErrRedisProtocol, sent, line[1:])
	default:
		return "", state.fail("%w: %q answered %q", ErrRedisProtocol, sent, line)
	}
}

// line reads one CRLF-terminated line with its terminator stripped.
func (c *redisConn) line(state *State, sent string) (string, error) {
	raw, err := c.reader.ReadString('\n')
	if err != nil {
		return "", state.fail("read redis's answer to %q: %w", sent, err)
	}

	return strings.TrimRight(raw, "\r\n"), nil
}

// bulk reads a bulk body, answering "" for the absent value — every clause
// here asks EXISTS first, so "" is never mistaken for a held empty string.
func (c *redisConn) bulk(state *State, sent, size string) (string, error) {
	length, err := strconv.Atoi(size)
	if err != nil {
		return "", state.fail("%w: %q answered bulk length %q", ErrRedisProtocol, sent, size)
	}

	if length == nilBulk {
		return "", nil
	}

	body := make([]byte, length+crlfLen)

	_, err = io.ReadFull(c.reader, body)
	if err != nil {
		return "", state.fail("read redis's %d-byte answer to %q: %w", length, sent, err)
	}

	return string(body[:length]), nil
}

// close gives the connection back.
func (c *redisConn) close() {
	_ = c.conn.Close()
}

// redisExists answers whether the key is present.
func redisExists(state *State, client *redisConn, key string) (bool, error) {
	answer, err := client.command(state, "EXISTS", key)
	if err != nil {
		return false, err
	}

	return answer == "1", nil
}

// redisGet is what the key holds, "" when it holds nothing.
func redisGet(state *State, client *redisConn, key string) (string, error) {
	return client.command(state, "GET", key)
}

// redisTTL is how many seconds the key has left. Redis answers -1 for a key
// with no expiry and -2 for one that is gone, which the clauses read literally.
func redisTTL(state *State, client *redisConn, key string) (int, error) {
	answer, err := client.command(state, "TTL", key)
	if err != nil {
		return 0, err
	}

	seconds, err := strconv.Atoi(answer)
	if err != nil {
		return 0, state.fail("%w: TTL %s answered %q", ErrRedisProtocol, key, answer)
	}

	return seconds, nil
}

// keyPrefix is the namespace every relay this scenario starts writes under, so
// one scenario's coordination state is never read as another's. Derived from
// the scenario id, so a clause resolves a key with no Given having recorded one.
func keyPrefix(state *State) string {
	return "e2e:" + state.Scenario.ID + ":"
}

// redisEnv is what a relay sharing this scenario's backend is started with.
func redisEnv(state *State) []string {
	return []string{"REDIS_URL=" + redisURL, "REDIS_KEY_PREFIX=" + keyPrefix(state)}
}

// workRecordKey is the key the relay files one claimed work item under.
func workRecordKey(state *State, item *workItem) string {
	return keyPrefix(state) + workKeyKind + ":" + item.WorkID
}

// resolveRedisKey turns the key a step names — "<kind>:<work label>" — into the
// key the relay wrote: this scenario's prefix, the kind, and the id the
// labelled poll was handed.
func resolveRedisKey(state *State, named string) (string, error) {
	kind, label, found := strings.Cut(named, ":")
	if !found {
		return "", state.fail("%w: %q", ErrMalformedRedisKey, named)
	}

	item, err := lookupWork(state, label)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%s:%s", keyPrefix(state), kind, item.WorkID), nil
}

// openKey dials the backend and resolves the key one clause is about.
func openKey(state *State, named string) (*redisConn, string, error) {
	key, err := resolveRedisKey(state, named)
	if err != nil {
		return nil, "", err
	}

	client, err := dialRedis(state)
	if err != nil {
		return nil, "", err
	}

	return client, key, nil
}
