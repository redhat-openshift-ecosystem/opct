package cleaner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
)

var patches = map[string]string{
	"resources/cluster/machineconfiguration.openshift.io_v1_controllerconfigs.json": `[
        {
            "op": "replace",
            "path": "/items/0/spec/internalRegistryPullSecret",
            "value": "REDACTED"
        }
    ]`,
}

// ScanPatchTarGzipReaderFor scan for patches the artifact stream, returning
// the cleaned artifact.
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

	// Find and process the desired file
	var desiredFile []byte
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, size, fmt.Errorf("unable to process file in archive: %w", err)
		}

		// Processing pre-defined patches, including recursively archives inside base.
		if _, ok := patches[header.Name]; ok {
			// Once the pre-defined/hardcoded patch matches with file stream, apply
			// the path according to the extension. Currently only JSON patches
			// are supported.
			log.Debugf("Patch pattern matched for: %s", header.Name)
			if strings.HasSuffix(header.Name, ".json") {
				var patchedFile []byte
				desiredFile, err = io.ReadAll(tarReader)
				if err != nil {
					log.Errorf("unable to read file in archive: %v", err)
					return nil, size, fmt.Errorf("unable to read file in archive: %w", err)
				}

				// Apply JSON patch to the file
				patchedFile, err = applyJSONPatch(header.Name, desiredFile)
				if err != nil {
					log.Errorf("unable to apply patch to file %s: %v", header.Name, err)
					return nil, size, fmt.Errorf("unable to apply patch to file %s: %w", header.Name, err)
				}

				// Update the file size in the header
				header.Size = int64(len(patchedFile))
				log.Debugf("Patched %d bytes", header.Size)

				// Write the updated file to stream.
				if err := tarWriter.WriteHeader(header); err != nil {
					log.Errorf("unable to write file header to new archive: %v", err)
					return nil, size, fmt.Errorf("unable to write file header to new archive: %w", err)
				}
				if _, err := tarWriter.Write(patchedFile); err != nil {
					log.Errorf("unable to write file data to new archive: %v", err)
					return nil, size, fmt.Errorf("unable to write file data to new archive: %w", err)
				}
			} else {
				log.Debugf("unknown extension, skipping patch for file %s", header.Name)
			}

		} else if strings.HasSuffix(header.Name, ".tar.gz") {
			// recursively scan for .tar.gz files rewriting it back to the original archive.
			// by default sonobuoy writes a base archive, and the result(s) will be inside of
			// the base. So it is required to recursively scan archives to find required, hardcoded,
			// patches.
			log.Debugf("Scanning tarball archive: %s", header.Name)
			resp, size, err = ScanPatchTarGzipReaderFor(tarReader)
			if err != nil {
				return nil, size, fmt.Errorf("unable to apply patch to file %s: %w", header.Name, err)
			}

			// Update the file size in the header
			header.Size = int64(size)

			// Write the updated header and file content
			buf := new(bytes.Buffer)
			_, err := io.Copy(buf, resp)
			if err != nil {
				return nil, size, err
			}

			// write archive back to stream.
			if err := tarWriter.WriteHeader(header); err != nil {
				log.Errorf("unable to write file header to new archive: %v", err)
				return nil, size, fmt.Errorf("unable to write file header to new archive: %w", err)
			}
			if _, err := tarWriter.Write(buf.Bytes()); err != nil {
				log.Errorf("unable to write file data to new archive: %v", err)
				return nil, size, fmt.Errorf("unable to write file data to new archive: %w", err)
			}
		} else {
			// Copy other files as-is
			if err := tarWriter.WriteHeader(header); err != nil {
				return nil, size, fmt.Errorf("error streaming file header to new archive: %w", err)
			}
			if _, err := io.Copy(tarWriter, tarReader); err != nil {
				return nil, size, fmt.Errorf("error streaming file data to new archive: %w", err)
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

// applyJSONPatch apply hard coded patches to stream, returning the cleaned file.
func applyJSONPatch(filepath string, data []byte) ([]byte, error) {
	patch, err := jsonpatch.DecodePatch([]byte(patches[filepath]))
	if err != nil {
		return nil, fmt.Errorf("decoding patch: %w", err)
	}

	modified, err := patch.Apply(data)
	if err != nil {
		return nil, fmt.Errorf("applying patch: %w", err)
	}

	return modified, nil
}
