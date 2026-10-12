package lsp

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/prelude"
)

type fragmentReader struct{ io.Reader }

func (r fragmentReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.Reader.Read(p)
}

func TestReadFrames(t *testing.T) {
	bodies := []string{`{"method":"中文"}`, `{"id":2}`, "{}"}
	var input strings.Builder
	for _, body := range bodies {
		fmt.Fprintf(&input, "Content-Length: %d\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n%s", len(body), body)
	}
	for _, fragmented := range []bool{false, true} {
		var source io.Reader = strings.NewReader(input.String())
		if fragmented {
			source = fragmentReader{source}
		}
		reader := bufio.NewReader(source)
		for _, want := range bodies {
			got, err := unpackFrame(readFrame(reader))
			if err != nil || got != want {
				t.Fatalf("fragmented=%v: got %q, %v; want %q", fragmented, got, err, want)
			}
		}
		if _, err := unpackFrame(readFrame(reader)); err != io.EOF {
			t.Fatalf("expected EOF, got %v", err)
		}
	}
}

func TestRejectInvalidFrames(t *testing.T) {
	for _, frame := range []string{"\r\n{}", "Content-Length: -1\r\n\r\n", "Content-Length: 4\r\n\r\nx", "Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}"} {
		if _, err := unpackFrame(readFrame(bufio.NewReader(strings.NewReader(frame)))); err == nil {
			t.Fatalf("accepted invalid frame %q", frame)
		}
	}
}

func unpackFrame(result prelude.Result[string, error]) (string, error) {
	switch value := result.(type) {
	case prelude.Result__Ok[string, error]:
		return value.F0, nil
	case prelude.Result__Err[string, error]:
		return "", value.F0
	default:
		panic("invalid frame result")
	}
}
