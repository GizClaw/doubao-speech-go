package doubaospeech

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type blockedASRConn struct {
	*fakeWSConn
	blockAt int32
	writes  atomic.Int32
	entered chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func newBlockedASRConn(blockAt int32) *blockedASRConn {
	return &blockedASRConn{fakeWSConn: newFakeWSConn(), blockAt: blockAt, entered: make(chan struct{}), closed: make(chan struct{})}
}

func (c *blockedASRConn) WriteMessage(kind int, data []byte) error {
	if c.writes.Add(1) == c.blockAt {
		close(c.entered)
		<-c.closed
		return io.ErrClosedPipe
	}
	return c.fakeWSConn.WriteMessage(kind, data)
}

func (c *blockedASRConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.fakeWSConn.Close()
}

func awaitBlockedASR[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(time.Second):
		t.Fatal("ASR cancellation did not unblock transport")
		var zero T
		return zero
	}
}

func TestASRBlockedStartHonorsCancellation(t *testing.T) {
	conn := newBlockedASRConn(1)
	client := NewClient("fixture")
	client.ASRV2.dialer = &fakeDialer{conn: conn}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		session, err := client.ASRV2.OpenStreamSession(ctx, &ASRV2Config{Format: FormatPCM, SampleRate: SampleRate16000})
		if session != nil {
			_ = session.Close()
		}
		done <- err
	}()
	awaitBlockedASR(t, conn.entered)
	cancel()
	if err := awaitBlockedASR(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("start error=%v", err)
	}
	awaitBlockedASR(t, conn.closed)
}

func TestASRBlockedSendHonorsCancellationAndClose(t *testing.T) {
	for _, action := range []string{"cancel", "close"} {
		t.Run(action, func(t *testing.T) {
			conn := newBlockedASRConn(2)
			client := NewClient("fixture")
			client.ASRV2.dialer = &fakeDialer{conn: conn}
			session, err := client.ASRV2.OpenStreamSession(t.Context(), &ASRV2Config{Format: FormatPCM, SampleRate: SampleRate16000})
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- session.SendAudio(ctx, []byte{1, 2}, false) }()
			awaitBlockedASR(t, conn.entered)
			if action == "cancel" {
				cancel()
			} else {
				closed := make(chan error, 1)
				go func() { closed <- session.Close() }()
				if err := awaitBlockedASR(t, closed); err != nil {
					t.Fatal(err)
				}
			}
			err = awaitBlockedASR(t, done)
			if err == nil || (action == "cancel" && !errors.Is(err, context.Canceled)) {
				t.Fatalf("send error=%v", err)
			}
			awaitBlockedASR(t, conn.closed)
			if !session.isClosed() {
				t.Fatal("canceled transport left the session reusable")
			}
			if err := session.SendAudio(t.Context(), []byte{3, 4}, false); err == nil {
				t.Fatal("fresh send reused the invalid session")
			}
			if writes := conn.writes.Load(); writes != 2 {
				t.Fatalf("fresh send reached closed transport: writes=%d", writes)
			}
		})
	}
}
