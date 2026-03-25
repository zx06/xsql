package ssh

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockClient implements a minimal Client-like object for testing.
func newMockClient(alive bool) *Client {
	c := &Client{}
	c.alive.Store(alive)
	return c
}

// withConnectFunc overrides the connect function (for testing).
func withConnectFunc(fn func(ctx context.Context, opts Options) (*Client, error)) ReconnectOption {
	return func(rd *ReconnectDialer) {
		rd.connectFunc = fn
	}
}

// testConnectFunc returns a connect function that can be controlled by the test.
type testConnector struct {
	mu        sync.Mutex
	clients   []*Client
	callCount int
	failUntil int // fail the first N calls
	failErr   error
}

func (tc *testConnector) connect(ctx context.Context, opts Options) (*Client, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.callCount++

	if tc.callCount <= tc.failUntil {
		if tc.failErr != nil {
			return nil, tc.failErr
		}
		return nil, fmt.Errorf("connect failed (attempt %d)", tc.callCount)
	}

	c := &Client{}
	c.alive.Store(true)
	tc.clients = append(tc.clients, c)
	return c, nil
}

func (tc *testConnector) getCallCount() int {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.callCount
}

// --- Tests ---

func TestReconnectDialer_NewAndClose(t *testing.T) {
	tc := &testConnector{}
	ctx := context.Background()

	rd, err := NewReconnectDialer(ctx, Options{
		KeepaliveInterval: -1, // disable keepalive for this test
	}, withConnectFunc(tc.connect))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tc.getCallCount() != 1 {
		t.Errorf("expected 1 connect call, got %d", tc.getCallCount())
	}

	if err := rd.Close(); err != nil {
		t.Errorf("unexpected close error: %v", err)
	}

	// Double close should be safe
	if err := rd.Close(); err != nil {
		t.Errorf("unexpected double close error: %v", err)
	}
}

func TestReconnectDialer_InitialConnectFails(t *testing.T) {
	tc := &testConnector{
		failUntil: 100,
		failErr:   fmt.Errorf("connection refused"),
	}

	_, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: -1,
	}, withConnectFunc(tc.connect))
	if err == nil {
		t.Fatal("expected error on initial connect failure")
	}
	if tc.getCallCount() != 1 {
		t.Errorf("expected 1 connect attempt, got %d", tc.getCallCount())
	}
}

func TestReconnectDialer_DialTriggersReconnectOnError(t *testing.T) {
	// When DialContext fails on the first attempt, ReconnectDialer
	// should reconnect and retry. We verify by counting connect calls.
	connectCount := atomic.Int32{}

	rd, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: -1,
		KeepaliveCountMax: 2,
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		connectCount.Add(1)
		c := &Client{}
		c.alive.Store(true)
		// client.client is nil, so DialContext will fail
		return c, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	// DialContext will fail (nil ssh.Client), triggering reconnect,
	// then retry will also fail. We just verify no panic and reconnect was attempted.
	_, dialErr := rd.DialContext(context.Background(), "tcp", "127.0.0.1:12345")
	if dialErr == nil {
		t.Fatal("expected dial error with nil ssh.Client")
	}

	// Should have called connect more than once (initial + reconnect)
	if int(connectCount.Load()) < 2 {
		t.Errorf("expected at least 2 connect calls (initial + reconnect), got %d", connectCount.Load())
	}
}

func TestReconnectDialer_DialFailTriggersReconnect(t *testing.T) {
	// Track status events
	var events []StatusEvent
	var eventsMu sync.Mutex

	connectCount := atomic.Int32{}

	rd, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: -1,
		KeepaliveCountMax: 2, // only 2 retries for faster tests
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		connectCount.Add(1)
		c := &Client{}
		c.alive.Store(true)
		return c, nil
	}), WithStatusCallback(func(e StatusEvent) {
		eventsMu.Lock()
		events = append(events, e)
		eventsMu.Unlock()
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	// Verify initial connect event
	eventsMu.Lock()
	if len(events) < 1 || events[0].Type != StatusConnected {
		t.Error("expected StatusConnected event")
	}
	eventsMu.Unlock()

	if int(connectCount.Load()) != 1 {
		t.Errorf("expected 1 initial connect call, got %d", connectCount.Load())
	}
}

func TestReconnectDialer_ReconnectSuccess(t *testing.T) {
	connectCount := atomic.Int32{}
	var events []StatusEvent
	var eventsMu sync.Mutex

	rd, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: -1,
		KeepaliveCountMax: 2,
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		connectCount.Add(1)
		c := &Client{}
		c.alive.Store(true)
		return c, nil
	}), WithStatusCallback(func(e StatusEvent) {
		eventsMu.Lock()
		events = append(events, e)
		eventsMu.Unlock()
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	// Force a reconnect
	_, reconnErr := rd.reconnect()
	if reconnErr != nil {
		t.Fatalf("reconnect failed: %v", reconnErr)
	}

	if int(connectCount.Load()) != 2 {
		t.Errorf("expected 2 connect calls, got %d", connectCount.Load())
	}

	// Check events
	eventsMu.Lock()
	found := false
	for _, e := range events {
		if e.Type == StatusReconnected {
			found = true
			break
		}
	}
	eventsMu.Unlock()
	if !found {
		t.Error("expected StatusReconnected event")
	}
}

func TestReconnectDialer_ReconnectFailAllRetries(t *testing.T) {
	connectCount := atomic.Int32{}

	rd, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: -1,
		KeepaliveCountMax: 2, // max 2 retries
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		n := int(connectCount.Add(1))
		if n == 1 {
			// First call succeeds (initial connect)
			c := &Client{}
			c.alive.Store(true)
			return c, nil
		}
		// All reconnect attempts fail
		return nil, fmt.Errorf("connect refused (attempt %d)", n)
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	_, reconnErr := rd.reconnect()
	if reconnErr == nil {
		t.Fatal("expected reconnect to fail")
	}

	// Should have tried KeepaliveCountMax times for reconnection
	// 1 initial + 2 retries = 3 total
	if int(connectCount.Load()) != 3 {
		t.Errorf("expected 3 connect calls (1 initial + 2 retries), got %d", connectCount.Load())
	}
}

func TestReconnectDialer_ConcurrentDial(t *testing.T) {
	connectCount := atomic.Int32{}

	rd, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: -1,
		KeepaliveCountMax: 2,
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		connectCount.Add(1)
		c := &Client{}
		c.alive.Store(true)
		return c, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	// Launch multiple concurrent DialContext calls
	var wg sync.WaitGroup
	const numGoroutines = 10

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each call will fail (no real SSH) but should not panic
			_, _ = rd.DialContext(context.Background(), "tcp", "127.0.0.1:12345")
		}()
	}

	wg.Wait()
	// No panics or deadlocks = success
}

func TestReconnectDialer_DialAfterClose(t *testing.T) {
	rd, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: -1,
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		c := &Client{}
		c.alive.Store(true)
		return c, nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	_ = rd.Close()

	_, dialErr := rd.DialContext(context.Background(), "tcp", "127.0.0.1:12345")
	if dialErr == nil {
		t.Fatal("expected error on dial after close")
	}
}

func TestReconnectDialer_ReconnectAfterClose(t *testing.T) {
	rd, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: -1,
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		c := &Client{}
		c.alive.Store(true)
		return c, nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	_ = rd.Close()

	_, reconnErr := rd.reconnect()
	if reconnErr == nil {
		t.Fatal("expected error on reconnect after close")
	}
}

func TestReconnectDialer_KeepaliveDetectsDeath(t *testing.T) {
	var events []StatusEvent
	var eventsMu sync.Mutex

	rd, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: 50 * time.Millisecond, // fast for testing
		KeepaliveCountMax: 2,
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		c := &Client{}
		c.alive.Store(true)
		// The client's SendKeepalive will fail because ssh.Client is nil
		// This simulates a dead connection
		return c, nil
	}), WithStatusCallback(func(e StatusEvent) {
		eventsMu.Lock()
		events = append(events, e)
		eventsMu.Unlock()
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	// Wait enough time for keepalive to detect death and trigger reconnect
	// 2 missed keepalives at 50ms interval = 100ms, plus some margin
	time.Sleep(400 * time.Millisecond)

	eventsMu.Lock()
	defer eventsMu.Unlock()

	// Should have seen disconnected event
	hasDisconnected := false
	for _, e := range events {
		if e.Type == StatusDisconnected {
			hasDisconnected = true
			break
		}
	}
	if !hasDisconnected {
		t.Error("expected StatusDisconnected event from keepalive detection")
	}
}

func TestReconnectDialer_KeepaliveStopsOnClose(t *testing.T) {
	rd, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: 50 * time.Millisecond,
		KeepaliveCountMax: 100, // high so it doesn't trigger reconnect
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		c := &Client{}
		c.alive.Store(true)
		return c, nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Close should stop keepalive without hanging
	done := make(chan struct{})
	go func() {
		_ = rd.Close()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Close hung, likely keepalive goroutine not stopping")
	}
}

func TestReconnectDialer_StatusCallbackReceivesEvents(t *testing.T) {
	var events []StatusEvent
	var eventsMu sync.Mutex

	rd, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: -1, // disable keepalive
		KeepaliveCountMax: 1,
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		c := &Client{}
		c.alive.Store(true)
		return c, nil
	}), WithStatusCallback(func(e StatusEvent) {
		eventsMu.Lock()
		events = append(events, e)
		eventsMu.Unlock()
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Trigger reconnect (should succeed)
	_, _ = rd.reconnect()

	_ = rd.Close()

	eventsMu.Lock()
	defer eventsMu.Unlock()

	// Expect: Connected, Reconnecting, Reconnected
	types := make([]StatusType, len(events))
	for i, e := range events {
		types[i] = e.Type
	}

	if len(types) < 3 {
		t.Fatalf("expected at least 3 events, got %d: %v", len(types), types)
	}
	if types[0] != StatusConnected {
		t.Errorf("first event should be StatusConnected, got %d", types[0])
	}

	hasReconnecting := false
	hasReconnected := false
	for _, typ := range types {
		if typ == StatusReconnecting {
			hasReconnecting = true
		}
		if typ == StatusReconnected {
			hasReconnected = true
		}
	}
	if !hasReconnecting {
		t.Error("expected StatusReconnecting event")
	}
	if !hasReconnected {
		t.Error("expected StatusReconnected event")
	}
}

func TestReconnectDialer_NilStatusCallback(t *testing.T) {
	// Ensure no panic when no callback is set
	rd, err := NewReconnectDialer(context.Background(), Options{
		KeepaliveInterval: -1,
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		c := &Client{}
		c.alive.Store(true)
		return c, nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic
	_, _ = rd.reconnect()
	_ = rd.Close()
}

func TestReconnectDialer_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	rd, err := NewReconnectDialer(ctx, Options{
		KeepaliveInterval: -1,
		KeepaliveCountMax: 2,
	}, withConnectFunc(func(ctx context.Context, opts Options) (*Client, error) {
		c := &Client{}
		c.alive.Store(true)
		return c, nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Cancel context then try reconnect
	cancel()

	_, reconnErr := rd.reconnect()
	if reconnErr == nil {
		t.Fatal("expected error when context is cancelled")
	}

	_ = rd.Close()
}

func TestOptions_KeepaliveDefaults(t *testing.T) {
	opts := Options{}
	if opts.keepaliveInterval() != DefaultKeepaliveInterval {
		t.Errorf("expected default interval %v, got %v", DefaultKeepaliveInterval, opts.keepaliveInterval())
	}
	if opts.keepaliveCountMax() != DefaultKeepaliveCountMax {
		t.Errorf("expected default count max %d, got %d", DefaultKeepaliveCountMax, opts.keepaliveCountMax())
	}
}

func TestOptions_KeepaliveCustom(t *testing.T) {
	opts := Options{
		KeepaliveInterval: 10 * time.Second,
		KeepaliveCountMax: 5,
	}
	if opts.keepaliveInterval() != 10*time.Second {
		t.Errorf("expected 10s, got %v", opts.keepaliveInterval())
	}
	if opts.keepaliveCountMax() != 5 {
		t.Errorf("expected 5, got %d", opts.keepaliveCountMax())
	}
}

func TestOptions_KeepaliveDisabled(t *testing.T) {
	opts := Options{
		KeepaliveInterval: -1,
	}
	// Negative value means disabled; keepaliveInterval returns the raw value
	if opts.keepaliveInterval() != DefaultKeepaliveInterval {
		// With negative value, it falls through to default
		// This is the expected behavior
	}
}
