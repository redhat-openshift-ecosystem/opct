package mustgather

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/montanaflynn/stats"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/archive"
	log "github.com/sirupsen/logrus"
)

// ErrorEtcdLogs handle errors extracted/parsed from etcd pod logs.
type ErrorEtcdLogs struct {
	ErrorCounters         archive.ErrorCounter
	FilterRequestSlowAll  map[string]*BucketFilterStat
	FilterRequestSlowHour map[string]*BucketFilterStat
	Buffer                []*string `json:"-"`
}

// common errors to create counters
var commonTestErrorPatternEtcdLogs = []string{
	`rejected connection`,
	`waiting for ReadIndex response took too long, retrying`,
	`failed to find remote peer in cluster`,
	`dropped Raft message since sending buffer is full (overloaded network)`,
	`request stats`,
	`apply request took too long`,
	`failed to lock file`,
}

func NewErrorEtcdLogs(buf *string) *ErrorEtcdLogs {
	etcdLogs := &ErrorEtcdLogs{}

	// create counters
	etcdLogs.ErrorCounters = archive.NewErrorCounter(buf, commonTestErrorPatternEtcdLogs)

	// filter Slow Requests (aggregate by hour)
	filterATTL1 := NewFilterApplyTookTooLong("hour")
	for _, line := range strings.Split(*buf, "\n") {
		errLogLine := filterATTL1.ProcessLine(line)
		if errLogLine != nil {
			etcdLogs.Buffer = append(etcdLogs.Buffer, errLogLine)
		}
	}
	// Check only the last 3 hours (average time of an opct execution)
	etcdLogs.FilterRequestSlowHour = filterATTL1.GetStat(3)

	// filter Slow Requests (aggregate all)
	filterATTL2 := NewFilterApplyTookTooLong("all")
	for _, line := range strings.Split(*buf, "\n") {
		filterATTL2.ProcessLine(line)
	}
	etcdLogs.FilterRequestSlowAll = filterATTL2.GetStat(1)

	return etcdLogs
}

// LogPayloadETCD parses the etcd log file to extract insights
// {"level":"warn","ts":"2023-03-01T15:14:22.192Z",
// "caller":"etcdserver/util.go:166",
// "msg":"apply request took too long",
// "took":"231.023586ms","expected-duration":"200ms",
// "prefix":"read-only range ",
// "request":"key:\"/kubernetes.io/configmaps/kube-system/kube-controller-manager\" ",
// "response":"range_response_count:1 size:608"}
type LogPayloadETCD struct {
	Took      string `json:"took"`
	Timestamp string `json:"ts"`
}

type BucketGroup struct {
	Bukets1s    Buckets
	Bukets500ms Buckets
}

type FilterApplyTookTooLong struct {
	Name    string
	GroupBy string
	Group   map[string]*BucketGroup

	// filter config
	lineFilter     string
	reLineSplitter *regexp.Regexp
	reMili         *regexp.Regexp
	reSec          *regexp.Regexp
	reTsMin        *regexp.Regexp
	reTsHour       *regexp.Regexp
	reTsDay        *regexp.Regexp
}

func NewFilterApplyTookTooLong(aggregator string) *FilterApplyTookTooLong {
	var filter FilterApplyTookTooLong

	filter.Name = "ApplyTookTooLong"
	filter.GroupBy = aggregator
	filter.Group = make(map[string]*BucketGroup)

	filter.lineFilter = "apply request took too long"
	filter.reLineSplitter, _ = regexp.Compile(`^\d+-\d+-\d+T\d+:\d+:\d+.\d+Z `)
	filter.reMili, _ = regexp.Compile("([0-9]+.[0-9]+)ms")
	filter.reSec, _ = regexp.Compile("([0-9]+.[0-9]+)s")
	filter.reTsMin, _ = regexp.Compile(`^(\d+-\d+-\d+T\d+:\d+):\d+.\d+Z`)
	filter.reTsHour, _ = regexp.Compile(`^(\d+-\d+-\d+T\d+):\d+:\d+.\d+Z`)
	filter.reTsDay, _ = regexp.Compile(`^(\d+-\d+-\d+)T\d+:\d+:\d+.\d+Z`)

	return &filter
}

func (f *FilterApplyTookTooLong) ProcessLine(line string) *string {

	// filter by required filter
	if !strings.Contains(line, f.lineFilter) {
		return nil
	}

	// split timestamp
	split := f.reLineSplitter.Split(line, -1)
	if len(split) < 1 {
		return nil
	}

	// parse json
	lineParsed := LogPayloadETCD{}
	if err := json.Unmarshal([]byte(split[1]), &lineParsed); err != nil {
		log.Errorf("couldn't parse json: %v", err)
	}

	if match := f.reMili.MatchString(lineParsed.Took); match { // Extract milisseconds
		matches := f.reMili.FindStringSubmatch(lineParsed.Took)
		if len(matches) == 2 {
			if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
				f.insertBucket(v, lineParsed.Timestamp)
			}
		}
	} else if match := f.reSec.MatchString(lineParsed.Took); match { // Extract seconds
		matches := f.reSec.FindStringSubmatch(lineParsed.Took)
		if len(matches) == 2 {
			if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
				v = v * 1000
				f.insertBucket(v, lineParsed.Timestamp)
			}
		}
	} else {
		fmt.Printf("No bucket for: %v\n", lineParsed.Took)
	}

	return &line
}

func (f *FilterApplyTookTooLong) insertBucket(v float64, ts string) {
	var group *BucketGroup
	var aggrKey string

	if f.GroupBy == "hour" {
		aggrValue := "all"
		if match := f.reTsHour.MatchString(ts); match {
			matches := f.reTsHour.FindStringSubmatch(ts)
			aggrValue = matches[1]
		}
		aggrKey = aggrValue
	} else if f.GroupBy == "day" {
		aggrValue := "all"
		if match := f.reTsDay.MatchString(ts); match {
			matches := f.reTsDay.FindStringSubmatch(ts)
			aggrValue = matches[1]
		}
		aggrKey = aggrValue
	} else if f.GroupBy == "minute" || f.GroupBy == "min" {
		aggrValue := "all"
		if match := f.reTsMin.MatchString(ts); match {
			matches := f.reTsMin.FindStringSubmatch(ts)
			aggrValue = matches[1]
		}
		aggrKey = aggrValue
	} else {
		aggrKey = f.GroupBy
	}

	if _, ok := f.Group[aggrKey]; !ok {
		f.Group[aggrKey] = &BucketGroup{}
		group = f.Group[aggrKey]
		group.Bukets1s = NewBuckets(buckets1s())
		group.Bukets500ms = NewBuckets(buckets500ms())
	} else {
		group = f.Group[aggrKey]
	}

	b1s := group.Bukets1s
	b500ms := group.Bukets500ms

	switch {
	case v < 200:
		log.Debugf("etcd log parser - got request slower than 200 (should not happen): %v", v)

	case ((v >= 200) && (v < 300)):
		k := "200-300"
		b1s[k] = append(b1s[k], v)
		b500ms[k] = append(b500ms[k], v)

	case ((v >= 300) && (v < 400)):
		k := "300-400"
		b1s[k] = append(b1s[k], v)
		b500ms[k] = append(b500ms[k], v)

	case ((v >= 400) && (v < 500)):
		k := "400-500"

		b1s[k] = append(b1s[k], v)
		b500ms[k] = append(b500ms[k], v)
	case ((v >= 500) && (v < 600)):
		k := "500-600"
		b1s[k] = append(b1s[k], v)

		k = "500-inf"
		b500ms[k] = append(b500ms[k], v)

	case ((v >= 600) && (v < 700)):
		k := "600-700"
		b1s[k] = append(b1s[k], v)

		k = "500-inf"
		b500ms[k] = append(b500ms[k], v)
	case ((v >= 700) && (v < 800)):
		k := "700-800"
		b1s[k] = append(b1s[k], v)

		k = "500-inf"
		b500ms[k] = append(b500ms[k], v)

	case ((v >= 800) && (v < 900)):
		k := "800-900"
		b1s[k] = append(b1s[k], v)

		k = "500-inf"
		b500ms[k] = append(b500ms[k], v)

	case ((v >= 900) && (v < 1000)):
		k := "900-999"
		b1s[k] = append(b1s[k], v)

		k = "500-inf"
		b500ms[k] = append(b500ms[k], v)

	case (v >= 1000):
		k := "1000-inf"
		b1s[k] = append(b1s[k], v)

		k = "500-inf"
		b500ms[k] = append(b500ms[k], v)

	default:
		k := "unkw"
		b1s[k] = append(b1s[k], v)
		b500ms[k] = append(b500ms[k], v)
	}
	k := "all"
	b1s[k] = append(b1s[k], v)
	b500ms[k] = append(b500ms[k], v)
}

type BucketFilterStat struct {
	RequestCount int64
	Higher500ms  string
	Buckets      map[string]string
	StatCount    string
	StatMin      string
	StatMedian   string
	StatMean     string
	StatMax      string
	StatSum      string
	StatStddev   string
	StatPerc90   string
	StatPerc99   string
	StatPerc999  string
	StatOutliers string
}

func (f *FilterApplyTookTooLong) GetStat(latest int) map[string]*BucketFilterStat {

	groups := make([]string, 0, len(f.Group))
	for k := range f.Group {
		groups = append(groups, k)
	}
	sort.Strings(groups)

	// filter latest group
	if latest > len(groups) {
		latest = len(groups)
	}
	latestGroups := groups[len(groups)-latest:]
	statGroups := make(map[string]*BucketFilterStat, latest)
	for _, gk := range latestGroups {
		group := f.Group[gk]
		statGroups[gk] = &BucketFilterStat{}

		b1s := group.Bukets1s
		b500ms := group.Bukets500ms

		getBucketStr := func(k string) string {
			countB1ms := len(b1s[k])
			countB1all := len(b1s["all"])
			perc := fmt.Sprintf("(%.3f%%)", (float64(countB1ms)/float64(countB1all))*100)
			if k == "all" {
				perc = ""
			}
			// return fmt.Sprintf("%8.8s %6s %11.10s", k, fmt.Sprintf("%d", countB1ms), perc)
			return fmt.Sprintf("%d %s", countB1ms, perc)
		}
		statGroups[gk].RequestCount = int64(len(b1s["all"]))

		v500 := len(b500ms["500-inf"])
		perc500inf := (float64(v500) / float64(len(b500ms["all"]))) * 100
		statGroups[gk].Higher500ms = fmt.Sprintf("%s (%.3f%%)", fmt.Sprintf("%d", v500), perc500inf)

		bukets := buckets1s()
		statGroups[gk].Buckets = make(map[string]string, len(bukets))
		for _, bkt := range bukets {
			statGroups[gk].Buckets[bkt] = getBucketStr(bkt)
		}

		min, _ := stats.Min(b1s["all"])
		max, _ := stats.Max(b1s["all"])
		sum, _ := stats.Sum(b1s["all"])
		mean, _ := stats.Mean(b1s["all"])
		median, _ := stats.Median(b1s["all"])
		p90, _ := stats.Percentile(b1s["all"], 90)
		p99, _ := stats.Percentile(b1s["all"], 99)
		p999, _ := stats.Percentile(b1s["all"], 99.9)
		stddev, _ := stats.StandardDeviationPopulation(b1s["all"])
		qoutliers, _ := stats.QuartileOutliers(b1s["all"])

		statGroups[gk].StatCount = fmt.Sprintf("%d", len(b1s["all"]))
		statGroups[gk].StatMin = fmt.Sprintf("%.3f (ms)", min)
		statGroups[gk].StatMedian = fmt.Sprintf("%.3f (ms)", median)
		statGroups[gk].StatMean = fmt.Sprintf("%.3f (ms)", mean)
		statGroups[gk].StatMax = fmt.Sprintf("%.3f (ms)", max)
		statGroups[gk].StatSum = fmt.Sprintf("%.3f (ms)", sum)
		statGroups[gk].StatStddev = fmt.Sprintf("%.3f", stddev)
		statGroups[gk].StatPerc90 = fmt.Sprintf("%.3f (ms)", p90)
		statGroups[gk].StatPerc99 = fmt.Sprintf("%.3f (ms)", p99)
		statGroups[gk].StatPerc999 = fmt.Sprintf("%.3f (ms)", p999)
		statGroups[gk].StatOutliers = fmt.Sprintf("%v", qoutliers)
	}
	return statGroups
}

func buckets1s() []string {
	return []string{
		"200-300",
		"300-400",
		"400-500",
		"500-600",
		"600-700",
		"700-800",
		"800-900",
		"900-999",
		"1000-inf",
		"all",
	}
}

func buckets500ms() []string {
	return []string{
		"200-300",
		"300-400",
		"400-500",
		"500-inf",
		"all",
	}
}

type Buckets map[string][]float64

func NewBuckets(values []string) Buckets {
	buckets := make(Buckets, len(values))
	for _, v := range values {
		buckets[v] = []float64{}
	}
	return buckets
}
