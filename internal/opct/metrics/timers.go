package metrics

import "time"

// Timer is a struct used internally to handle execution markers,
// used to calculate the total execution time for some parsers/checkpoints,
// over the report flow.
type Timer struct {
	start time.Time

	// Total is a calculation of elapsed time from start timestamp.
	Total float64 `json:"seconds"`
}

// Timers is a struct used internally to handle execution markers,
// used to check the total execution time for some parsers.
type Timers struct {
	Timers map[string]*Timer `json:"Timers,omitempty"`
	last   string
}

func NewTimers() *Timers {
	ts := Timers{Timers: make(map[string]*Timer)}
	return &ts
}

// set is a method to persist a timer, updating if exists.
// The current timestamp will be used when a new item is created.
func (ts *Timers) set(k string) {
	if _, ok := ts.Timers[k]; !ok {
		ts.Timers[k] = &Timer{start: time.Now()}
	} else {
		stop := time.Now()
		ts.Timers[k].Total = stop.Sub(ts.Timers[k].start).Seconds()
	}
}

// Set method is an external interface to create/update a timer.
// Interface for start, stop and add a new one (lap).
func (ts *Timers) Set(k string) {
	if ts.last != "" {
		ts.set(ts.last)
	}
	ts.set(k)
	ts.last = k
}

// Add method creates a new timer metric.
func (ts *Timers) Add(k string) {
	ts.set(k)
}
