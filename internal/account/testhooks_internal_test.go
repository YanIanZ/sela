package account

import "sync"

// HoldNextVerify stops the next Verify of a stored hash after its argon2id
// comparison has run and before it returns, until release is called;
// reached is closed when it stops there. Verifications after it run
// through, and release may be called more than once (an unused hold is
// removed). Tests use it to commit a password change or a recovery while a
// proof sits between its verification and its success transaction.
func (h *Hasher) HoldNextVerify() (reached <-chan struct{}, release func()) {
	r, gate := make(chan struct{}), make(chan struct{})
	hook := func() {
		close(r)
		<-gate
	}
	h.afterVerify.Store(&hook)
	var once sync.Once
	return r, func() {
		once.Do(func() {
			h.afterVerify.CompareAndSwap(&hook, nil)
			close(gate)
		})
	}
}
