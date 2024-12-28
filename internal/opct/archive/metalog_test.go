/*
Handle items in the file path meta/run.log
*/
package archive

import (
	"log"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	opcttests "github.com/redhat-openshift-ecosystem/opct/test"
)

func TestParseMetaLogs(t *testing.T) {

	testFile := "testdata/archive-001/meta/run.log"
	raw, err := opcttests.TestData.ReadFile(testFile)
	if err != nil {
		log.Fatalf("unable to load test data %s: %v", testFile, err)
	}
	validTestDataSet := strings.Split(string(raw), "\n")

	type args struct {
		logs []string
	}
	tests := []struct {
		name string
		args args
		want []*RuntimeInfoItem
	}{
		{
			name: "empty",
			args: args{
				logs: []string{},
			},
			want: []*RuntimeInfoItem{},
		},
		{
			name: "parse server start",
			args: args{
				logs: []string{`{"msg":"Starting server Expected Results: ...","time":"2023-09-28T00:00:00Z"}`},
			},
			want: []*RuntimeInfoItem{
				{
					Name: "server started",
					Time: "2023-09-28T00:00:00Z",
				},
			},
		},
		{
			name: "parse all",
			args: args{
				logs: validTestDataSet,
			},
			want: []*RuntimeInfoItem{
				{Name: "server started", Time: "2023-09-28T00:00:00Z"},
				{Name: "plugin started 99-openshift-artifacts-collector", Time: "2023-09-28T00:10:00Z"},
				{Name: "plugin started 05-openshift-cluster-upgrade", Time: "2023-09-28T00:10:00Z"},
				{Name: "plugin started 20-openshift-conformance-validated", Time: "2023-09-28T00:10:00Z"},
				{Name: "plugin started 10-openshift-kube-conformance", Time: "2023-09-28T00:10:00Z"},
				{Name: "plugin finished 05-openshift-cluster-upgrade", Time: "2023-09-28T00:20:00Z", Total: "10m0s", Delta: "10m0s"},
				{Name: "plugin finished 10-openshift-kube-conformance", Time: "2023-09-28T00:30:00Z", Total: "20m0s", Delta: "10m0s"},
				{Name: "plugin finished 20-openshift-conformance-validated", Time: "2023-09-28T01:30:00Z", Total: "1h20m0s", Delta: "1h0m0s"},
				{Name: "plugin finished 99-openshift-artifacts-collector", Time: "2023-09-28T02:00:00Z", Total: "1h50m0s", Delta: "30m0s"},
				{Name: "server finished", Time: "2023-09-28T02:00:00Z", Total: "2h0m0s"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseMetaLogs(tt.args.logs); !reflect.DeepEqual(got, tt.want) {
				if !cmp.Equal(got, tt.want) {
					return
				}
			}
		})
	}
}
