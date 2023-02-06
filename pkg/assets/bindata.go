// Code generated for package assets by go-bindata DO NOT EDIT. (@generated)
// sources:
// manifests/openshift-artifacts-collector.yaml
// manifests/openshift-cluster-upgrade.yaml
// manifests/openshift-conformance-validated.yaml
// manifests/openshift-kube-conformance.yaml
package assets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)
type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _manifestsOpenshiftArtifactsCollectorYaml = []byte(`podSpec:
  restartPolicy: Never
  serviceAccountName: sonobuoy-serviceaccount
  volumes:
    - name: shared
      emptyDir: {}
  containers:
    - name: report-progress
      image: quay.io/ocp-cert/openshift-tests-provider-cert:v0.3.0
      imagePullPolicy: Always
      priorityClassName: system-node-critical
      command: ["./report-progress.sh"]
      volumeMounts:
      - mountPath: /tmp/sonobuoy/results
        name: results
      - mountPath: /tmp/shared
        name: shared
      env:
        - name: PLUGIN_ID
          value: "99"
        - name: ENV_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: ENV_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: ENV_POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
sonobuoy-config:
  driver: Job
  plugin-name: 99-openshift-artifacts-collector
  result-format: raw
  description: The OpenShift Provider Certification Tool artifacts collector executed on the post-certification.
  source-url: https://github.com/redhat-openshift-ecosystem/provider-certification-tool/blob/main/manifests/openshift-artifacts-collector.yaml
  skipCleanup: true
spec:
  name: plugin
  image: quay.io/ocp-cert/openshift-tests-provider-cert:v0.3.0
  imagePullPolicy: Always
  volumeMounts:
  - mountPath: /tmp/sonobuoy/results
    name: results
  - mountPath: /tmp/shared
    name: shared
  env:
    - name: PLUGIN_ID
      value: "99"
    - name: ENV_NODE_NAME
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
    - name: ENV_POD_NAME
      valueFrom:
        fieldRef:
          fieldPath: metadata.name
    - name: ENV_POD_NAMESPACE
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace
    - name: RUN_MODE
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: run-mode
    - name: UPGRADE_RELEASES
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: upgrade-target-images
    - name: MIRROR_IMAGE_REPOSITORY
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: mirror-registry
          optional: true

`)

func manifestsOpenshiftArtifactsCollectorYamlBytes() ([]byte, error) {
	return _manifestsOpenshiftArtifactsCollectorYaml, nil
}

func manifestsOpenshiftArtifactsCollectorYaml() (*asset, error) {
	bytes, err := manifestsOpenshiftArtifactsCollectorYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/openshift-artifacts-collector.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsOpenshiftClusterUpgradeYaml = []byte(`podSpec:
  restartPolicy: Never
  serviceAccountName: sonobuoy-serviceaccount
  volumes:
    - name: shared
      emptyDir: {}
  containers:
    - name: report-progress
      image: quay.io/ocp-cert/openshift-tests-provider-cert:v0.3.0
      imagePullPolicy: Always
      priorityClassName: system-node-critical
      command: ["./report-progress.sh"]
      volumeMounts:
      - mountPath: /tmp/sonobuoy/results
        name: results
      - mountPath: /tmp/shared
        name: shared
      env:
        - name: PLUGIN_ID
          value: "05"
        - name: ENV_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: ENV_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: ENV_POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
sonobuoy-config:
  driver: Job
  plugin-name: 05-openshift-cluster-upgrade
  result-format: junit
  description: The end-to-end tests maintained by OpenShift to certify the Provider running the OpenShift Container Platform.
  source-url: https://github.com/redhat-openshift-ecosystem/provider-certification-tool/blob/main/manifests/openshift-conformance-validated.yaml
  skipCleanup: true
spec:
  name: plugin
  image: quay.io/ocp-cert/openshift-tests-provider-cert:v0.3.0
  imagePullPolicy: Always
  priorityClassName: system-node-critical
  volumeMounts:
  - mountPath: /tmp/sonobuoy/results
    name: results
  - mountPath: /tmp/shared
    name: shared
  env:
    - name: PLUGIN_ID
      value: "05"
    - name: ENV_NODE_NAME
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
    - name: ENV_POD_NAME
      valueFrom:
        fieldRef:
          fieldPath: metadata.name
    - name: ENV_POD_NAMESPACE
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace
    - name: UPGRADE_RELEASES
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: upgrade-target-images
    - name: RUN_MODE
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: run-mode
    - name: MIRROR_IMAGE_REPOSITORY
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: mirror-registry
          optional: true

`)

func manifestsOpenshiftClusterUpgradeYamlBytes() ([]byte, error) {
	return _manifestsOpenshiftClusterUpgradeYaml, nil
}

func manifestsOpenshiftClusterUpgradeYaml() (*asset, error) {
	bytes, err := manifestsOpenshiftClusterUpgradeYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/openshift-cluster-upgrade.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsOpenshiftConformanceValidatedYaml = []byte(`podSpec:
  restartPolicy: Never
  serviceAccountName: sonobuoy-serviceaccount
  volumes:
    - name: shared
      emptyDir: {}
  containers:
    - name: report-progress
      image: quay.io/ocp-cert/openshift-tests-provider-cert:v0.3.0
      imagePullPolicy: Always
      priorityClassName: system-node-critical
      command: ["./report-progress.sh"]
      volumeMounts:
      - mountPath: /tmp/sonobuoy/results
        name: results
      - mountPath: /tmp/shared
        name: shared
      env:
        - name: PLUGIN_ID
          value: "20"
        - name: ENV_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: ENV_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: ENV_POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
sonobuoy-config:
  driver: Job
  plugin-name: 20-openshift-conformance-validated
  result-format: junit
  description: The end-to-end tests maintained by OpenShift to certify the Provider running the OpenShift Container Platform.
  source-url: https://github.com/redhat-openshift-ecosystem/provider-certification-tool/blob/main/manifests/openshift-conformance-validated.yaml
  skipCleanup: true
spec:
  name: plugin
  image: quay.io/ocp-cert/openshift-tests-provider-cert:v0.3.0
  imagePullPolicy: Always
  priorityClassName: system-node-critical
  volumeMounts:
  - mountPath: /tmp/sonobuoy/results
    name: results
  - mountPath: /tmp/shared
    name: shared
  env:
    - name: PLUGIN_ID
      value: "20"
    - name: ENV_NODE_NAME
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
    - name: ENV_POD_NAME
      valueFrom:
        fieldRef:
          fieldPath: metadata.name
    - name: ENV_POD_NAMESPACE
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace
    - name: RUN_MODE
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: run-mode
    - name: DEV_MODE_COUNT
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: dev-count
    - name: MIRROR_IMAGE_REPOSITORY
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: mirror-registry
          optional: true
`)

func manifestsOpenshiftConformanceValidatedYamlBytes() ([]byte, error) {
	return _manifestsOpenshiftConformanceValidatedYaml, nil
}

func manifestsOpenshiftConformanceValidatedYaml() (*asset, error) {
	bytes, err := manifestsOpenshiftConformanceValidatedYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/openshift-conformance-validated.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsOpenshiftKubeConformanceYaml = []byte(`podSpec:
  restartPolicy: Never
  serviceAccountName: sonobuoy-serviceaccount
  volumes:
    - name: shared
      emptyDir: {}
  containers:
    - name: report-progress
      image: quay.io/ocp-cert/openshift-tests-provider-cert:v0.3.0
      imagePullPolicy: Always
      priorityClassName: system-node-critical
      command: ["./report-progress.sh"]
      volumeMounts:
      - mountPath: /tmp/sonobuoy/results
        name: results
      - mountPath: /tmp/shared
        name: shared
      env:
        - name: PLUGIN_ID
          value: "10"
        - name: ENV_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: ENV_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: ENV_POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
sonobuoy-config:
  driver: Job
  plugin-name: 10-openshift-kube-conformance
  result-format: junit
  description: The end-to-end tests maintained by Kubernetes to certify the platform.
  source-url: https://github.com/redhat-openshift-ecosystem/provider-certification-tool/blob/main/manifests/openshift-kube-conformance.yaml
  skipCleanup: true
spec:
  name: plugin
  image: quay.io/ocp-cert/openshift-tests-provider-cert:v0.3.0
  imagePullPolicy: Always
  priorityClassName: system-node-critical
  volumeMounts:
  - mountPath: /tmp/sonobuoy/results
    name: results
  - mountPath: /tmp/shared
    name: shared
  env:
    - name: PLUGIN_ID
      value: "10"
    - name: ENV_NODE_NAME
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
    - name: ENV_POD_NAME
      valueFrom:
        fieldRef:
          fieldPath: metadata.name
    - name: ENV_POD_NAMESPACE
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace
    - name: RUN_MODE
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: run-mode
    - name: DEV_MODE_COUNT
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: dev-count
    - name: MIRROR_IMAGE_REPOSITORY
      valueFrom:
        configMapKeyRef:
          name: plugins-config
          key: mirror-registry
          optional: true
`)

func manifestsOpenshiftKubeConformanceYamlBytes() ([]byte, error) {
	return _manifestsOpenshiftKubeConformanceYaml, nil
}

func manifestsOpenshiftKubeConformanceYaml() (*asset, error) {
	bytes, err := manifestsOpenshiftKubeConformanceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/openshift-kube-conformance.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"manifests/openshift-artifacts-collector.yaml":   manifestsOpenshiftArtifactsCollectorYaml,
	"manifests/openshift-cluster-upgrade.yaml":       manifestsOpenshiftClusterUpgradeYaml,
	"manifests/openshift-conformance-validated.yaml": manifestsOpenshiftConformanceValidatedYaml,
	"manifests/openshift-kube-conformance.yaml":      manifestsOpenshiftKubeConformanceYaml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"manifests": &bintree{nil, map[string]*bintree{
		"openshift-artifacts-collector.yaml":   &bintree{manifestsOpenshiftArtifactsCollectorYaml, map[string]*bintree{}},
		"openshift-cluster-upgrade.yaml":       &bintree{manifestsOpenshiftClusterUpgradeYaml, map[string]*bintree{}},
		"openshift-conformance-validated.yaml": &bintree{manifestsOpenshiftConformanceValidatedYaml, map[string]*bintree{}},
		"openshift-kube-conformance.yaml":      &bintree{manifestsOpenshiftKubeConformanceYaml, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
