package status

import (
	"fmt"
	"html/template"
	"os"
	"testing"
	"time"

	"github.com/vmware-tanzu/sonobuoy/pkg/plugin"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/aggregation"
)

func Test_PrintStatus(t *testing.T) {
	a := &aggregation.Status{
		Plugins: []aggregation.PluginStatus{
			{
				Plugin:             "test",
				Status:             "running",
				ResultStatus:       "failed",
				ResultStatusCounts: map[string]int{"passed": 10, "failed": 5},
				Progress: &plugin.ProgressUpdate{
					Total:     30,
					Completed: 10,
					Errors:    nil,
					Failures:  []string{"a", "b", "c"},
					Message:   "waiting post-processor...",
				},
			},
		},
		Status: "running",
	}
	now := time.Now()
	ps := getPrintableRunningStatus(a, now)

	tmpl, err := template.New("test").Parse(runningStatusTemplate)
	if err != nil {
		fmt.Println(err)
		return
	}

	_ = tmpl.Execute(os.Stderr, ps)
}
