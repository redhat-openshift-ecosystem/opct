package summary

import (
	"bytes"
	"strings"
)

const (
	SuiteNameKubernetesConformance = "kubernetes/conformance"
	SuiteNameOpenshiftConformance  = "openshift/conformance"
)

type OpenshiftTestsSuites struct {
	KubernetesConformance *OpenshiftTestsSuite
	OpenshiftConformance  *OpenshiftTestsSuite
}

func (ts *OpenshiftTestsSuites) GetTotalOCP() int {
	return ts.OpenshiftConformance.Count
}

func (ts *OpenshiftTestsSuites) GetTotalK8S() int {
	return ts.KubernetesConformance.Count
}

type OpenshiftTestsSuite struct {
	InputFile string
	Name      string
	Count     int
	Tests     []string
}

func (s *OpenshiftTestsSuite) Load(ifile string, buf *bytes.Buffer) error {
	var e2e []string
	for _, m := range strings.Split(buf.String(), "\n") {
		if m != "" {
			e2e = append(e2e, strings.Trim(m, "\""))
		}
	}
	s.InputFile = ifile
	s.Tests = e2e
	s.Count = len(s.Tests)
	return nil
}
