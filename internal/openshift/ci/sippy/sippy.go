package sippy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultConnTimeoutSec       = 10
	defaultMaxIdleConns         = 100
	defaultMaxConnsPerHost      = 100
	defaultMaxIddleConnsPerHost = 100
	apiBaseURL                  = "https://sippy.dptools.openshift.org/api"
	apiPathTests                = "/tests"
)

type SippyTestsRequestInput struct {
	TestName string
	Release  string
	Filter   SippyTestsRequestFilter
}

// SippyTestsRequestFilter is the filter structure to the Sippy query to /tests
type SippyTestsRequestFilter struct {
	// Example: {"items":[{"columnField":"name","operatorValue":"equals","value":"test_name"}]}
	Items []SippyTestsRequestFilterItems `json:"items"`
}

// SippyTestsRequestFilterItems is the filter parameters
type SippyTestsRequestFilterItems struct {
	ColumnField   string `json:"columnField"`
	OperatorValue string `json:"operatorValue"`
	Value         string `json:"value"`
}

// SippyTestsResponse is the payload item returned by the API endpoint /tests
type SippyTestsResponse struct {
	Name               string  `json:"name"`
	CurrentFailures    int64   `json:"current_failures"`
	CurrentFlakes      int64   `json:"current_flakes"`
	CurrentRuns        int64   `json:"current_runs"`
	CurrentPassPerc    float64 `json:"current_pass_percentage"`
	CurrentFlakePerc   float64 `json:"current_flake_percentage"`
	CurrentWorkingPerc float64 `json:"current_working_percentage"`
	PreviousFailures   int64   `json:"previous_failures"`
	PreviousFlakes     int64   `json:"previous_flakes"`
}

// SippyTestsRequestOutput is the payload returned by the API endpoint /tests
type SippyTestsRequestOutput []SippyTestsResponse

// SippyAPI is the Sippy API structure holding the API client
type SippyAPI struct {
	client     *http.Client
	ocpVersion string
}

// NewSippyAPI creates a new API setting the http attributes to improve the connection reuse.
func NewSippyAPI(ocpVersion string) *SippyAPI {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = defaultMaxIdleConns
	t.MaxConnsPerHost = defaultMaxConnsPerHost
	t.MaxIdleConnsPerHost = defaultMaxIddleConnsPerHost

	return &SippyAPI{
		ocpVersion: ocpVersion,
		client: &http.Client{
			Timeout:   defaultConnTimeoutSec * time.Second,
			Transport: t,
		},
	}
}

// QueryTests receive a input with attributes to query the results of a single test
// by name on the CI, returning the list with result items.
func (a *SippyAPI) QueryTests(in *SippyTestsRequestInput) (*SippyTestsRequestOutput, error) {

	filter := SippyTestsRequestFilter{
		Items: []SippyTestsRequestFilterItems{
			{
				ColumnField:   "name",
				OperatorValue: "equals",
				Value:         in.TestName,
			},
		},
	}

	b, err := json.Marshal(filter)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse response body. %+v", err)
	}

	baseUrl, err := url.Parse(apiBaseURL + apiPathTests)
	if err != nil {
		return nil, fmt.Errorf("malformed URL: %+v", err)
	}

	params := url.Values{}
	params.Add("release", a.ocpVersion)
	params.Add("filter", string(b))

	baseUrl.RawQuery = params.Encode()

	req, err := http.NewRequest(http.MethodGet, baseUrl.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't create the request: %+v", err)
	}

	res, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("couldn't call URL %s: %+v", baseUrl.String(), err)

	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse response body. %+v", err)

	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("invalid status code: %d", res.StatusCode)
	}

	sippyResponse := SippyTestsRequestOutput{}
	if err := json.Unmarshal([]byte(body), &sippyResponse); err != nil {
		return nil, fmt.Errorf("couldn't unmarshal response body: %+v \nBody: %s", string(body), err)
	}
	return &sippyResponse, nil
}
