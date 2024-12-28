package status

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	sonobuoyclient "github.com/vmware-tanzu/sonobuoy/pkg/client"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/aggregation"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	wait2 "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
	"github.com/redhat-openshift-ecosystem/opct/pkg/client"
)

const (
	DefaultStatusIntervalSeconds = 10
	StatusInterval               = time.Second * 10
	StatusRetryLimit             = 10
)

// Status is the interface to store input options to
// interface with Status command.
type Status struct {
	StartTime           time.Time
	Latest              *aggregation.Status
	watch               bool
	shownPostProcessMsg bool
	waitInterval        time.Duration

	// clients
	kclient kubernetes.Interface
	sclient sonobuoyclient.Interface
}

type StatusInput struct {
	Watch           bool
	IntervalSeconds int

	// clients
	KClient kubernetes.Interface
	SClient sonobuoyclient.Interface
}

func NewStatus(in *StatusInput) *Status {
	s := &Status{
		watch:        in.Watch,
		waitInterval: DefaultStatusIntervalSeconds * time.Second,
		StartTime:    time.Now(),
	}
	if in.IntervalSeconds != 0 {
		s.waitInterval = time.Duration(in.IntervalSeconds) * time.Second
	}
	kclient, sclient, err := client.CreateClients()
	if err != nil {
		log.WithError(err).Errorf("error creating clients: %v", err)
		return s
	}
	s.kclient = in.KClient
	if s.kclient == nil {
		s.kclient = kclient
	}
	s.sclient = in.SClient
	if s.sclient == nil {
		s.sclient = sclient
	}
	return s
}

// PreRunCheck will check if the validation environment is running.
func (s *Status) PreRunCheck() error {
	_, err := s.kclient.CoreV1().Namespaces().Get(context.TODO(), pkg.CertificationNamespace, metav1.GetOptions{})
	if err != nil {
		// If error is due to namespace not being found, return guidance.
		if kerrors.IsNotFound(err) {
			return errors.New("looks like there is no validation environment running. use run command to start the validation process")
		}
	}
	return nil
}

// Update the Sonobuoy state saved in Status
func (s *Status) Update() error {
	sstatus, err := s.sclient.GetStatus(&sonobuoyclient.StatusConfig{Namespace: pkg.CertificationNamespace})
	if err != nil {
		return err
	}

	s.Latest = sstatus
	return nil
}

// GetStatusForPlugin will get a plugin's status from the state saved in Status.
func (s *Status) GetStatusForPlugin(name string) *aggregation.PluginStatus {
	if s.Latest == nil {
		return nil
	}

	for _, pstatus := range s.Latest.Plugins {
		if pstatus.Plugin == name {
			return &pstatus
		}
	}

	return nil
}

// GetStatus returns the latest aggregator status if there is one, otherwise empty string.
func (s *Status) GetStatus() string {
	if s.Latest != nil {
		return s.Latest.Status
	}

	return ""
}

// WaitForStatusReport will block until either context is canceled, status is reported, or retry limit reach.
// An error will not result in immediate failure and will be retried.
func (s *Status) WaitForStatusReport(ctx context.Context) error {
	tries := 1
	err := wait2.PollUntilContextCancel(ctx, s.waitInterval, true, func(ctx context.Context) (done bool, err error) {
		if tries == StatusRetryLimit {
			return false, errors.New("retry limit reached checking for aggregator status")
		}

		if err := s.Update(); err != nil {
			log.WithError(err).Warn("error retrieving current aggregator status")
		} else if s.Latest.Status != "" {
			return true, nil
		}

		tries++
		log.Warnf("waiting %ds to retry", int(StatusInterval.Seconds()))
		return false, nil
	})
	return err
}

func (s *Status) Print(cmd *cobra.Command) error {
	if !s.watch {
		_, err := s.doPrint()
		return err
	}

	tries := 1
	return wait2.PollUntilContextCancel(cmd.Context(), s.waitInterval, true, func(ctx context.Context) (done bool, err error) {
		if tries == StatusRetryLimit {
			// we hit back-to-back errors too many times.
			return true, errors.New("retry limit reached checking status")
		}
		err = s.Update()
		if err != nil {
			tries++ // increment retries sinc we hit error.
			log.Error(err)
			return false, nil
		}
		tries = 1 // reset retries
		return s.doPrint()
	})
}

func (s *Status) doPrint() (complete bool, err error) {
	switch s.GetStatus() {
	case aggregation.RunningStatus:
		if err := s.printRunningStatus(); err != nil {
			return false, err
		}
	case aggregation.PostProcessingStatus:
		if !s.watch {
			if err := s.printRunningStatus(); err != nil {
				return false, err
			}
		} else if !s.shownPostProcessMsg {
			if err := s.printRunningStatus(); err != nil {
				return false, err
			}
			log.Info("Waiting for post-processor...")
			s.shownPostProcessMsg = true
		}
	case aggregation.CompleteStatus:
		if err := s.printRunningStatus(); err != nil {
			return true, err
		}
		log.Infof("The execution has completed! Use retrieve command to collect the results and share the archive with your Red Hat partner.")
		return true, nil
	default:
		log.Infof("Unknown state %s", s.GetStatus())
	}

	return false, nil
}

func (s *Status) GetSonobuoyClient() sonobuoyclient.Interface {
	return s.sclient
}
