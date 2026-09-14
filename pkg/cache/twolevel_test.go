package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 以下测试均走 L1-only 路径（rdb=nil）

func TestTwoLevelGetSet(t *testing.T) {
	c := NewTwoLevel(nil, "t:", time.Minute, time.Minute, false)
	ctx := context.Background()

	if _, ok := c.Get(ctx, "k"); ok {
		t.Fatal("expected miss on empty cache")
	}
	c.Set(ctx, "k", "v")
	if v, ok := c.Get(ctx, "k"); !ok || v != "v" {
		t.Fatalf("expected hit v, got %q ok=%v", v, ok)
	}
	c.Del(ctx, "k")
	if _, ok := c.Get(ctx, "k"); ok {
		t.Fatal("expected miss after Del")
	}
}

func TestTwoLevelL1Expire(t *testing.T) {
	c := NewTwoLevel(nil, "t:", 30*time.Millisecond, time.Minute, false)
	ctx := context.Background()

	c.Set(ctx, "k", "v")
	if _, ok := c.Get(ctx, "k"); !ok {
		t.Fatal("expected hit before expiry")
	}
	time.Sleep(50 * time.Millisecond)
	if _, ok := c.Get(ctx, "k"); ok {
		t.Fatal("expected miss after L1 TTL")
	}
}

func TestTwoLevelLoadSingleflight(t *testing.T) {
	c := NewTwoLevel(nil, "t:", time.Minute, time.Minute, false)
	ctx := context.Background()

	var calls atomic.Int64
	loader := func(ctx context.Context) (string, error) {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond) // 放大并发窗口
		return "v", nil
	}

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			v, err := c.Load(ctx, "k", loader)
			if err != nil || v != "v" {
				t.Errorf("Load = %q, %v", v, err)
			}
		}()
	}
	wg.Wait()
	if got := calls.Load(); got != 1 {
		t.Fatalf("loader called %d times, want 1", got)
	}
}

func TestTwoLevelLoadErrorNotCached(t *testing.T) {
	c := NewTwoLevel(nil, "t:", time.Minute, time.Minute, false)
	ctx := context.Background()

	wantErr := errors.New("db down")
	if _, err := c.Load(ctx, "k", func(ctx context.Context) (string, error) {
		return "", wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("Load err = %v, want %v", err, wantErr)
	}
	// loader 出错不回填，下一次仍可回源成功
	v, err := c.Load(ctx, "k", func(ctx context.Context) (string, error) {
		return "v", nil
	})
	if err != nil || v != "v" {
		t.Fatalf("Load = %q, %v", v, err)
	}
}

func TestTwoLevelL1Cap(t *testing.T) {
	c := NewTwoLevel(nil, "t:", time.Hour, time.Minute, false)
	ctx := context.Background()

	for i := 0; i < l1MaxEntries+500; i++ {
		c.Set(ctx, fmt.Sprintf("k%d", i), "v")
	}
	c.mu.RLock()
	got := len(c.l1)
	c.mu.RUnlock()
	if got > l1MaxEntries {
		t.Fatalf("l1 size %d exceeds cap %d", got, l1MaxEntries)
	}
}

func TestTwoLevelFlushClearsL1(t *testing.T) {
	c := NewTwoLevel(nil, "t:", time.Minute, time.Minute, false)
	ctx := context.Background()

	c.Set(ctx, "k", "v")
	c.Flush(ctx)
	if _, ok := c.Get(ctx, "k"); ok {
		t.Fatal("expected miss after Flush")
	}
}
