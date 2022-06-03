package assets

import (
	"os"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewCmdAssets() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "assets",
		Args:  cobra.MaximumNArgs(1),
		Short: "Save provider certification tool plugin assets to disk",
		Long:  `Saves the provider certification tool's plugin asset YAML files locally for troubleshooting with run command'`,
		Run: func(cmd *cobra.Command, args []string) {
			// Parse optional destination directory argument
			destinationDirectory, err := os.Getwd()
			if err != nil {
				log.Error(err)
				return
			}
			if len(args) == 1 {
				destinationDirectory = args[0]
				finfo, err := os.Stat(destinationDirectory)
				if err != nil {
					log.Error(err)
					return
				}
				if !finfo.IsDir() {
					log.Error("Save destination must be directory")
					return
				}
			}

			if force {
				log.Warn("Overwriting existing files while saving assets")
			}

			for _, asset := range AssetNames() {
				assetBytes := MustAsset(asset)
				asset = path.Base(asset) // Trim off leading "manifest/"
				destination := path.Join(destinationDirectory, asset)

				// Check before overwriting an existing file
				_, err := os.Stat(destination)
				if err != nil {
					if !os.IsNotExist(err) {
						// We care about all errors except when its "no such file or directory"
						log.WithError(err).Error("Error checking asset desination")
						return
					}
				} else if !force {
					// Force not enabled so exit
					log.Errorf("File already exists (overwrite with --force): %s", destination)
					return
				}

				// Save the asset
				err = os.WriteFile(destination, assetBytes, 0644)
				if err != nil {
					log.WithError(err).Errorf("Error writing asset %s", destination)
				}
				log.Infof("Asset %s saved to %s", asset, destination)
			}
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing files when saving assets to disk")

	return cmd
}
