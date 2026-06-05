package cleaner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"regexp"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/ptr"
)

type PatchRule struct {
	JSONPatch    *string
	RegexPattern *regexp.Regexp
	KeepCount    uint64
	Count        uint64
}

var (
	// JSONPatchRules is a map with the paths to files in the archive to apply RFC6902 JSON patches.
	JSONPatchRules = map[string]*PatchRule{
		"resources/cluster/machineconfiguration.openshift.io_v1_controllerconfigs.json": &PatchRule{
			JSONPatch: ptr.To[string](`[
					{
						"op": "replace",
						"path": "/items/0/spec/internalRegistryPullSecret",
						"value": "REDACTED"
					}
				]`,
			),
		},
	}

	// RemoveFilePatternRules is a map with regular expressions to remove files in the result archive.
	RemoveFilePatternRules = map[string]*PatchRule{
		"packages.operators.coreos.com_v1_packagemanifests.json": &PatchRule{
			RegexPattern: regexp.MustCompile("resources/ns/.*/packages.operators.coreos.com_v1_packagemanifests.json"),
			// Keeping at least one object for auditing.
			KeepCount: 1,
			Count:     0,
		},
	}
)

// ScanPatchTarGzipReaderFor scans and patches the artifact stream, returning the cleaned artifact.
func ScanPatchTarGzipReaderFor(r io.Reader) (resp io.Reader, size int, err error) {
	log.Debug("Scanning the artifact for patches...")
	size = 0

	// Create a gzip reader
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, size, fmt.Errorf("unable to open gzip file: %w", err)
	}
	defer gzipReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzipReader)

	// Create a buffer to store the updated tar.gz content
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)
	var leakFindings []LeakFinding

	// Process the tar headers
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, size, fmt.Errorf("unable to process file in archive: %w", err)
		}

		findings, procErr := processTarHeader(header, tarReader, tarWriter)
		if procErr != nil {
			return nil, size, procErr
		}
		leakFindings = append(leakFindings, findings...)
	}

	if len(leakFindings) > 0 {
		log.Warnf("Leak scan: %d potential finding(s) detected", len(leakFindings))
		for _, f := range leakFindings {
			if f.Line > 0 {
				log.Warnf("  %s:%d — %s", f.File, f.Line, f.Pattern)
			} else {
				log.Warnf("  %s — %s", f.File, f.Pattern)
			}
		}
	}

	// Close the writers
	if err := tarWriter.Close(); err != nil {
		return nil, size, fmt.Errorf("closing tarball: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, size, fmt.Errorf("closing gzip: %w", err)
	}

	// Return the updated tar.gz content as an io.Reader
	size = len(buf.Bytes())
	return bytes.NewReader(buf.Bytes()), size, nil
}

// processTarHeader processes the tar header and applies patches or removes files as needed.
// Returns any leak findings detected in the file content.
func processTarHeader(header *tar.Header, tarReader *tar.Reader, tarWriter *tar.Writer) ([]LeakFinding, error) {
	// Processing pre-defined patches, including recursively archives inside base.
	if _, ok := JSONPatchRules[header.Name]; ok {
		log.Debugf("Patch pattern matched for: %s", header.Name)
		if strings.HasSuffix(header.Name, ".json") {
			desiredFile, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("unable to read file in archive: %w", err)
			}
			patchedFile, err := applyJSONPatch(header.Name, desiredFile)
			if err != nil {
				return nil, fmt.Errorf("unable to apply patch to file %s: %w", header.Name, err)
			}
			header.Size = int64(len(patchedFile))
			log.Debugf("File %s size %d bytes", header.Name, header.Size)
			if err := tarWriter.WriteHeader(header); err != nil {
				return nil, fmt.Errorf("unable to write file header to new archive: %w", err)
			}
			if _, err := tarWriter.Write(patchedFile); err != nil {
				return nil, fmt.Errorf("unable to write file data to new archive: %w", err)
			}
			findings := ScanContentForLeaks(header.Name, patchedFile)
			return findings, nil
		}
		log.Debugf("Unknown extension, skipping patch for file %s", header.Name)
		return nil, nil
	}

	if strings.HasSuffix(header.Name, ".tar.gz") {
		log.Debugf("Scanning tarball archive: %s", header.Name)
		resp, size, err := ScanPatchTarGzipReaderFor(tarReader)
		if err != nil {
			return nil, fmt.Errorf("unable to apply patch to file %s: %w", header.Name, err)
		}
		header.Size = int64(size)
		archiveBuf := new(bytes.Buffer)
		if _, err = io.Copy(archiveBuf, resp); err != nil {
			return nil, err
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("unable to write file header to new archive: %w", err)
		}
		if _, err := tarWriter.Write(archiveBuf.Bytes()); err != nil {
			return nil, fmt.Errorf("unable to write file data to new archive: %w", err)
		}
		return nil, nil
	}

	// Check removal rules
	for _, rule := range RemoveFilePatternRules {
		if rule.RegexPattern.MatchString(header.Name) {
			if rule.Count >= rule.KeepCount {
				log.Debugf("Skipping file %s due to matching pattern rules", header.Name)
				return nil, nil
			}
			rule.Count++
		}
	}

	// Write header first
	if err := tarWriter.WriteHeader(header); err != nil {
		return nil, fmt.Errorf("error streaming file header to new archive: %w", err)
	}

	// For large files (>10MB), stream directly without scanning
	if header.Size > int64(maxLeakScanSize) {
		if _, err := io.Copy(tarWriter, tarReader); err != nil {
			return nil, fmt.Errorf("error streaming large file data to new archive: %w", err)
		}
		return nil, nil
	}

	// For smaller files, read into memory for scanning, then write
	content, err := io.ReadAll(tarReader)
	if err != nil {
		return nil, fmt.Errorf("error reading file data from archive: %w", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		return nil, fmt.Errorf("error streaming file data to new archive: %w", err)
	}

	findings := ScanContentForLeaks(header.Name, content)
	return findings, nil
}

// applyJSONPatch applies hardcoded patches to the stream, returning the cleaned file.
func applyJSONPatch(filepath string, data []byte) ([]byte, error) {
	if _, ok := JSONPatchRules[filepath]; !ok {
		return nil, fmt.Errorf("no patch rule for file: %s", filepath)
	}
	patch, err := jsonpatch.DecodePatch([]byte(*JSONPatchRules[filepath].JSONPatch))
	if err != nil {
		return nil, fmt.Errorf("decoding patch: %w", err)
	}

	modified, err := patch.Apply(data)
	if err != nil {
		return nil, fmt.Errorf("applying patch: %w", err)
	}

	return modified, nil
}
