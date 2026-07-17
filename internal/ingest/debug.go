package ingest

import (
	"os"
	"sync"
)

// debugSink appends raw event payloads to a JSONL file for troubleshooting.
type debugSink struct {
	mu sync.Mutex
	f  *os.File
}

func newDebugSink(path string) *debugSink {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil
	}
	return &debugSink{f: f}
}

func (d *debugSink) Write(raw []byte) {
	if d == nil || d.f == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_, _ = d.f.Write(raw)
	_, _ = d.f.Write([]byte("\n"))
}

func (d *debugSink) Close() {
	if d == nil || d.f == nil {
		return
	}
	_ = d.f.Close()
}
