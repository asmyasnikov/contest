package contest

type Mutex interface {
	Lock()
	LockChannel() <-chan struct{}

	Unlock()
}
