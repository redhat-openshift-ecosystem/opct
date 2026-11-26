package api

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

// Parse the XML data (JUnit created by openshift-tests)
type TestStatus string

const (
	TestStatusPass    TestStatus = "pass"
	TestStatusFail    TestStatus = "fail"
	TestStatusSkipped TestStatus = "skipped"
)

type propSkipped struct {
	Message string `xml:"message,attr"`
}

type Property struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type TestCase struct {
	Name      string      `xml:"name,attr"`
	Time      string      `xml:"time,attr"`
	Failure   string      `xml:"failure"`
	Skipped   propSkipped `xml:"skipped"`
	SystemOut string      `xml:"system-out"`
	Status    TestStatus
}

type TestSuite struct {
	XMLName    xml.Name   `xml:"testsuite"`
	Name       string     `xml:"name,attr"`
	Tests      int        `xml:"tests,attr"`
	Skipped    int        `xml:"skipped,attr"`
	Failures   int        `xml:"failures,attr"`
	Time       string     `xml:"time,attr"`
	Property   Property   `xml:"property"`
	Properties Property   `xml:"properties,omitempty"`
	TestCases  []TestCase `xml:"testcase"`
}

type TestSuites struct {
	Tests     int       `xml:"tests,attr"`
	Disabled  int       `xml:"disabled,attr"`
	Errors    int       `xml:"errors,attr"`
	Failures  int       `xml:"failures,attr"`
	Time      string    `xml:"time,attr"`
	TestSuite TestSuite `xml:"testsuite"`
}

type JUnitCounter struct {
	Total    int
	Skipped  int
	Failures int
	Pass     int
}

type JUnitXMLParser struct {
	XMLFile  string
	Parsed   *TestSuite
	Counters *JUnitCounter
	Failures []string
	Cases    []*TestCase
}

func NewJUnitXMLParser(xmlFile string) (*JUnitXMLParser, error) {
	xmlData, err := os.ReadFile(xmlFile)
	if err != nil {
		return nil, fmt.Errorf("error reading XML file: %w", err)
	}
	p, err := NewJUnitXMLParserFromBytes(xmlData)
	if err != nil {
		return nil, err
	}
	p.XMLFile = xmlFile
	return p, nil
}

// NewJUnitXMLParserFromBytes parses JUnit XML data already loaded in memory,
// for example from a results archive, and computes the test counters from the
// test cases rather than the testsuite attributes.
func NewJUnitXMLParserFromBytes(xmlData []byte) (*JUnitXMLParser, error) {
	p := &JUnitXMLParser{
		Parsed:   &TestSuite{},
		Counters: &JUnitCounter{},
		Cases:    []*TestCase{},
	}
	// Archives redacted by the OPCT cleaner replace secrets with a raw
	// "<REDACTED_BY_OPCT>" marker which is not XML-escaped, making otherwise
	// valid JUnit files (the sonobuoy results were post-processed before the
	// redaction) unparsable. Escape the marker and retry before giving up.
	if err := xml.Unmarshal(xmlData, p.Parsed); err != nil {
		if escaped := bytes.ReplaceAll(xmlData, []byte("<REDACTED_BY_OPCT>"), []byte("&lt;REDACTED_BY_OPCT&gt;")); !bytes.Equal(escaped, xmlData) {
			log.Warnf("Found redaction markers while parsing JUnit XML, retrying with escaped markers...")
			xmlData = escaped
			// A failed Unmarshal may have appended test cases to the struct
			// before the syntax error, so reset it before the retry.
			p.Parsed = &TestSuite{}
			err = xml.Unmarshal(xmlData, p.Parsed)
		}
		if err != nil {
			ts := &TestSuites{}
			if err.Error() == "expected element type <testsuite> but have <testsuites>" {
				log.Warnf("Found errors while processing default JUnit format, attempting new JUnit format for e2e...")
				if err := xml.Unmarshal(xmlData, ts); err != nil {
					return nil, fmt.Errorf("error parsing XML data with testsuites: %w", err)
				}
				p.Parsed = &ts.TestSuite
			} else {
				return nil, fmt.Errorf("error parsing XML data: %w", err)
			}
		}
	}
	// Iterate over the test cases
	for _, testcase := range p.Parsed.TestCases {
		// Create a new local copy of the testcase
		tc := &testcase
		p.Counters.Total++
		if len(testcase.Skipped.Message) > 0 {
			p.Counters.Skipped++
			tc.Status = TestStatusSkipped
			p.Cases = append(p.Cases, tc)
			continue
		}
		if len(testcase.Failure) > 0 {
			p.Counters.Failures++
			p.Failures = append(p.Failures, fmt.Sprintf("\"%s\"", testcase.Name))
			tc.Status = TestStatusFail
			p.Cases = append(p.Cases, tc)
			continue
		}
		tc.Status = TestStatusPass
		p.Cases = append(p.Cases, tc)
	}
	p.Counters.Pass = p.Counters.Total - (p.Counters.Skipped + p.Counters.Failures)

	return p, nil
}
