package executionbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"agent/logger"
	"agent/models/session"
)

/*
nkk: NEW IMPLEMENTATION
Notes by nkk:
- Implements BatchWriter for batching session saves to reduce network overhead.
- Flushes either when maxSize is reached or after flushInterval.
- Uses provided httpClient for connection pooling.
- Aligns with architecture plan for high concurrency and low latency.
*/
type BatchWriter struct {
	buffer       []*session.Session
	mu           sync.Mutex
	maxSize      int
	flushTimer   *time.Timer
	endpoint     string
	httpClient   *http.Client
	flushInterval time.Duration
}

func NewBatchWriter(endpoint string, maxSize int, flushInterval time.Duration) *BatchWriter {
	return &BatchWriter{
		buffer:       make([]*session.Session, 0, maxSize),
		maxSize:      maxSize,
		endpoint:     endpoint,
		httpClient:   &http.Client{},
		flushInterval: flushInterval,
	}
}

func (b *BatchWriter) Add(session *session.Session) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer = append(b.buffer, session)

	if len(b.buffer) >= b.maxSize {
		b.flushLocked()
	} else if b.flushTimer == nil {
		b.flushTimer = time.AfterFunc(b.flushInterval, b.Flush)
	}
}

func (b *BatchWriter) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushLocked()
}

func (b *BatchWriter) flushLocked() {
	if len(b.buffer) == 0 {
		return
	}

	batch := b.buffer
	b.buffer = nil

	go b.sendBatch(batch)
}

func (b *BatchWriter) sendBatch(sessions []*session.Session) {
	body, _ := json.Marshal(map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})

	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		b.endpoint+"/batch/sessions", bytes.NewReader(body))

	resp, err := b.httpClient.Do(req)
	if err != nil {
		logger.Error("BatchWriter sendBatch error", err)
		return
	}
	defer resp.Body.Close()
}
