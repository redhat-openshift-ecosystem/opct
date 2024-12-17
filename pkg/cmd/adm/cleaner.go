package adm

import (
	"bufio"
	"io"
	"os"

	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/cleaner"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type cleanerArguments struct {
	input  string
	output string
}

var cleanerArgs cleanerArguments
var cleanerCmd = &cobra.Command{
	Use:     "cleaner",
	Example: "opct adm cleaner --input ./results.tar.gz --output ./results-cleaned.tar.gz",
	Short:   "Utility to apply pre-defined patches to existing result archive.",
	Run:     cleanerRun,
}

func init() {
	cleanerCmd.Flags().StringVar(&cleanerArgs.input, "input", "", "Input archive file. Example: ./opct-xyz.tar.gz")
	cleanerCmd.Flags().StringVar(&cleanerArgs.output, "output", "", "Output archive file. Example: ./opct-cleaned.tar.gz")
}

func cleanerRun(cmd *cobra.Command, args []string) {

	if cleanerArgs.input == "" {
		log.Error("missing argumet --input <archive.tar.gz>")
		os.Exit(1)
	}

	if cleanerArgs.output == "" {
		log.Error("missing argumet --output <new-archive.tar.gz>")
		os.Exit(1)
	}

	log.Infof("Starting artifact cleaner for %s", cleanerArgs.input)

	fin, err := os.Open(cleanerArgs.input)
	if err != nil {
		panic(err)
	}

	// close fi on exit and check for its returned error
	defer func() {
		if err := fin.Close(); err != nil {
			panic(err)
		}
	}()

	r := bufio.NewReader(fin)

	// scanning for sensitive data
	cleaned, _, err := cleaner.ScanPatchTarGzipReaderFor(r)
	if err != nil {
		panic(err)
	}

	// Create a new file
	file, err := os.Create(cleanerArgs.output)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Write the cleaned data to the file
	_, err = io.Copy(file, cleaned)
	if err != nil {
		panic(err)
	}

	log.Infof("Data successfully written to %s", cleanerArgs.output)
}
