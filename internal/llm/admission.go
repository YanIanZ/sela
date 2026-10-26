package llm

import (
	"context"
	"sync"
)

// Admission bounds concurrent model calls with a counting semaphore. It is
// safe for concurrent use.
type Admission struct{ sem chan struct{} }

// NewAdmission allows at most max concurrent calls (at least one).
func NewAdmission(max int) *Admission {
	if max < 1 {
		max = 1
	}
	return &Admission{sem: make(chan struct{}, max)}
}

// Acquire waits for a slot or the context; release must be called once.
// Calling release again is a no-op, so a duplicate release can never free a
// slot held by another caller. On error release is nil and err is ctx.Err().
func (a *Admission) Acquire(ctx context.Context) (release func(), err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case a.sem <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-a.sem }) }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// InUse reports occupied slots.
func (a *Admission) InUse() int { return len(a.sem) }

// Cap reports the maximum number of concurrent calls.
func (a *Admission) Cap() int { return cap(a.sem) }
