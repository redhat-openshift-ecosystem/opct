package status

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/aggregation"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	wait2 "k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/provider-certification-tool/pkg"
	"github.com/openshift/provider-certification-tool/pkg/wait"
)

const (
	StatusInterval   = time.Second * 10
	StatusRetryLimit = 10
)

type StatusOptions struct {
	config *pkg.Config
	Latest *aggregation.Status

	shownPostProcessMsg bool
}

func NewStatusOptions(config *pkg.Config) *StatusOptions {
	return &StatusOptions{
		config: config,
	}
}

func NewCmdStatus(config *pkg.Config) *cobra.Command {
	o := NewStatusOptions(config)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current status of the certification tool",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			// Pre-checks and setup
			err := o.PreRunCheck()
			if err != nil {
				log.WithError(err).Error("error running pre-checks")
				return
			}

			// Wait for Sonobuoy to create
			err = wait.WaitForRequiredResources(o.config)
			if err != nil {
				log.WithError(err).Error("error waiting for sonobuoy pods to become ready")
				return
			}

			// Wait for Sononbuoy to start reporting status
			err = o.WaitForStatusReport(cmd.Context())
			if err != nil {
				log.WithError(err).Error("error retrieving current aggregator status")
				return
			}

			err = o.Print(cmd)
			if err != nil {
				log.WithError(err).Error("error printing status")
				return
			}
		},
	}

	cmd.PersistentFlags().BoolVarP(&o.config.Watch, "watch", "w", false, "Keep watch status after running")

	return cmd
}

func (s *StatusOptions) PreRunCheck() error {
	client := s.config.Clientset.CoreV1()

	// Check if sonobuoy namespac already exists
	_, err := client.Namespaces().Get(context.TODO(), pkg.CertificationNamespace, metav1.GetOptions{})
	if err != nil {
		// If error is due to namespace not being found, return guidance.
		if kerrors.IsNotFound(err) {
			return errors.New("looks like there is no Certification environment running. use run command to start Certification process")
		}
	}

	// Sonobuoy namespace exists so no error
	return nil
}

// Update the Sonobuoy state saved in StatusOptions
func (s *StatusOptions) Update() error {
	// TODO Is a retry in here needed?
	sstatus, err := s.config.SonobuoyClient.GetStatus(&client.StatusConfig{Namespace: pkg.CertificationNamespace})
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
	err := wait2.PollImmediateUntilWithContext(ctx, StatusInterval, func(ctx context.Context) (done bool, err error) {
		if tries == StatusRetryLimit {
			return false, errors.New("retry limit reached checking for aggregator status")
		}

		err = s.Update()
		if err != nil {
			// TODO Should the warning be shown to user by default? It can be misleading during startup
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
	if !s.config.Watch {
		_, err := s.doPrint(cmd)
		return err
	}

	tries := 1
	return wait2.PollImmediateInfiniteWithContext(cmd.Context(), StatusInterval, func(ctx context.Context) (done bool, err error) {
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
		return s.doPrint(cmd)
	})
}

func (s *StatusOptions) doPrint(cmd *cobra.Command) (complete bool, err error) {
	switch s.GetStatus() {
	case aggregation.RunningStatus:
		err := PrintRunningStatus(s.Latest)
		if err != nil {
			return false, err
		}
	case aggregation.PostProcessingStatus:
		if !s.config.Watch {
			err := PrintRunningStatus(s.Latest)
			if err != nil {
				return false, err
			}
		} else if !s.shownPostProcessMsg {
			log.Info("Waiting for post-processor...")
			s.shownPostProcessMsg = true
		}
	case aggregation.CompleteStatus:
		if !s.config.Watch || !s.shownPostProcessMsg {
			log.Infof("The execution has completed! Run '%s retrieve' to collect the results.", cmd.Root().Name())
			return true, nil
		}
		err := PrintRunningStatus(s.Latest)
		return true, err
	default:
		log.Infof("Unknown state %s", s.GetStatus())
	}

	return false, nil
}
