package jsQuickJs

// TaskQueue allows executing the C++ calls from only one thread.
// Without that, Go can be short on available threads which lead to a crash.
//
// This protection is only required where there is a lot of calls that can be blocked the thread.
// It's essentially when calling an event and calling a callback function.
type TaskQueue struct {
	channel  chan func()
	disposed bool
}

func newTaskQueue() *TaskQueue {
	res := &TaskQueue{
		// The size avoid blocking.
		// Once exceeded, the system is locked and unexpected behaviors can occur.
		// It's why me make it big here.
		//
		channel: make(chan func(), 1024),
	}

	SafeGoRoutine(func() { res.start() })
	return res
}

func (m *TaskQueue) Push(f func()) {
	if m.disposed {
		return
	}

	m.channel <- f
}

func (m *TaskQueue) IsDisposed() bool {
	return m.disposed
}

func (m *TaskQueue) Dispose() {
	if m.disposed {
		return
	}

	m.disposed = true
	close(m.channel)
}

func (m *TaskQueue) start() {
	for {
		next := <-m.channel

		if m.disposed {
			break
		}

		next()
	}
}
