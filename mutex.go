package contest

type Mutex interface {
	Lock()

	// LockChannel возвращает канал блокировки -
	// вычитывание из канала приводит к блокировке мьютекса
	LockChannel() <-chan struct{}

	Unlock()
}
