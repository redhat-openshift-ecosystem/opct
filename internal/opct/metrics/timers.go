package metrics

import "time"

type Timers struct {
	Timers map[string]*Timer `json:"Timers,omitempty"`
	last   string
}

func NewTimers() Timers {
	ts := Timers{Timers: make(map[string]*Timer)}
	return ts
}

// set a timer, updating if existing.
func (ts *Timers) set(k string) {
	if _, ok := ts.Timers[k]; !ok {
		ts.Timers[k] = &Timer{start: time.Now()}
	} else {
		stop := time.Now()
		ts.Timers[k].Total = stop.Sub(ts.Timers[k].start).Seconds()
	}
}

// Set check last timer, stop and add a new one (lap).
func (ts *Timers) Set(k string) {
	if ts.last != "" {
		ts.set(ts.last)
	}
	ts.set(k)
	ts.last = k
}

// Add a new timer.
func (ts *Timers) Add(k string) {
	ts.set(k)
}

type Timer struct {
	start time.Time

	// Total time in milisseconds
	Total float64 `json:"seconds"`
}
