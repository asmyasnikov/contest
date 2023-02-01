# contest

## Задание для контеста Мастерской для go-разработчиков

Написать реализацию интерфейса `Mutex`
```go
type Mutex interface {
	Lock()
	Unlock()
	LockChannel() <-chan struct{}
}
```
в файле `mutex_impl.go`. В этом файле уже содержится конструктор 
```go
func New() Mutex {
	// TODO
	return ... 
}
```
с пустым телом. Нужно отдать в конструкторе реализацию мьютекса `Mutex`.

## Типичное использование этого мьютекса

1. Как обычный мьютекс
```go
mu := contest.New()
mu.Lock()
// doing under lock
mu.Unlock()
```

2. Как `LockChannel` мьютекс
```go
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
select {
case <-ctx.Done():
	// nop
case <-mu.LockChannel():
	// doing under lock
	mu.Unlock()
}
```
