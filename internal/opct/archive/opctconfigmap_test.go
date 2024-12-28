/*
Handle items in the file path resources/ns/{opct_namespace}/core_v1_configmaps.json
*/
package archive

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"testing"

	opcttests "github.com/redhat-openshift-ecosystem/opct/test"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseOpctConfig(t *testing.T) {

	testFile := "testdata/archive-001/resources/ns/openshift-provider-certification/core_v1_configmaps.json"
	raw, err := opcttests.TestData.ReadFile(testFile)
	if err != nil {
		log.Fatalf("unable to load test data %s: %v", testFile, err)
	}
	var testDataConfigMaps v1.ConfigMapList
	err = json.Unmarshal(raw, &testDataConfigMaps)
	if err != nil {
		log.Fatalf("unable to unmarshal config map %s: %v", testFile, err)
	}

	type args struct {
		cms *v1.ConfigMapList
	}
	tests := []struct {
		name string
		args args
		want []*RuntimeInfoItem
	}{
		{
			name: "nil config",
			args: args{
				cms: nil,
			},
			want: []*RuntimeInfoItem{},
		},
		{
			name: "empty config",
			args: args{
				cms: &v1.ConfigMapList{},
			},
			want: []*RuntimeInfoItem{},
		},
		{
			name: "config not found",
			args: args{
				cms: &v1.ConfigMapList{Items: []v1.ConfigMap{v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "unknown-namespace",
					},
				}}},
			},
			want: []*RuntimeInfoItem{},
		},
		{
			name: "load and parse valid config",
			args: args{
				cms: &testDataConfigMaps,
			},
			want: []*RuntimeInfoItem{
				{
					Name:   "cli-commit",
					Value:  "20d1405",
					Config: "openshift-provider-certification-version",
				},
				{
					Name:   "cli-version",
					Value:  "1.0.0",
					Config: "openshift-provider-certification-version",
				},
				{
					Name:   "sonobuoy-image",
					Value:  "quay.io/ocp-cert/sonobuoy:v0.56.10",
					Config: "openshift-provider-certification-version",
				},
				{
					Name:   "sonobuoy-version",
					Value:  "v0.56.10",
					Config: "openshift-provider-certification-version",
				},
				{
					Name:   "dev-count",
					Value:  "0",
					Config: "plugins-config",
				},
				{
					Name:   "run-mode",
					Value:  "regular",
					Config: "plugins-config",
				},
				{
					Name:   "upgrade-target-images",
					Value:  "",
					Config: "plugins-config",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseOpctConfig(tt.args.cms); !reflect.DeepEqual(got, tt.want) {
				gotStr := ""
				wantStr := ""
				for _, s := range got {
					gotStr += fmt.Sprintf("%s,", s)
				}
				for _, s := range tt.want {
					wantStr += fmt.Sprintf("%s,", s)
				}
				t.Errorf("ParseOpctConfig() => \ngot: %v \nwant: %v", gotStr, wantStr)
			}
		})
	}
}
