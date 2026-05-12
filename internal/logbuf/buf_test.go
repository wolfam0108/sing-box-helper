package logbuf

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestBuffer_BasicAppend(t *testing.T) {
	b := New(10)
	fmt.Fprintln(b, "first")
	fmt.Fprintln(b, "second")
	lines := b.Tail(0)
	if len(lines) != 2 {
		t.Fatalf("len=%d, want 2", len(lines))
	}
	if lines[0].Text != "first" || lines[1].Text != "second" {
		t.Errorf("got %+v", lines)
	}
}

func TestBuffer_RingOverflow(t *testing.T) {
	b := New(3)
	for i := 0; i < 5; i++ {
		fmt.Fprintf(b, "line-%d\n", i)
	}
	lines := b.Tail(0)
	if len(lines) != 3 {
		t.Fatalf("len=%d, want 3 (max)", len(lines))
	}
	if lines[0].Text != "line-2" || lines[2].Text != "line-4" {
		t.Errorf("oldest dropped wrong, got %+v", lines)
	}
}

func TestBuffer_TailN(t *testing.T) {
	b := New(10)
	for i := 0; i < 5; i++ {
		fmt.Fprintf(b, "line-%d\n", i)
	}
	last2 := b.Tail(2)
	if len(last2) != 2 {
		t.Fatalf("tail(2) len=%d", len(last2))
	}
	if last2[0].Text != "line-3" || last2[1].Text != "line-4" {
		t.Errorf("got %+v", last2)
	}
}

func TestBuffer_PartialLineAcrossWrites(t *testing.T) {
	b := New(10)
	_, _ = b.Write([]byte("hello "))
	_, _ = b.Write([]byte("world\n"))
	lines := b.Tail(0)
	if len(lines) != 1 || lines[0].Text != "hello world" {
		t.Errorf("got %+v", lines)
	}
}

func TestBuffer_TrimCarriageReturn(t *testing.T) {
	b := New(10)
	_, _ = b.Write([]byte("crlf-line\r\n"))
	lines := b.Tail(0)
	if len(lines) != 1 || lines[0].Text != "crlf-line" {
		t.Errorf("got %+v", lines)
	}
}

func TestBuffer_Concurrent(t *testing.T) {
	b := New(100)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				fmt.Fprintf(b, "w%d-i%d\n", w, i)
			}
		}(w)
	}
	wg.Wait()
	lines := b.Tail(0)
	if len(lines) != 100 {
		t.Errorf("len=%d, want 100", len(lines))
	}
	// All lines should match the expected pattern.
	for _, l := range lines {
		if !strings.HasPrefix(l.Text, "w") {
			t.Errorf("unexpected line: %q", l.Text)
		}
	}
}
