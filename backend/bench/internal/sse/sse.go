// Package sse provides a client-side reader for the SmartKrishi chat streaming
// endpoints (POST /api/v1/chat/send-stream and /upload-and-analyze-stream). It
// consumes the server's Server-Sent-Events body and records agent-phase timing
// metrics from the event stream. It never imports the server's agent package:
// events are parsed generically into a minimal struct holding only the fields
// the harness needs.
package sse

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// EventMetrics captures per-request agent timing derived client-side from the
// SSE event stream. All durations are measured relative to a caller-provided
// start time (the moment just before the HTTP response body starts being read).
type EventMetrics struct {
	// TimeToFirstEvent is the time until the first parseable event arrives.
	TimeToFirstEvent time.Duration
	// TimeToPlan is the time until the first "plan" event.
	TimeToPlan time.Duration
	// TimeToFirstToken is the time until the first "response_chunk" or
	// "thinking" event (whichever arrives first).
	TimeToFirstToken time.Duration
	// StreamDuration is the time from start until the "end" event or, absent
	// an explicit end, until the stream closes (EOF).
	StreamDuration time.Duration

	// ToolCallCount is the number of "tool_call" events observed.
	ToolCallCount int
	// ToolCallNames lists the tool names from "tool_call" events, in order.
	ToolCallNames []string

	// EventsTotal counts parseable frames (malformed frames are skipped).
	EventsTotal int

	// StreamCompleted reports whether an "end" event was seen.
	StreamCompleted bool
	// SawError reports whether an "error" event was seen.
	SawError bool
	// ErrorMessage holds the message from the first "error" event, if any.
	ErrorMessage string

	// FileUploaded reports whether a "file_uploaded" event was seen.
	FileUploaded bool
}

// event is a generic view of a single SSE frame. Only the fields the harness
// needs are decoded; the full agent event shape is intentionally not imported.
type event struct {
	Type  string `json:"type"`
	Tool  string `json:"tool"`
	Error string `json:"error"`
}

// Parse reads an SSE response body and returns agent-phase timing metrics.
//
// The wire format is `data: {json}\n\n`: one JSON object per frame after the
// `data: ` prefix, frames separated by a blank line. Blank lines and any
// non-`data:` lines are ignored. Timestamps are captured per event as each
// frame is read (via time.Since(start)). Parsing stops on the "end" event or
// EOF. A malformed JSON frame is skipped without failing the whole stream and
// does not increment EventsTotal. A non-nil error is returned only on an
// underlying read error (never for EOF).
func Parse(r io.Reader, start time.Time) (EventMetrics, error) {
	var m EventMetrics
	br := bufio.NewReader(r)

	// A single data line may exceed bufio.Scanner's 64KB default, so we
	// accumulate the full logical line across ReadString calls: ReadString
	// returns whatever it has (without the delimiter) when it hits EOF, and
	// the partial-plus-final pieces are joined until a newline terminates the
	// line.
	var lineBuf strings.Builder

	for {
		chunk, err := br.ReadString('\n')
		if len(chunk) > 0 {
			lineBuf.WriteString(chunk)
			// Only process once we have a complete line (terminated by '\n')
			// or we are at EOF (handled below).
			if strings.HasSuffix(chunk, "\n") {
				line := strings.TrimRight(lineBuf.String(), "\r\n")
				lineBuf.Reset()
				if done := handleLine(line, start, &m); done {
					return m, nil
				}
			}
		}
		if err != nil {
			// Flush any trailing partial line (a final frame without a
			// terminating newline).
			if err == io.EOF {
				if lineBuf.Len() > 0 {
					line := strings.TrimRight(lineBuf.String(), "\r\n")
					if done := handleLine(line, start, &m); done {
						return m, nil
					}
				}
				if !m.StreamCompleted {
					m.StreamDuration = time.Since(start)
				}
				return m, nil
			}
			return m, err
		}
	}
}

// handleLine processes one raw SSE line. It returns true when the caller should
// stop reading (an "end" event was seen).
func handleLine(line string, start time.Time, m *EventMetrics) bool {
	if line == "" {
		return false
	}
	// Only `data:` lines carry frames; ignore comments and other field lines.
	if !strings.HasPrefix(line, "data:") {
		return false
	}
	payload := strings.TrimPrefix(line, "data:")
	payload = strings.TrimPrefix(payload, " ")
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return false
	}

	var ev event
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		// Skip malformed frames without failing the stream or counting them.
		return false
	}

	now := time.Since(start)
	m.EventsTotal++
	if m.TimeToFirstEvent == 0 {
		m.TimeToFirstEvent = now
	}

	switch ev.Type {
	case "plan":
		if m.TimeToPlan == 0 {
			m.TimeToPlan = now
		}
	case "response_chunk", "thinking":
		if m.TimeToFirstToken == 0 {
			m.TimeToFirstToken = now
		}
	case "tool_call":
		m.ToolCallCount++
		m.ToolCallNames = append(m.ToolCallNames, ev.Tool)
	case "file_uploaded":
		m.FileUploaded = true
	case "error":
		if !m.SawError {
			m.SawError = true
			m.ErrorMessage = ev.Error
		}
	case "end":
		m.StreamCompleted = true
		m.StreamDuration = now
		return true
	}
	return false
}
