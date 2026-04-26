package clock

import (
	"container/heap"
	"sync"
	"time"
)

// --- Scheduler entry types ---

type entryKind int

const (
	kindTicker    entryKind = iota // repeating ticker
	kindAfterFunc                  // one-shot callback
	kindAfter                      // one-shot channel send (After)
)

// tickerState holds shared mutable state for a ticker across all its
// re-inserted heap entries. Using a shared pointer means Stop() correctly
// prevents all future firings even after the current heap entry was replaced
// by re-insertion.
type tickerState struct {
	stopped bool
	ch      chan time.Time
	period  time.Duration
}

// schedEntry is a single scheduled event managed by the min-heap.
type schedEntry struct {
	// deadline is the wall-clock time (relative to TestClock.now) at which this
	// entry fires.
	deadline time.Time

	// id is a monotonically increasing counter used to break deadline ties in
	// FIFO order.
	id uint64

	// kind distinguishes tickers, AfterFunc callbacks, and After channels.
	kind entryKind

	// dead marks an entry as cancelled (tombstone). Dead entries are skipped
	// when popped from the heap.
	dead bool

	// index is the heap index — maintained by the heap.Interface methods.
	index int

	// ticker is set for kindTicker entries; the pointer is shared across all
	// re-insertions of the same logical ticker.
	ticker *tickerState

	// --- AfterFunc / After fields ---
	fn     func()         // AfterFunc callback (nil for kindAfter)
	afterC chan time.Time // After result channel (nil for kindAfterFunc)
}

// schedHeap is a min-heap of *schedEntry ordered by (deadline, id).
type schedHeap []*schedEntry

func (h schedHeap) Len() int { return len(h) }

func (h schedHeap) Less(i, j int) bool {
	if h[i].deadline.Equal(h[j].deadline) {
		return h[i].id < h[j].id
	}
	return h[i].deadline.Before(h[j].deadline)
}

func (h schedHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *schedHeap) Push(x any) {
	e := x.(*schedEntry)
	e.index = len(*h)
	*h = append(*h, e)
}

func (h *schedHeap) Pop() any {
	old := *h
	n := len(old)
	e := old[n-1]
	old[n-1] = nil // avoid memory leak
	*h = old[:n-1]
	e.index = -1
	return e
}

// --- testTicker / testTimer ---

// testTicker is the Ticker returned by TestClock.NewTicker.
// It holds a reference to the shared tickerState so Stop marks all future
// re-insertions as cancelled.
type testTicker struct {
	ts    *tickerState
	sched *scheduler
}

func (tt *testTicker) C() <-chan time.Time { return tt.ts.ch }

func (tt *testTicker) Stop() {
	tt.sched.stopTicker(tt.ts)
}

// testTimer is the Timer returned by TestClock.AfterFunc and used internally
// by TestClock.After.
//
// mu guards entryRef so Stop/Reset are safe to call concurrently.
type testTimer struct {
	mu       sync.Mutex
	entryRef **schedEntry // indirect pointer so resetTimer can swap the entry
	sched    *scheduler
}

func (tm *testTimer) Stop() bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.sched.cancel(*tm.entryRef)
}

func (tm *testTimer) Reset(d time.Duration) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	newEntry, wasActive := tm.sched.resetTimer(*tm.entryRef, d)
	*tm.entryRef = newEntry
	return wasActive
}

// --- scheduler ---

// scheduler is the min-heap-based event scheduler embedded in TestClock.
type scheduler struct {
	mu      sync.Mutex
	now     *time.Time // pointer to TestClock.now — read under scheduler.mu
	nextID  uint64
	entries schedHeap
}

func newScheduler(now *time.Time) *scheduler {
	s := &scheduler{now: now}
	heap.Init(&s.entries)
	return s
}

func (s *scheduler) allocID() uint64 {
	s.nextID++
	return s.nextID
}

// addTicker registers a new repeating ticker entry and returns a handle to its
// shared tickerState. Caller must hold s.mu.
func (s *scheduler) addTicker(period time.Duration) *tickerState {
	ts := &tickerState{
		ch:     make(chan time.Time, 1),
		period: period,
	}
	e := &schedEntry{
		deadline: s.now.Add(period),
		id:       s.allocID(),
		kind:     kindTicker,
		ticker:   ts,
	}
	heap.Push(&s.entries, e)
	return ts
}

// addAfterFunc registers a one-shot callback entry and returns it.
// Caller must hold s.mu.
func (s *scheduler) addAfterFunc(d time.Duration, f func()) *schedEntry {
	e := &schedEntry{
		deadline: s.now.Add(d),
		id:       s.allocID(),
		kind:     kindAfterFunc,
		fn:       f,
	}
	heap.Push(&s.entries, e)
	return e
}

// addAfter registers a one-shot channel-send entry and returns it.
// Caller must hold s.mu.
func (s *scheduler) addAfter(d time.Duration) *schedEntry {
	e := &schedEntry{
		deadline: s.now.Add(d),
		id:       s.allocID(),
		kind:     kindAfter,
		afterC:   make(chan time.Time, 1),
	}
	heap.Push(&s.entries, e)
	return e
}

// stopTicker marks a tickerState as stopped. All heap entries for this ticker
// (including any re-inserted ones) check this flag before firing.
func (s *scheduler) stopTicker(ts *tickerState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts.stopped = true
}

// remove marks an entry as dead (tombstone). The entry will be skipped when
// popped during Advance.
func (s *scheduler) remove(e *schedEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.dead = true
}

// cancel marks a timer entry as dead and returns true if the timer had not yet
// fired. Returns false if the entry was already dead.
func (s *scheduler) cancel(e *schedEntry) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.dead {
		return false
	}
	e.dead = true
	return true
}

// resetTimer tombstones e and adds a fresh entry at now+d. It returns the new
// entry and whether the old entry was still alive (i.e., had not yet fired).
// The caller is responsible for updating any handle that references e.
//
// Only kindAfter and kindAfterFunc entries may be reset; kindTicker entries
// use stopTicker instead.
func (s *scheduler) resetTimer(e *schedEntry, d time.Duration) (*schedEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wasActive := !e.dead
	e.dead = true // tombstone the old entry

	// Create a new entry re-using the same channel (kindAfter) or callback
	// (kindAfterFunc) from the original entry.
	fresh := &schedEntry{
		deadline: s.now.Add(d),
		id:       s.allocID(),
		kind:     e.kind,
		fn:       e.fn,
		afterC:   e.afterC,
	}
	heap.Push(&s.entries, fresh)
	return fresh, wasActive
}

// advance fires all entries whose deadline <= newNow, in deadline order.
// It updates *s.now to newNow AFTER draining the heap, and releases the mutex
// before sending on channels or launching goroutines (to avoid deadlocks when
// a goroutine calls Stop/Reset).
//
// Returns a slice of entries to fire, in deadline order, after releasing the
// mutex.
func (s *scheduler) advance(d time.Duration) {
	s.mu.Lock()
	newNow := s.now.Add(d)

	// Collect entries to fire in deadline order.
	type toFire struct {
		entry    *schedEntry
		fireTime time.Time
	}
	var fires []toFire

	for len(s.entries) > 0 {
		top := s.entries[0]
		if top.deadline.After(newNow) {
			break
		}
		heap.Pop(&s.entries)
		if top.dead {
			continue
		}
		if top.kind == kindTicker && top.ticker.stopped {
			continue
		}
		fires = append(fires, toFire{entry: top, fireTime: top.deadline})

		// Re-insert tickers immediately so subsequent Advance calls see them.
		if top.kind == kindTicker {
			next := &schedEntry{
				deadline: top.deadline.Add(top.ticker.period),
				id:       s.allocID(),
				kind:     kindTicker,
				ticker:   top.ticker,
			}
			heap.Push(&s.entries, next)
		}
	}

	*s.now = newNow
	s.mu.Unlock()

	// Fire outside the lock.
	for _, f := range fires {
		switch f.entry.kind {
		case kindTicker:
			// Non-blocking send: drop on full buffer (matches stdlib).
			select {
			case f.entry.ticker.ch <- f.fireTime:
			default:
			}
		case kindAfterFunc:
			fn := f.entry.fn
			go fn()
		case kindAfter:
			// Non-blocking send: the channel is buffered(1) so this should
			// always succeed unless Reset re-used the channel and it was already
			// read before a second fire.
			select {
			case f.entry.afterC <- f.fireTime:
			default:
			}
		}
	}
}
