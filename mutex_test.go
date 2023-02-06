package contest_test

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"contest"
)

func HammerMutex(m contest.Mutex, loops int, cdone chan bool) {
	for i := 0; i < loops; i++ {
		if i%3 == 0 {
			select {
			case <-m.LockChannel():
				m.Unlock()
			default:
				// nop
			}
			continue
		}
		m.Lock()
		m.Unlock() //nolint:staticcheck
	}
	cdone <- true
}

func TestMutex(t *testing.T) {
	if n := runtime.SetMutexProfileFraction(1); n != 0 {
		t.Logf("got mutexrate %d expected 0", n)
	}
	defer runtime.SetMutexProfileFraction(0)

	m := contest.New()

	m.Lock()
	select {
	case <-m.LockChannel():
		t.Fatalf("LockChannel succeeded with mutex locked")
	default:
		// nop
	}
	m.Unlock()
	select {
	case <-m.LockChannel():
		// nop
	default:
		t.Fatalf("LockChannel failed with mutex unlocked")
	}
	m.Unlock()

	c := make(chan bool)
	for i := 0; i < 10; i++ {
		go HammerMutex(m, 1000, c)
	}
	for i := 0; i < 10; i++ {
		<-c
	}
}

var misuseTests = []struct {
	name string
	f    func()
}{
	{
		"Mutex.Unlock",
		func() {
			mu := contest.New()
			mu.Unlock()
		},
	},
	{
		"Mutex.Unlock2",
		func() {
			mu := contest.New()
			mu.Lock()
			mu.Unlock() //nolint:staticcheck
			mu.Unlock()
		},
	},
}

func init() {
	if len(os.Args) == 3 && os.Args[1] == "TESTMISUSE" {
		for _, test := range misuseTests {
			if test.name == os.Args[2] {
				func() {
					defer func() { recover() }() //nolint:errcheck
					test.f()
				}()
				fmt.Printf("test completed\n")
				os.Exit(0)
			}
		}
		fmt.Printf("unknown test\n")
		os.Exit(0)
	}
}

func TestMutexDeadlock(t *testing.T) {
	mu := contest.New()
	stop := make(chan bool)
	defer close(stop)
	lockedTwice := make(chan bool)
	go func() {
		mu.Lock()
		defer mu.Unlock()
		select {
		case <-mu.LockChannel():
			close(lockedTwice)
			return
		case <-stop:
			return
		}
	}()
	select {
	case <-lockedTwice:
		t.Fatalf("mutex locked twice")
	case <-time.After(10 * time.Second):
		// nop
	}
}

func TestMutexLockedChannelTwice(t *testing.T) {
	mu := contest.New()
	<-mu.LockChannel()
	lockedTwice := make(chan bool)
	go func() {
		<-mu.LockChannel()
		defer mu.Unlock()
		close(lockedTwice)
	}()
	select {
	case <-time.After(time.Second):
		mu.Unlock()
	case <-lockedTwice:
		t.Fatalf("mutex locked twice")
	}
}

func TestMutexLockInTheFuture(t *testing.T) {
	mu := contest.New()
	mu.Lock()
	var (
		lockedChannel = mu.LockChannel()
		simpleLock    = make(chan struct{})
		selectLock    = make(chan struct{})
		timeout       = make(chan struct{})
	)
	mu.Unlock()
	go func() {
		mu.Lock()
		defer mu.Unlock()
		close(simpleLock)
	}()
	go func() {
		select {
		case <-time.After(time.Second):
			close(timeout)
		case <-lockedChannel:
			close(selectLock)
			mu.Unlock()
		}
	}()
	select {
	case <-time.After(time.Second):
		t.Fatalf("can't lock simple")
	case <-simpleLock:
		t.Logf("expected behaviour")
	}
	select {
	case <-timeout:
		t.Fatalf("can't lock by select")
	case <-selectLock:
		t.Logf("mutex locked by select")
	}
}

func TestMutexFairness(t *testing.T) {
	mu := contest.New()
	stop := make(chan bool)
	defer close(stop)
	go func() {
		for {
			mu.Lock()
			time.Sleep(100 * time.Microsecond)
			mu.Unlock()
			select {
			case <-stop:
				return
			default:
			}
		}
	}()
	done := make(chan bool, 1)
	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(100 * time.Microsecond)
			mu.Lock()
			mu.Unlock() //nolint:staticcheck
		}
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("can't acquire Mutex in 10 seconds")
	}
}

func BenchmarkMutexUncontended(b *testing.B) {
	type PaddedMutex struct {
		contest.Mutex
	}
	b.RunParallel(func(pb *testing.PB) {
		mu := PaddedMutex{
			Mutex: contest.New(),
		}
		for pb.Next() {
			mu.Lock()
			mu.Unlock() //nolint:staticcheck
		}
	})
}

func benchmarkMutex(b *testing.B, slack, work bool) {
	mu := contest.New()
	if slack {
		b.SetParallelism(10)
	}
	b.RunParallel(func(pb *testing.PB) {
		foo := 0
		for pb.Next() {
			mu.Lock()
			mu.Unlock() //nolint:staticcheck
			if work {
				for i := 0; i < 100; i++ {
					foo *= 2
					foo /= 2
				}
			}
		}
		_ = foo
	})
}

func BenchmarkMutex(b *testing.B) {
	benchmarkMutex(b, false, false)
}

func BenchmarkMutexSlack(b *testing.B) {
	benchmarkMutex(b, true, false)
}

func BenchmarkMutexWork(b *testing.B) {
	benchmarkMutex(b, false, true)
}

func BenchmarkMutexWorkSlack(b *testing.B) {
	benchmarkMutex(b, true, true)
}

func BenchmarkMutexNoSpin(b *testing.B) {
	// This benchmark models a situation where spinning in the mutex should be
	// non-profitable and allows to confirm that spinning does not do harm.
	// To achieve this we create excess of goroutines most of which do local work.
	// These goroutines yield during local work, so that switching from
	// a blocked goroutine to other goroutines is profitable.
	// As a matter of fact, this benchmark still triggers some spinning in the mutex.
	m := contest.New()
	var acc0, acc1 uint64
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		c := make(chan bool)
		var data [4 << 10]uint64
		for i := 0; pb.Next(); i++ {
			if i%4 == 0 {
				m.Lock()
				acc0 -= 100
				acc1 += 100
				m.Unlock()
			} else {
				for i := 0; i < len(data); i += 4 {
					data[i]++
				}
				// Elaborate way to say runtime.Gosched
				// that does not put the goroutine onto global runq.
				go func() {
					c <- true
				}()
				<-c
			}
		}
	})
}

func BenchmarkMutexSpin(b *testing.B) {
	// This benchmark models a situation where spinning in the mutex should be
	// profitable. To achieve this we create a goroutine per-proc.
	// These goroutines access considerable amount of local data so that
	// unnecessary rescheduling is penalized by cache misses.
	m := contest.New()
	var acc0, acc1 uint64
	b.RunParallel(func(pb *testing.PB) {
		var data [16 << 10]uint64
		for i := 0; pb.Next(); i++ {
			m.Lock()
			acc0 -= 100
			acc1 += 100
			m.Unlock()
			for i := 0; i < len(data); i += 4 {
				data[i]++
			}
		}
	})
}
