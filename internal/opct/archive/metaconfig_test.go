/*
Handle items in the file path meta/config.json
*/
package archive

import (
	"encoding/json"
	"log"
	"reflect"
	"testing"

	opcttests "github.com/redhat-openshift-ecosystem/opct/test"
)

var testDataMetaConfig MetaConfigSonobuoy

func loadTestDataMetaConfig() {
	file := "testdata/archive-001/meta/config.json"
	metaFile, err := opcttests.TestData.ReadFile(file)
	if err != nil {
		log.Fatalf("unable to load test data %s: %v", file, err)
	}
	if err := json.Unmarshal(metaFile, &testDataMetaConfig); err != nil {
		log.Fatalf("unable to parse test data %s: %v", file, err)
	}
}

func TestParseMetaConfig(t *testing.T) {

	loadTestDataMetaConfig()

	type args struct {
		cfg *MetaConfigSonobuoy
	}
	tests := []struct {
		name string
		args args
		want []*RuntimeInfoItem
	}{
		{
			name: "process meta config.json",
			args: args{
				cfg: &testDataMetaConfig,
			},
			want: []*RuntimeInfoItem{
				{
					Name:  "UUID",
					Value: testDataMetaConfig.UUID,
				},
				{
					Name:  "Version",
					Value: testDataMetaConfig.Version,
				},
				{
					Name:  "ResultsDir",
					Value: testDataMetaConfig.ResultsDir,
				},
				{
					Name:  "Namespace",
					Value: testDataMetaConfig.Namespace,
				},
				{
					Name:  "WorkerImage",
					Value: testDataMetaConfig.WorkerImage,
				},
				{
					Name:  "ImagePullPolicy",
					Value: testDataMetaConfig.ImagePullPolicy,
				},
				{
					Name:  "AggregatorPermissions",
					Value: testDataMetaConfig.AggregatorPermissions,
				},
				{
					Name:  "ServiceAccountName",
					Value: testDataMetaConfig.ServiceAccountName,
				},
				{
					Name:  "ExistingServiceAccount",
					Value: "yes",
				},
				{
					Name:  "SecurityContextMode",
					Value: testDataMetaConfig.SecurityContextMode,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseMetaConfig(tt.args.cfg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseMetaConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
