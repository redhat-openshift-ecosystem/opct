package plugin

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type TestDocumentation struct {
	UserBaseURL   *string
	SourceBaseURL *string
	Raw           *string
	Tests         map[string]*TestDocumentationItem
}

type TestDocumentationItem struct {
	Title    string
	Name     string
	PageLink string
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

func (d *TestDocumentation) BuildIndex() error {
	lines := strings.Split(*d.Raw, "\n")
	d.Tests = make(map[string]*TestDocumentationItem, len(lines))
	for number, line := range lines {

		// Build index for Kubernetes Conformance tests, parsing the page for version:
		// https://github.com/cncf/k8s-conformance/blob/master/docs/KubeConformance-1.27.md
		if strings.HasPrefix(line, "- Defined in code as: ") {
			testName := strings.Split(line, "Defined in code as: ")[1]
			d.Tests[testName] = &TestDocumentationItem{
				Title: lines[number-3],
				Name:  testName,
			}
			// create links for each test section
			reDoc := regexp.MustCompile(`^## \[(.*)\]`)
			match := reDoc.FindStringSubmatch(lines[number-3])
			if len(match) == 2 {
				link := match[1]
				for _, c := range []string{":", "-", ".", ",", "="} {
					link = strings.Replace(link, c, "", -1)
				}
				link = strings.Replace(link, " ", "-", -1)
				link = strings.ToLower(link)
				d.Tests[testName].PageLink = fmt.Sprintf("%s#%s", *d.UserBaseURL, link)
			}
		}
	}
	return nil
}
