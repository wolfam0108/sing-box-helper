// Package logbuf implements a thread-safe ring buffer of recent log lines
// for the helper itself (one-process tail). It is used by /api/logs when
// the source is "helper".
package logbuf

import (
	"bytes"
	"io"
	"sync"
	"time"
)

// Buffer keeps the last N lines written to it. Writes are append-only;
// when the buffer is full, the oldest line is dropped.
//
// Buffer implements io.Writer so it can be plugged into log.SetOutput,
// http.Server.ErrorLog, etc. Each Write is parsed into one or more lines
// (split on '\n') and each non-empty line is stored with the wall-clock
// timestamp the write happened.
type Buffer struct {
	mu     sync.Mutex
	lines  []Line
	max    int
	pendng []byte // accumulator for a partial line crossing Write boundaries
}

// Line is one stored log entry.
type Line struct {
	When time.Time `json:"when"`
	Text string    `json:"text"`
}

// New creates a Buffer that keeps up to max lines.
func New(max int) *Buffer {
	if max <= 0 {
		max = 500
	}
	return &Buffer{max: max}
}

// Write satisfies io.Writer. Splits p on '\n' and appends each completed
// line to the ring. Partial trailing data is buffered until the next call
// completes the line.
func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	b.pendng = append(b.pendng, p...)
	for {
		i := bytes.IndexByte(b.pendng, '\n')
		if i < 0 {
			break
		}
		line := bytes.TrimRight(b.pendng[:i], "\r")
		if len(line) > 0 {
			b.appendLocked(Line{When: now, Text: string(line)})
		}
		b.pendng = b.pendng[i+1:]
	}
	return len(p), nil
}

func (b *Buffer) appendLocked(l Line) {
	if len(b.lines) < b.max {
		b.lines = append(b.lines, l)
		return
	}
	// Drop oldest by shifting in-place. The buffer is bounded, so this
	// is cheap relative to the overall cost of logging.
	copy(b.lines, b.lines[1:])
	b.lines[len(b.lines)-1] = l
}

// Tail returns the most recent n lines (or fewer if not yet that many),
// oldest-first. A copy of the underlying slice is returned, so the
// caller can mutate it safely.
func (b *Buffer) Tail(n int) []Line {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n <= 0 || n >= len(b.lines) {
		out := make([]Line, len(b.lines))
		copy(out, b.lines)
		return out
	}
	out := make([]Line, n)
	copy(out, b.lines[len(b.lines)-n:])
	return out
}

// Tee returns an io.Writer that writes to b AND to other. Useful for
// preserving the original os.Stderr while also feeding the ring.
func (b *Buffer) Tee(other io.Writer) io.Writer {
	if other == nil {
		return b
	}
	return io.MultiWriter(b, other)
}
