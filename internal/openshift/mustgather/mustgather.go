package mustgather

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/utils/ptr"
)

// rawFile hold the raw data from must-gather.
type rawFile struct {
	Path      string
	PathAlias string `json:"PathAlias,omitempty"`
	Data      string `json:"Data,omitempty"`
}

type MustGather struct {
	// path to the directory must-gather will be saved.
	path string
	save bool

	// ErrorEtcdLogs summary of etcd errors parsed from must-gather.
	ErrorEtcdLogs       *ErrorEtcdLogs `json:"ErrorEtcdLogs,omitempty"`
	ErrorEtcdLogsBuffer []*string      `json:"-"`

	// ErrorCounters summary error counters parsed from must-gather.
	ErrorCounters archive.ErrorCounter `json:"ErrorCounters,omitempty"`

	// NamespaceErrors hold pods reporting errors.
	NamespaceErrors []*MustGatherLog `json:"NamespaceErrors,omitempty"`
	namespaceCtrl   sync.Mutex

	// FileData hold raw data from files must-gather.
	RawFiles     []*rawFile `json:"RawFiles,omitempty"`
	rawFilesCtrl sync.Mutex

	PodNetworkChecks MustGatherPodNetworkChecks
}

func NewMustGather(file string, save bool) *MustGather {
	return &MustGather{
		path: file,
		save: save,
	}
}

// Process reads and process in memory the must-gather tarball file.
func (mg *MustGather) Process(buf *bytes.Buffer) error {
	log.Debugf("Processing results/Populating/Populating Summary/Processing/MustGather/Reading")
	tar, err := getTarFromXZBuffer(buf)
	if err != nil {
		return err
	}

	log.Debugf("Processing results/Populating/Populating Summary/Processing/MustGather/Processing")
	if err := mg.extract(tar); err != nil {
		return err
	}
	return nil
}

func (mg *MustGather) AggregateCounters() {
	if mg.ErrorCounters == nil {
		mg.ErrorCounters = make(archive.ErrorCounter, len(archive.CommonErrorPatterns))
	}
	if mg.ErrorEtcdLogs == nil {
		mg.ErrorEtcdLogs = &ErrorEtcdLogs{}
	}
	// calculate error findings across all nesmapces.
	for nsi := range mg.NamespaceErrors {
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
					mg.ErrorEtcdLogs.ErrorCounters = make(archive.ErrorCounter, len(EtcdLogErrorPatterns))
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
	mg.calculateCountersEtcd()
}

// insertNamespaceErrors append the extracted information to the namespaced-resource.
func (mg *MustGather) insertNamespaceErrors(log *MustGatherLog) error {
	mg.namespaceCtrl.Lock()
	mg.NamespaceErrors = append(mg.NamespaceErrors, log)
	mg.namespaceCtrl.Unlock()
	return nil
}

// insertRawFiles append the file data in safe way.
func (mg *MustGather) insertRawFiles(file *rawFile) error {
	mg.rawFilesCtrl.Lock()
	mg.RawFiles = append(mg.RawFiles, file)
	mg.rawFilesCtrl.Unlock()
	return nil
}

// calculateCountersEtcd creates the aggregators, generating counters for each one.
func (mg *MustGather) calculateCountersEtcd() {

	// filter Slow Requests (aggregate by hour)
	filterATTL1 := NewFilterApplyTookTooLong("hour")
	for _, line := range mg.ErrorEtcdLogsBuffer {
		filterATTL1.ProcessLine(*line)
	}
	mg.ErrorEtcdLogs.FilterRequestSlowHour = filterATTL1.GetStat(parserETCDLogsReqTTLMaxPastHour)

	// filter Slow Requests (aggregate all)
	filterATTL2 := NewFilterApplyTookTooLong("all")
	for _, line := range mg.ErrorEtcdLogsBuffer {
		filterATTL2.ProcessLine(*line)
	}
	mg.ErrorEtcdLogs.FilterRequestSlowAll = filterATTL2.GetStat(1)
}

// extract reads, and process the tarball and extract the required information.
func (mg *MustGather) extract(tarball *tar.Reader) error {
	// Create must-gather directory under the result path.
	// Creates directory only when needs it.
	if mg.save {
		if _, err := os.Stat(mg.path); err != nil {
			if err := os.MkdirAll(mg.path, 0755); err != nil {
				return fmt.Errorf("error creating must-gather directory: %v", err)
			}
		}
	}

	processorBucket := newLeakyBucket(defaultSizeLeakyBucket, defaultRateLimitIntervalLeakyBucket, mg.processNamespaceErrors)

	// Walk through files in must-gather tarball file.
	for processorBucket.activeReading {
		header, err := tarball.Next()

		switch {
		// no more files
		case err == io.EOF:
			log.Debugf("Must-gather processor queued, queue size: %d", processorBucket.queueCount)
			processorBucket.waiter.Wait()
			processorBucket.activeReading = false
			log.Debugf("Must-gather processor finished, queue size: %d", processorBucket.queueCount)
			return nil

		// return on error
		case err != nil:
			return errors.Wrapf(err, "error reading tarball")

		// skip it when the headr isn't set (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created.
		target := filepath.Join(mg.path, header.Name)

		// check if the file should be processed.
		ok, itemType := getFileTypeToProcess(target)
		if !ok {
			continue
		}
		targetAlias := normalizeRelativePath(target)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		switch header.Typeflag {

		// directories in tarball.
		case tar.TypeDir:
			// creating subdirectories structures will be ignored and need
			// sub-directories under mg.path must be created previously if needed.
			// Enable it only there is a use case to extract more data to disk preserving source dirs.
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
			// Save/Process only files matching known types, it will prevent processing && saving
			// all the files in must-gather, extracting only information required by OPCT.
			switch itemType {
			case patternNamePodLogs:
				// logs are processed in parallel, the buffer is released when processed.
				buf := bytes.Buffer{}
				if _, err := io.Copy(&buf, tarball); err != nil {
					log.Errorf("must-gather processor/podLogs: error copying buffer for %s: %v", targetAlias, err)
					continue
				}
				processorBucket.Incremet()
				go func(filename string, buffer *bytes.Buffer) {
					processorBucket.AppendQueue(&MustGatherLog{
						Path:   filename,
						buffer: buffer,
					})
				}(targetAlias, &buf)

			case patternNameEvents:
				// skip extracting when save directory is not set. (in-memory processing only)
				if !mg.save {
					log.Debugf("skipping file %s", targetAlias)
					continue
				}
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

			case patternNameRawFile:
				log.Debugf("Must-gather extracting file %s", targetAlias)
				raw := &rawFile{}
				raw.Path = targetAlias
				buf := bytes.Buffer{}
				if _, err := io.Copy(&buf, tarball); err != nil {
					log.Errorf("error copying rawfile: %v", err)
					break
				}
				raw.Data = buf.String()
				err := mg.insertRawFiles(raw)
				if err != nil {
					log.Errorf("error inserting rawfile: %v", err)
				}

			case patternNamePodNetCheck:
				log.Debugf("Must-gather extracting file %s", targetAlias)
				raw := &rawFile{}
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

	// parse errors from logs
	mgLog.ErrorCounters = archive.NewErrorCounter(ptr.To(mgLog.buffer.String()), archive.CommonErrorPatterns)

	// additional parsers: etcd error counter extractor
	if mgLog.Namespace == "openshift-etcd" &&
		mgLog.Container == "etcd" &&
		strings.HasSuffix(mgLog.Path, "current.log") {
		log.Debugf("Must-gather processor - Processing pods logs: %s/%s/%s", mgLog.Namespace, mgLog.Pod, mgLog.Container)
		// TODO: collect errors
		mgLog.ErrorEtcdLogs = NewErrorEtcdLogs(ptr.To(mgLog.buffer.String()))
		log.Debugf("Must-gather processor - Done logs processing: %s/%s/%s", mgLog.Namespace, mgLog.Pod, mgLog.Container)
	}

	// Insert only if there are logs parsed
	if mgLog.Processed() {
		if err := mg.insertNamespaceErrors(mgLog); err != nil {
			log.Errorf("one or more errors found when inserting errors: %v", err)
		}
	}

	// release buffer
	mgLog.buffer.Reset()
}
