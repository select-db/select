package debounce

import (
	"sync"
	"time"
)

type Debouncer struct {
	timeout  time.Duration
	timer    *time.Timer
	callback func()
	mutex    sync.Mutex
}

func NewDebounce(timeout time.Duration, callback func()) Debouncer {
	return Debouncer{
		timeout:  timeout,
		callback: callback,
	}
}

func (m *Debouncer) Debounce() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.timer == nil {
		m.timer = time.AfterFunc(m.timeout, m.callback)

		return
	}

	m.timer.Stop()
	m.timer.Reset(m.timeout)
}

func (m *Debouncer) UpdateDebounceCallback(callback func()) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.timer.Stop()
	m.timer = time.AfterFunc(m.timeout, callback)
}
