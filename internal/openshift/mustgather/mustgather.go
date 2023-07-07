package mustgather

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/archive"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/openshift/ci"
	log "github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
	"gopkg.in/yaml.v2"
	"k8s.io/utils/pointer"
)

/* MustGatehr raw files */
type MustGatherFile struct {
	Path      string
	PathAlias string `json:"PathAlias,omitempty"`
	Data      string `json:"Data,omitempty"`
}

type MustGather struct {
	// path to the directory must-gather will be saved.
	path string

	// ErrorEtcdLogs summary of etcd errors parsed from must-gather.
	ErrorEtcdLogs       *ErrorEtcdLogs `json:"ErrorEtcdLogs,omitempty"`
	ErrorEtcdLogsBuffer []*string      `json:"-"`

	// ErrorCounters summary error counters parsed from must-gather.
	ErrorCounters archive.ErrorCounter `json:"ErrorCounters,omitempty"`

	// NamespaceErrors hold pods reporting errors.
	NamespaceErrors []*MustGatherLog `json:"NamespaceErrors,omitempty"`
	namespaceCtrl   sync.Mutex

	// FileData hold raw data from files must-gather.
	RawFiles     []*MustGatherFile `json:"RawFiles,omitempty"`
	rawFilesCtrl sync.Mutex

	PodNetworkChecks MustGatherPodNetworkChecks
}

func NewMustGather(file string) *MustGather {
	return &MustGather{
		path: file,
	}
}

// InsertNamespaceErrors append the log data in safe way.
func (mg *MustGather) InsertNamespaceErrors(log *MustGatherLog) error {
	mg.namespaceCtrl.Lock()
	mg.NamespaceErrors = append(mg.NamespaceErrors, log)
	mg.namespaceCtrl.Unlock()
	return nil
}

// InsertRawFiles append the file data in safe way.
func (mg *MustGather) InsertRawFiles(file *MustGatherFile) error {
	mg.rawFilesCtrl.Lock()
	mg.RawFiles = append(mg.RawFiles, file)
	mg.rawFilesCtrl.Unlock()
	return nil
}

func (mg *MustGather) AggregateCounters() {
	if mg.ErrorCounters == nil {
		mg.ErrorCounters = make(archive.ErrorCounter, len(ci.CommonErrorPatterns))
	}
	if mg.ErrorEtcdLogs == nil {
		mg.ErrorEtcdLogs = &ErrorEtcdLogs{}
	}
	for nsi := range mg.NamespaceErrors {
		// calculate
		hasErrorCounters := false
		hasEtcdCounters := false
		if mg.NamespaceErrors[nsi].ErrorCounters != nil {
			hasErrorCounters = true
		}
		if mg.NamespaceErrors[nsi].ErrorEtcdLogs != nil {
			hasEtcdCounters = true
		}
		if mg.NamespaceErrors[nsi].ErrorEtcdLogs != nil {
			if mg.NamespaceErrors[nsi].Namespace == "openshift-etcd" &&
				mg.NamespaceErrors[nsi].Container == "etcd" &&
				strings.HasSuffix(mg.NamespaceErrors[nsi].Path, "current.log") {
				hasEtcdCounters = true
			}

		}
		// Error Counters
		if hasErrorCounters {
			for errName, errCounter := range mg.NamespaceErrors[nsi].ErrorCounters {
				if _, ok := mg.ErrorCounters[errName]; !ok {
					mg.ErrorCounters[errName] = errCounter
				} else {
					mg.ErrorCounters[errName] += errCounter
				}
			}
		}

		// Aggregate logs for each etcd pod
		if hasEtcdCounters {
			// aggregate etcd request errors
			log.Debugf("Processing results/Populating/Populating Summary/Processing/MustGather/CalculatingErrors/AggregatingPod{%s}", mg.NamespaceErrors[nsi].Pod)
			if mg.NamespaceErrors[nsi].ErrorEtcdLogs.Buffer != nil {
				mg.ErrorEtcdLogsBuffer = append(mg.ErrorEtcdLogsBuffer, mg.NamespaceErrors[nsi].ErrorEtcdLogs.Buffer...)
			}
			// aggregate etcd error counters
			if mg.NamespaceErrors[nsi].ErrorEtcdLogs.ErrorCounters != nil {
				if mg.ErrorEtcdLogs.ErrorCounters == nil {
					mg.ErrorEtcdLogs.ErrorCounters = make(archive.ErrorCounter, len(commonTestErrorPatternEtcdLogs))
				}
				for errName, errCounter := range mg.NamespaceErrors[nsi].ErrorEtcdLogs.ErrorCounters {
					if _, ok := mg.ErrorEtcdLogs.ErrorCounters[errName]; !ok {
						mg.ErrorEtcdLogs.ErrorCounters[errName] = errCounter
					} else {
						mg.ErrorEtcdLogs.ErrorCounters[errName] += errCounter
					}
				}
			}
		}
	}
	log.Debugf("Processing results/Populating/Populating Summary/Processing/MustGather/CalculatingErrors/CalculatingEtcdErrors")
	mg.CalculateCountersEtcd()
}

// CalculateCountersEtcd creates the aggregators, generating counters for each one.
func (mg *MustGather) CalculateCountersEtcd() {

	// filter Slow Requests (aggregate by hour)
	filterATTL1 := NewFilterApplyTookTooLong("hour")
	for _, line := range mg.ErrorEtcdLogsBuffer {
		filterATTL1.ProcessLine(*line)
	}
	mg.ErrorEtcdLogs.FilterRequestSlowHour = filterATTL1.GetStat(4)

	// filter Slow Requests (aggregate all)
	filterATTL2 := NewFilterApplyTookTooLong("all")
	for _, line := range mg.ErrorEtcdLogsBuffer {
		filterATTL2.ProcessLine(*line)
	}
	mg.ErrorEtcdLogs.FilterRequestSlowAll = filterATTL2.GetStat(1)
}

// Process read the must-gather tarball.
func (mg *MustGather) Process(buf *bytes.Buffer) error {
	log.Debugf("Processing results/Populating/Populating Summary/Processing/MustGather/Reading")
	tar, err := mg.read(buf)
	if err != nil {
		return err
	}
	log.Debugf("Processing results/Populating/Populating Summary/Processing/MustGather/Processing")
	err = mg.extract(tar)
	if err != nil {
		return err
	}
	return nil
}

func (mg *MustGather) read(buf *bytes.Buffer) (*tar.Reader, error) {
	file, err := xz.NewReader(buf)
	if err != nil {
		return nil, err
	}
	return tar.NewReader(file), nil
}

// matchToExtract define patterns to continue the must-gather processor.
// the pattern must be defined if the must be extracted. It will return
// a boolean with match and the file group (pattern type).
func (mg *MustGather) matchToExtract(path string) (bool, string) {
	patterns := make(map[string]string, 4)
	patterns["logs"] = `(\/namespaces\/.*\/pods\/.*.log)`
	patterns["events"] = `(\/event-filter.html)`
	patterns["rawFile"] = `(\/etcd_info\/.*.json)`
	patterns["podNetCheck"] = `(\/pod_network_connectivity_check\/podnetworkconnectivitychecks.yaml)`
	// TODO /host_service_logs/.*.log
	for typ, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(path) {
			return true, typ
		}
	}
	return false, ""
}

// extractRelativePath removes the prefix of must-gather path/image to save the
// relative file path when extracting the file or mapping in the counters.
// OPCT collects must-gather automatically saving in the directory must-gather-opct.
func (mg *MustGather) extractRelativePath(file string) string {
	re := regexp.MustCompile(`must-gather-opct/([A-Za-z0-9]+(-[A-Za-z0-9]+)+\/)`)

	split := re.Split(file, -1)
	if len(split) != 2 {
		return file
	}
	return split[1]
}

// extract dispatch to process must-gather items.
func (mg *MustGather) extract(tarball *tar.Reader) error {

	// Create must-gather directory
	if _, err := os.Stat(mg.path); err != nil {
		if err := os.MkdirAll(mg.path, 0755); err != nil {
			return err
		}
	}

	// TODO()#1: create a queue package with a instance of MustGatherLog.
	// TODO()#2: increase the parallelism targetting to decrease the total proc time.
	// Leaky bucket implementation (queue limit) to parallel process must-gather items
	// without exhausting resources.
	// Benckmark info: this parallel processing decreased 3 times the total processing time.
	// Samples: Serial=~100s, rate(100)=~30s, rate(150)=~25s.
	keepReading := true
	procQueueSize := 0
	var procQueueLocker sync.Mutex
	// Creating queue monitor as Waiter group does not provide interface to check the
	// queue size.
	procQueueInc := func() {
		procQueueLocker.Lock()
		procQueueSize += 1
		procQueueLocker.Unlock()
	}
	procQueueDec := func() {
		procQueueLocker.Lock()
		procQueueSize -= 1
		procQueueLocker.Unlock()
	}
	go func() {
		for keepReading {
			log.Debugf("Must-gather processor - queue size monitor: %d", procQueueSize)
			time.Sleep(10 * time.Second)
		}
	}()

	waiterProcNS := &sync.WaitGroup{}
	chProcNSErrors := make(chan *MustGatherLog, 50)
	semaphore := make(chan struct{}, 50)
	// have a max rate of N/sec
	rate := make(chan struct{}, 20)
	for i := 0; i < cap(rate); i++ {
		rate <- struct{}{}
	}
	// leaky bucket
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			_, ok := <-rate
			// if this isn't going to run indefinitely, signal
			// this to return by closing the rate channel.
			if !ok {
				return
			}
		}
	}()
	// consumer
	go func() {
		for mgLog := range chProcNSErrors {
			mg.processNamespaceErrors(mgLog)
			waiterProcNS.Done()
			procQueueDec()
		}
	}()

	// Walk through files in must-gather tarball file.
	for keepReading {
		header, err := tarball.Next()

		switch {
		// no more files
		case err == io.EOF:
			log.Debugf("Must-gather processor queued, queue size: %d", procQueueSize)
			waiterProcNS.Wait()
			keepReading = false
			log.Debugf("Must-gather processor finished, queue size: %d", procQueueSize)
			return nil

		// return on error
		case err != nil:
			return errors.Wrapf(err, "error reading tarball")
			// return err

		// skip it when the headr isn't set (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created.
		target := filepath.Join(mg.path, header.Name)
		ok, typ := mg.matchToExtract(target)
		if !ok {
			continue
		}
		targetAlias := mg.extractRelativePath(target)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		switch header.Typeflag {
		// directories in tarball.
		case tar.TypeDir:

			// creating subdirectories structures will be ignored and need
			// sub-directories under mg.path must be created previously if needed.
			/*
				targetDir := filepath.Join(mg.path, targetAlias)
				if _, err := os.Stat(targetDir); err != nil {
					if err := os.MkdirAll(targetDir, 0755); err != nil {
						return err
					}
				}
			*/
			continue

		// files in tarball. Process only files classified by 'typ'.
		case tar.TypeReg:
			// Save/Process only files matching now types, it will prevent processing && saving
			// all the files in must-gather, extracting only information needed by OPCT.
			switch typ {
			case "logs":
				// parallel processing the logs
				buf := bytes.Buffer{}
				if _, err := io.Copy(&buf, tarball); err != nil {
					return err
				}
				waiterProcNS.Add(1)
				procQueueInc()
				go func(filename string, buffer *bytes.Buffer) {
					// wait for the rate limiter
					rate <- struct{}{}

					// check the concurrency semaphore
					semaphore <- struct{}{}
					defer func() {
						<-semaphore
					}()
					// log.Debugf("Producing log processor for file: %s", mgLog.Path)
					chProcNSErrors <- &MustGatherLog{
						Path:   filename,
						buffer: buffer,
					}
				}(targetAlias, &buf)

			case "events":
				// forcing file name for event filter
				targetLocal := filepath.Join(mg.path, "event-filter.html")
				f, err := os.OpenFile(targetLocal, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
				if err != nil {
					return err
				}
				if _, err := io.Copy(f, tarball); err != nil {
					return err
				}
				f.Close()

			case "rawFile":
				log.Debugf("Must-gather extracting file %s", targetAlias)
				raw := &MustGatherFile{}
				raw.Path = targetAlias
				buf := bytes.Buffer{}
				if _, err := io.Copy(&buf, tarball); err != nil {
					log.Errorf("error copying rawfile: %v", err)
					break
				}
				raw.Data = buf.String()
				err := mg.InsertRawFiles(raw)
				if err != nil {
					log.Errorf("error inserting rawfile: %v", err)
				}

			case "podNetCheck":
				log.Debugf("Must-gather extracting file %s", targetAlias)
				raw := &MustGatherFile{}
				raw.Path = targetAlias
				buf := bytes.Buffer{}
				if _, err := io.Copy(&buf, tarball); err != nil {
					log.Errorf("error copying rawfile: %v", err)
					break
				}
				var data map[string]interface{}

				err := yaml.Unmarshal(buf.Bytes(), &data)
				if err != nil {
					log.Errorf("error parsing yaml podNetCheck: %v", err)
					break
				}

				mg.PodNetworkChecks.Parse(data)
			}
		}
	}

	return nil
}

// processNamespaceErrors implements the consumer logic, creating the
// mustGather log item, processing it, appending to the data stored in
// NamespaceError. It must not stop on errors, but must log it.
func (mg *MustGather) processNamespaceErrors(mgLog *MustGatherLog) {
	pathItems := strings.Split(mgLog.Path, "namespaces/")

	mgItems := strings.Split(pathItems[1], "/")
	mgLog.Namespace = mgItems[0]
	mgLog.Pod = mgItems[2]
	mgLog.Container = mgItems[3]
	// TODO: log errors
	mgLog.ErrorCounters = archive.NewErrorCounter(pointer.String(mgLog.buffer.String()), ci.CommonErrorPatterns)
	// additional parsers
	if mgLog.Namespace == "openshift-etcd" &&
		mgLog.Container == "etcd" &&
		strings.HasSuffix(mgLog.Path, "current.log") {
		log.Debugf("Must-gather processor - Processing pods logs: %s/%s/%s", mgLog.Namespace, mgLog.Pod, mgLog.Container)
		// TODO: collect errors
		mgLog.ErrorEtcdLogs = NewErrorEtcdLogs(pointer.String(mgLog.buffer.String()))
		log.Debugf("Must-gather processor - Done logs processing: %s/%s/%s", mgLog.Namespace, mgLog.Pod, mgLog.Container)
	}

	// Insert only if there are logs parsed
	if mgLog.Processed() {
		if err := mg.InsertNamespaceErrors(mgLog); err != nil {
			log.Errorf("one or more errors found when inserting errors: %v", err)
		}
	}
}

/* MustGatehr log items */

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
func (mge *MustGatherLog) Processed() bool {
	if len(mge.ErrorCounters) > 0 {
		return true
	}
	if mge.ErrorEtcdLogs != nil {
		return true
	}
	return false
}
