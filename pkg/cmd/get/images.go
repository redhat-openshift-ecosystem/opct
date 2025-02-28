package get

import (
	"fmt"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
	"github.com/spf13/cobra"
)

type imageOptions struct {
	ToRepository string
}

var options *imageOptions

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Print images used by OPCT.",
	Run:   runGetImages,
}

func init() {
	options = new(imageOptions)
	imagesCmd.Flags().StringVar(&options.ToRepository, "to-repository", "", "Show images with format to mirror to repository. Example: registry.example.io:5000")
}

func generateImage(repo, name string) string {
	if options.ToRepository == "" {
		return fmt.Sprintf("%s/%s", repo, name)
	} else {
		from := fmt.Sprintf("%s/%s", repo, name)
		to := fmt.Sprintf("%s/%s", options.ToRepository, name)
		return fmt.Sprintf("%s %s", from, to)
	}
}

func runGetImages(cmd *cobra.Command, args []string) {
	images := []string{}

	// Sonobuoy
	images = append(images, generateImage(pkg.DefaultToolsRepository, pkg.SonobuoyImage))

	// Plugins
	images = append(images, generateImage(pkg.DefaultToolsRepository, pkg.PluginsImage))
	images = append(images, generateImage(pkg.DefaultToolsRepository, pkg.CollectorImage))
	images = append(images, generateImage(pkg.DefaultToolsRepository, pkg.MustGatherMonitoringImage))

	// etcdfio
	img_etcdfio := "quay.io/openshift-scale/etcd-perf:latest"
	if options.ToRepository == "" {
		images = append(images, img_etcdfio)
	} else {
		to := fmt.Sprintf("%s/%s", options.ToRepository, "etcd-perf:latest")
		images = append(images, fmt.Sprintf("%s %s", img_etcdfio, to))
	}

	// test's specific images (not related with OPCT)
	img_e2epause := "registry.k8s.io/pause:3.8"
	if options.ToRepository == "" {
		images = append(images, img_e2epause)
	} else {
		to := fmt.Sprintf("%s/%s", options.ToRepository, "opct:e2e-28-registry-k8s-io-pause-3-8-aP7uYsw5XCmoDy5W")
		images = append(images, fmt.Sprintf("%s %s", img_e2epause, to))
	}

	for _, image := range images {
		fmt.Println(image)
	}
}
