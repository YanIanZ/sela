package llm_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/llm"
)

func TestAdmissionBoundsConcurrency(t *testing.T) {
	const slots, workers = 3, 24
	a := llm.NewAdmission(slots)
	if a.Cap() != slots || a.InUse() != 0 {
		t.Fatalf("cap %d in use %d", a.Cap(), a.InUse())
	}
	var active, peak atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := a.Acquire(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			defer release()
			n := active.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			if in := a.InUse(); in > slots {
				t.Errorf("InUse %d > %d", in, slots)
			}
			time.Sleep(2 * time.Millisecond)
			active.Add(-1)
		}()
	}
	wg.Wait()
	if peak.Load() > slots || peak.Load() == 0 {
		t.Fatalf("peak concurrency %d, want 1..%d", peak.Load(), slots)
	}
	if a.InUse() != 0 {
		t.Fatalf("InUse %d after all releases", a.InUse())
	}
}

func TestAdmissionContextCancellation(t *testing.T) {
	a := llm.NewAdmission(1)
	release, err := a.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r2, err := a.Acquire(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || r2 != nil {
		t.Fatalf("full admission: release %v err %v", r2 != nil, err)
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := a.Acquire(ctx2)
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel2()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Acquire ignored cancellation")
	}
	if a.InUse() != 1 {
		t.Fatalf("a canceled waiter took a slot: InUse %d", a.InUse())
	}

	// A waiter gets the slot once it is released.
	got := make(chan func(), 1)
	go func() {
		r, err := a.Acquire(context.Background())
		if err != nil {
			t.Error(err)
		}
		got <- r
	}()
	time.Sleep(20 * time.Millisecond)
	release()
	select {
	case r := <-got:
		r()
	case <-time.After(2 * time.Second):
		t.Fatal("waiter not admitted after release")
	}

	// An already-canceled context never takes a free slot.
	canceled, c := context.WithCancel(context.Background())
	c()
	if r, err := a.Acquire(canceled); err == nil || r != nil || a.InUse() != 0 {
		t.Fatalf("canceled context admitted: err %v in use %d", err, a.InUse())
	}
}

func TestAdmissionReleaseIsIdempotent(t *testing.T) {
	a := llm.NewAdmission(2)
	r1, _ := a.Acquire(context.Background())
	r2, _ := a.Acquire(context.Background())
	r1()
	r1() // must not free r2's slot
	if a.InUse() != 1 {
		t.Fatalf("InUse %d after double release, want 1", a.InUse())
	}
	r2()
	if a.InUse() != 0 {
		t.Fatalf("InUse %d", a.InUse())
	}
}

func TestAdmissionMinimumOneSlot(t *testing.T) {
	for _, n := range []int{0, -3} {
		a := llm.NewAdmission(n)
		if a.Cap() != 1 {
			t.Fatalf("NewAdmission(%d).Cap() = %d", n, a.Cap())
		}
		r, err := a.Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		r()
	}
}
