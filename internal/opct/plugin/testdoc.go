package plugin

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// TestDocumentation is the struct that holds the test documentation.
// The struct is used to store the documentation URL, the raw data, and the
// tests indexed by name.
// The test documentation is discovered by name, and the URL fragment is used
// to mount the URL for the test documentation.
type TestDocumentation struct {
	// UserBaseURL is a the User Facing base URL for the documentation.
	UserBaseURL *string

	// SourceBaseURL is the raw URL to be indexed.
	SourceBaseURL *string

	// Raw stores the data extracted from SourceBaseURL.
	Raw *string

	// Tests is the map indexed by test name, with URL fragment (page references) as a value.
	// Example: for the e2e test '[sig-machinery] run instance', the following map will be created:
	// map['[sig-machinery] run instance']='#sig-machinery--run-instance'
	Tests map[string]*TestDocumentationItem
}

// TestDocumentationItem refers to items documented by
type TestDocumentationItem struct {
	Title string
	Name  string
	// URLFragment stores the discovered fragment parsed by the Documentation page,
	// indexed by test name, used to mount the Documentation URL for failed tests.
	URLFragment string
}

func NewTestDocumentation(user, source string) *TestDocumentation {
	return &TestDocumentation{
		UserBaseURL:   &user,
		SourceBaseURL: &source,
	}
}

// Load documentation from Suite and save it to further query
func (d *TestDocumentation) Load() error {
	app := "Test Documentation"
	req, err := http.NewRequest(http.MethodGet, *d.SourceBaseURL, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to create request to get %s", app)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "failed to make request to %s", app)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("unexpected HTTP status code to %s", app))
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to read response body for %s", app)
	}
	str := string(resBody)
	d.Raw = &str
	return nil
}

// BuildIndex reads the raw Document, discoverying the test name, and the URL
// fragments. The parser is based in the Kubernetes Conformance documentation:
// https://github.com/cncf/k8s-conformance/blob/master/docs/KubeConformance-1.27.md
func (d *TestDocumentation) BuildIndex() error {
	lines := strings.Split(*d.Raw, "\n")
	d.Tests = make(map[string]*TestDocumentationItem, len(lines))
	for number, line := range lines {

		// Build index for Kubernetes Conformance tests, parsing the page for version:
		// https://github.com/cncf/k8s-conformance/blob/master/docs/KubeConformance-1.27.md
		if strings.HasPrefix(line, "- Defined in code as: ") {
			testArr := strings.Split(line, "Defined in code as: ")
			if len(testArr) < 2 {
				log.Debugf("Error BuildIndex(): unable to build documentation index for line: %s", line)
			}
			testName := testArr[1]
			d.Tests[testName] = &TestDocumentationItem{
				Name: testName,
				// The test reference/section are defined in the third line before the name definition.
				Title: lines[number-3],
			}

			// create url fragment for each test section
			reDoc := regexp.MustCompile(`^## \[(.*)\]`)
			match := reDoc.FindStringSubmatch(lines[number-3])
			if len(match) == 2 {
				fragment := match[1]
				// mount the fragment removing undesired symbols.
				for _, c := range []string{":", "-", ".", ",", "="} {
					fragment = strings.Replace(fragment, c, "", -1)
				}
				fragment = strings.Replace(fragment, " ", "-", -1)
				fragment = strings.ToLower(fragment)
				d.Tests[testName].URLFragment = fmt.Sprintf("%s#%s", *d.UserBaseURL, fragment)
			}
		}
	}
	return nil
}
