package mustgather

import (
	"bytes"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
	log "github.com/sirupsen/logrus"
)

var (
	// maxRateItemsToProcessQueue is the max number of items to process in parallel.
	defaultBufferLeakyBucket = 50
	// queueMaxSize is the max number of items to be queued in the bucket/memory before
	// unblocked by the rate limiter.
	defaultSizeLeakyBucket = 100
	// rateLimitIntervalMillisec lower values will increase the rate of processing,
	// but it will increase the risk of exhausting resources.
	defaultRateLimitIntervalLeakyBucket = 10 * time.Millisecond
)

func init() {
	// allow to override the rate limit to control the processing speed,
	// and consume less resources.
	overrideRateLimit := os.Getenv("OPCT_MUSTGATHER_RATELIMIT")
	if overrideRateLimit == "" {
		return
	}
	rate, err := strconv.Atoi(overrideRateLimit)
	if err != nil {
		log.Errorf("error parsing rate limit environment var OPCT_MUSTGATHER_RATELIMIT: %v", err)
		return
	}
	if rate <= 0 || rate > 100 {
		log.Errorf("invalid rate limit value, must be between 1 and 100: %d", rate)
		return
	}
	defaultRateLimitIntervalLeakyBucket = time.Duration(rate) * time.Millisecond
}

// MustGatherLog hold the must-gather findings in logs.
type MustGatherLog struct {
	Path          string
	PathAlias     string
	Namespace     string
	Pod           string
	Container     string
	ErrorCounters archive.ErrorCounter `json:"ErrorCounters,omitempty"`
	ErrorEtcdLogs *ErrorEtcdLogs       `json:"ErrorEtcdLogs,omitempty"`
	buffer        *bytes.Buffer        `json:"-"`
}

// Processed check if there are items processed, otherwise will save
// storage preventing items without relevant information.
func (mgl *MustGatherLog) Processed() bool {
	if len(mgl.ErrorCounters) > 0 {
		return true
	}
	if mgl.ErrorEtcdLogs != nil {
		return true
	}
	return false
}

// Leaky bucket implementation (queue limit) to parallel process must-gather items
// without exhausting resources. Increase the leakRate to process more items.
// The value of 10 (ms) is a ideal value, if want to decrease the CPU usage while
// processing the must-gather logs, increase the value to 100 (ms) by setting
// the environment variable OPCT_MUSTGATHER_RATELIMIT.
type leakyBucket struct {
	// bucketSize is the maximum number of items that can be stored in the bucket.
	bucketSize int
	// leakRate is the number of items that are removed from the bucket every second.
	leakRate time.Duration
	// bucket is the current number of items in the bucket.
	bucket int

	queue       chan *MustGatherLog
	queueCount  int
	rateLimiter chan struct{}
	semaphore   chan struct{}
	waiter      sync.WaitGroup
	locker      sync.Mutex

	// activeReading is a flag to indicate if the bucket is being read.
	activeReading bool

	// processor function to be called when the bucket is full.
	processor func(*MustGatherLog)
}

func newLeakyBucket(bucketSize int, leakRate time.Duration, fn func(*MustGatherLog)) *leakyBucket {
	lb := &leakyBucket{
		bucketSize:    bucketSize,
		leakRate:      leakRate,
		bucket:        0,
		queue:         make(chan *MustGatherLog, bucketSize),
		queueCount:    0,
		rateLimiter:   make(chan struct{}, defaultBufferLeakyBucket),
		semaphore:     make(chan struct{}, defaultBufferLeakyBucket),
		processor:     fn,
		activeReading: true,
	}

	for i := 0; i < cap(lb.rateLimiter); i++ {
		lb.rateLimiter <- struct{}{}
	}

	// leaky bucket ticker pausing the rate of processing every
	// rateLimitIntervalMillisec.
	go func() {
		log.Debug("Leaky bucket ticker - starting")
		ticker := time.NewTicker(lb.leakRate)
		defer ticker.Stop()
		for range ticker.C {
			_, ok := <-lb.rateLimiter
			// if this isn't going to run indefinitely, signal
			// this to return by closing the rate channel.
			if !ok {
				print("Leaky bucket rate limiter - closing")
				return
			}
		}
	}()

	// consume the queued pod logs to be processed/extracted information.
	go func() {
		log.Debug("Leaky bucket processor - starting")
		for data := range lb.queue {
			lb.processor(data)
			lb.decrement()
		}
	}()

	// monitor the queue size
	go func() {
		log.Debug("Leaky bucket monitor - starting")
		for lb.activeReading {
			log.Debugf("Must-gather processor - queue size monitor: %d", lb.queueCount)
			time.Sleep(10 * time.Second)
		}
	}()

	return lb
}

// decrement decrements the number of items in the queue.
func (lb *leakyBucket) decrement() {
	lb.waiter.Done()
	lb.locker.Lock()
	lb.queueCount -= 1
	lb.locker.Unlock()
}

// Incremet increments the number of items in the queue.
func (lb *leakyBucket) Incremet() {
	lb.waiter.Add(1)
	lb.locker.Lock()
	lb.queueCount += 1
	lb.locker.Unlock()
}

// AppendQueue checks the rate limiter and semaphore, then
// add a new item to the queue.
func (lb *leakyBucket) AppendQueue(mgl *MustGatherLog) {
	// wait for the rate limiter
	lb.rateLimiter <- struct{}{}

	// check the concurrency semaphore
	lb.semaphore <- struct{}{}
	defer func() {
		<-lb.semaphore
	}()

	// Sending the item to the queue
	lb.queue <- mgl
}
