package adm

import (
	"fmt"
	"strings"

	opctapi "github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/api"
	"github.com/spf13/cobra"
)

type parseJUnitInput struct {
	skipFailed  bool
	skipPassed  bool
	skipSkipped bool
}

var parseJUnitArgs parseJUnitInput
var parseJUnitCmd = &cobra.Command{
	Use:     "parse-junit",
	Example: "opct adm parse-junit",
	Short:   "Parse JUnit file.",
	RunE:    parseJUnitRun,
}

func init() {
	parseJUnitCmd.Flags().BoolVar(&parseJUnitArgs.skipFailed, "skip-failed", false, "Skip printing on stdout the failed test names.")
	parseJUnitCmd.Flags().BoolVar(&parseJUnitArgs.skipPassed, "skip-passed", false, "Skip printing on stdout the passed test names.")
	parseJUnitCmd.Flags().BoolVar(&parseJUnitArgs.skipSkipped, "skip-skipped", false, "Skip printing on stdout the skipped test names.")
}

func parseJUnitRun(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide the path to the JUnit file")
	}

	junitFile := args[0]
	parser, err := opctapi.NewJUnitXMLParser(junitFile)
	if err != nil {
		return fmt.Errorf("error parsing JUnit file: %v", err)
	}

	// Printing summary
	fmt.Println("Summary:")
	fmt.Printf("- File: %s\n", parser.XMLFile)
	fmt.Printf("- Total: %d\n", parser.Counters.Total)
	fmt.Printf("- Pass: %d\n", parser.Counters.Pass)
	fmt.Printf("- Skipped: %d\n", parser.Counters.Skipped)
	fmt.Printf("- Failures: %d\n", parser.Counters.Failures)
	fmt.Println()
	fmt.Println("JUnit Attributes:")
	fmt.Printf("- XMLName: %v\n", parser.Parsed.XMLName)
	fmt.Printf("- Name: %s\n", parser.Parsed.Name)
	fmt.Printf("- Tests: %d\n", parser.Parsed.Tests)
	fmt.Printf("- Skipped: %d\n", parser.Parsed.Skipped)
	fmt.Printf("- Failures: %d\n", parser.Parsed.Failures)
	fmt.Printf("- Time: %s\n", parser.Parsed.Time)
	fmt.Printf("- Property: %v\n", parser.Parsed.Property)

	// Passed tests
	passed := []string{}
	skipped := []string{}
	for _, testcase := range parser.Cases {
		if testcase.Status == opctapi.TestStatusPass {
			passed = append(passed, testcase.Name)
		}
		if testcase.Status == opctapi.TestStatusSkipped {
			skipped = append(skipped, testcase.Name)
		}
	}

	if !parseJUnitArgs.skipPassed {
		fmt.Printf("\n#> Passed tests (%d): \n%s\n", len(passed), strings.Join(passed, "\n"))
	}
	if !parseJUnitArgs.skipFailed {
		fmt.Printf("\n#> Failed tests (%d): \n%s\n", len(parser.Failures), strings.Join(parser.Failures, "\n"))
	}
	if !parseJUnitArgs.skipSkipped {
		fmt.Printf("\n#> Skipped tests (%d): \n%s\n", len(skipped), strings.Join(skipped, "\n"))
	}
	return nil
}
