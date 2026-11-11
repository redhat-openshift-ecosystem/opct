/*
Handle items in the file path meta/run.log
*/
package archive

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	opcttests "github.com/redhat-openshift-ecosystem/opct/test"
)

func TestParseMetaLogs(t *testing.T) {

	testFile := "testdata/archive-001/meta/run.log"
	raw, err := opcttests.TestData.ReadFile(testFile)
	if err != nil {
		t.Fatalf("unable to load test data %s: %v", testFile, err)
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
				{Name: "plugin finished 99-openshift-artifacts-collector", Time: "2023-09-28T02:00:00Z", Total: "1h50m0s", Delta: "1h50m0s"},
				{Name: "server finished", Time: "2023-09-28T02:00:00Z", Total: "2h0m0s"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, ParseMetaLogs(tt.args.logs)); diff != "" {
				t.Errorf("ParseMetaLogs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseMetaLogsDurations(t *testing.T) {
	t.Parallel()
	const start = "2026-10-21T15:24:43Z"
	const end = "2026-10-21T15:34:43Z"
	tests := []struct {
		name        string
		start       string
		end         string
		omitStart   bool
		server      bool
		plugin      string
		predecessor string
		wantTotal   string
		wantDelta   string
	}{
		{name: "valid plugin", start: start, end: end, wantTotal: "10m0s", wantDelta: "10m0s"},
		{name: "zero elapsed", start: end, end: end, wantTotal: "0s", wantDelta: "0s"},
		{name: "skipped upgrade without POST", omitStart: true, end: end},
		{name: "empty start", end: end},
		{name: "invalid start", start: "invalid", end: end},
		{name: "empty end", start: start},
		{name: "invalid end", start: start, end: "invalid"},
		{name: "both empty"},
		{name: "both invalid", start: "invalid", end: "invalid"},
		{name: "missing predecessor falls back to own start", plugin: pluginName99, start: start, end: end, wantTotal: "10m0s", wantDelta: "10m0s"},
		{name: "missing predecessor and own start", plugin: pluginName99, omitStart: true, end: end},
		{name: "valid predecessor", plugin: pluginName10, predecessor: "2026-10-21T15:29:43Z", start: start, end: end, wantTotal: "10m0s", wantDelta: "5m0s"},
		{name: "valid predecessor without own start", plugin: pluginName10, predecessor: start, omitStart: true, end: end, wantDelta: "10m0s"},
		{name: "invalid predecessor", plugin: pluginName10, predecessor: "invalid", start: start, end: end, wantTotal: "10m0s"},
		{name: "valid server", server: true, start: start, end: end, wantTotal: "10m0s"},
		{name: "server without start", server: true, omitStart: true, end: end},
		{name: "server invalid start", server: true, start: "invalid", end: end},
		{name: "server empty end", server: true, start: start},
		{name: "server invalid end", server: true, start: start, end: "invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			plugin := test.plugin
			if plugin == "" {
				plugin = pluginName05
			}
			var logs []string
			want := []*RuntimeInfoItem{}
			appendLog := func(item MetaLogItem) {
				t.Helper()
				raw, err := json.Marshal(item)
				if err != nil {
					t.Fatalf("marshal meta log: %v", err)
				}
				logs = append(logs, string(raw))
			}
			if test.predecessor != "" {
				appendLog(MetaLogItem{Method: "PUT", PluginName: pluginName05, Time: test.predecessor})
				want = append(want, &RuntimeInfoItem{Name: "plugin finished " + pluginName05, Time: test.predecessor})
			}
			if !test.omitStart {
				item := MetaLogItem{Method: "POST", Message: "received request", PluginName: plugin, Time: test.start}
				name := "plugin started " + plugin
				if test.server {
					item = MetaLogItem{Message: "Starting server Expected Results: ...", Time: test.start}
					name = "server started"
				}
				appendLog(item)
				want = append(want, &RuntimeInfoItem{Name: name, Time: test.start})
			}
			item := MetaLogItem{Method: "PUT", PluginName: plugin, Time: test.end}
			name := "plugin finished " + plugin
			if test.server {
				item = MetaLogItem{Message: "Invoking plugin cleanup", Time: test.end}
				name = "server finished"
			}
			appendLog(item)
			want = append(want, &RuntimeInfoItem{Name: name, Time: test.end, Total: test.wantTotal, Delta: test.wantDelta})
			if diff := cmp.Diff(want, ParseMetaLogs(logs)); diff != "" {
				t.Errorf("ParseMetaLogs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
