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

	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/client"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/wait"
)

const (
	DefaultStatusIntervalSeconds = 10
	StatusInterval               = time.Second * 10
	StatusRetryLimit             = 10
)

// StatusOptions is the interface to store input options to
// interface with Status command.
type StatusOptions struct {
	StartTime           time.Time
	Latest              *aggregation.Status
	watch               bool
	shownPostProcessMsg bool
	watchInterval       int
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

func NewStatusOptions(in *StatusInput) *StatusOptions {
	s := &StatusOptions{
		watch:        in.Watch,
		waitInterval: time.Second * DefaultStatusIntervalSeconds,
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

func NewCmdStatus() *cobra.Command {
	o := NewStatusOptions(&StatusInput{Watch: false})

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current status of the validation tool",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Pre-checks and setup
			if err := o.PreRunCheck(); err != nil {
				log.WithError(err).Error("error running pre-checks")
				return err
			}

			// Wait for Sonobuoy to create
			if err := wait.WaitForRequiredResources(o.kclient); err != nil {
				log.WithError(err).Error("error waiting for sonobuoy pods to become ready")
				return err
			}

			// Wait for Sononbuoy to start reporting status
			if err := o.WaitForStatusReport(cmd.Context()); err != nil {
				log.WithError(err).Error("error retrieving current aggregator status")
				return err
			}

			if err := o.Print(cmd); err != nil {
				log.WithError(err).Error("error printing status")
				return err
			}
			return nil
		},
	}

	cmd.PersistentFlags().BoolVarP(&o.watch, "watch", "w", false, "Keep watch status after running")
	cmd.Flags().IntVarP(&o.watchInterval, "watch-interval", "", DefaultStatusIntervalSeconds, "Interval to watch the status and print in the stdout")
	if o.watchInterval != DefaultStatusIntervalSeconds {
		o.waitInterval = time.Duration(o.watchInterval) * time.Second
	}

	return cmd
}

func (s *StatusOptions) PreRunCheck() error {
	// Check if sonobuoy namespac already exists
	_, err := s.kclient.CoreV1().Namespaces().Get(context.TODO(), pkg.CertificationNamespace, metav1.GetOptions{})
	if err != nil {
		// If error is due to namespace not being found, return guidance.
		if kerrors.IsNotFound(err) {
			return errors.New("looks like there is no validation environment running. use run command to start the validation process")
		}
	}

	// Sonobuoy namespace exists so no error
	return nil
}

// Update the Sonobuoy state saved in StatusOptions
func (s *StatusOptions) Update() error {
	// TODO Is a retry in here needed?
	sstatus, err := s.sclient.GetStatus(&sonobuoyclient.StatusConfig{Namespace: pkg.CertificationNamespace})
	if err != nil {
		return err
	}

	s.Latest = sstatus
	return nil
}

// GetStatusForPlugin will get a plugin's status from the state saved in StatusOptions
func (s *StatusOptions) GetStatusForPlugin(name string) *aggregation.PluginStatus {
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
func (s *StatusOptions) GetStatus() string {
	if s.Latest != nil {
		return s.Latest.Status
	}

	return ""
}

// WaitForStatusReport will block until either context is canceled, status is reported, or retry limit reach.
// An error will not result in immediate failure and will be retried.
func (s *StatusOptions) WaitForStatusReport(ctx context.Context) error {
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

func (s *StatusOptions) Print(cmd *cobra.Command) error {
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

func (s *StatusOptions) doPrint() (complete bool, err error) {
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

func (s *StatusOptions) GetSonobuoyClient() sonobuoyclient.Interface {
	return s.sclient
}
