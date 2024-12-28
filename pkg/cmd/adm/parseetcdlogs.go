package adm

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
	mg "github.com/redhat-openshift-ecosystem/opct/internal/openshift/mustgather"
	"github.com/spf13/cobra"
)

type parseEtcdLogsInput struct {
	aggregator        string
	skipErrorCounters bool
}

var parseEtcdLogsArgs parseEtcdLogsInput
var parseEtcdLogsCmd = &cobra.Command{
	Use:     "parse-etcd-logs",
	Example: "opct adm parse-etcd-logs --aggregator hour",
	Short:   "Parse ETCD logs.",
	Run:     parseEtcdLogsRun,
}

func init() {
	parseEtcdLogsCmd.Flags().StringVar(&parseEtcdLogsArgs.aggregator, "aggregator", "hour", "Aggregator. Valid: all, day, hour, minute. Default: all")
	parseEtcdLogsCmd.Flags().BoolVar(&parseEtcdLogsArgs.skipErrorCounters, "skip-error-counter", false, "Skip calculation of error counter. Increase speed. Default: false")
}

func printTable(table [][]string) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 0, '\t', 0)
	for _, row := range table {
		for _, col := range row {
			fmt.Fprintf(writer, "%s\t", col)
		}
		fmt.Fprintf(writer, "\n")
	}
	writer.Flush()
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func parseEtcdLogsRun(cmd *cobra.Command, args []string) {

	errCounters := &archive.ErrorCounter{}
	filterATTL := mg.NewFilterApplyTookTooLong(parseEtcdLogsArgs.aggregator)

	// when must-gather directory is provided as argument
	if len(args) > 0 {
		log.Printf("Processing logs from directory %s...\n", args[0])
		reEtcdLog := regexp.MustCompile(`(\/namespaces\/openshift-etcd\/pods\/.*\/etcd\/etcd\/logs\/.*.log)`)
		err := filepath.Walk(args[0],
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !reEtcdLog.MatchString(path) {
					return nil
				}

				log.Debugf("Processing etcd log file: %s", path)
				dat, err := os.ReadFile(path)
				check(err)

				for _, line := range strings.Split(string(dat), "\n") {
					filterATTL.ProcessLine(line)
					if !parseEtcdLogsArgs.skipErrorCounters {
						lineErrCounter := archive.NewErrorCounter(&line, mg.EtcdLogErrorPatterns)
						errCounters = archive.MergeErrorCounters(errCounters, &lineErrCounter)
					}
				}
				log.Debugf("etcd log processed: %s", path)
				return nil
			})
		if err != nil {
			log.Errorf("One or more errors when reading from directory: %v", err)
			os.Exit(1)
		}

	} else {
		log.Println("Processing logs from stdin...")
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			line := s.Text()
			filterATTL.ProcessLine(line)
			if !parseEtcdLogsArgs.skipErrorCounters {
				lineErrCounter := archive.NewErrorCounter(&line, mg.EtcdLogErrorPatterns)
				errCounters = archive.MergeErrorCounters(errCounters, &lineErrCounter)
			}
		}
	}

	stat := filterATTL.GetStat(0)

	fmt.Printf("= Filter Name: %s =\n", filterATTL.Name)
	fmt.Printf("== Group by: %s ==\n", filterATTL.GroupBy)

	fmtCol := func(col string) string {
		return fmt.Sprintf("%-13s", col)
	}

	tbSummary := [][]string{{fmtCol("ID"), fmtCol("COUNT"), fmtCol(">=500ms"), fmtCol(">=1s"), fmtCol("Max(ms)")}}

	tbBuckets := [][]string{{fmtCol("ID"), fmtCol("COUNT"),
		fmtCol(mg.BucketRangeName200Ms),
		fmtCol(mg.BucketRangeName300Ms),
		fmtCol(mg.BucketRangeName400Ms),
		fmtCol(mg.BucketRangeName500Ms),
		fmtCol(mg.BucketRangeName600Ms),
		fmtCol(mg.BucketRangeName700Ms),
		fmtCol(mg.BucketRangeName800Ms),
		fmtCol(mg.BucketRangeName900Ms),
		fmtCol(">=1s")}}

	tbTimers := [][]string{{fmtCol("ID"), fmtCol("COUNT"), fmtCol("MIN"), fmtCol("AVG"),
		fmtCol("MAX"), fmtCol("P99"), fmtCol("P99.9"), fmtCol("P90"), fmtCol("StdDev")}}

	groups := make([]string, 0, len(stat))
	for k := range stat {
		groups = append(groups, k)
	}
	sort.Strings(groups)
	for _, gk := range groups {

		tbSummary = append(tbSummary, []string{fmtCol(gk),
			fmtCol(fmt.Sprintf("%d", stat[gk].RequestCount)),
			fmtCol(stat[gk].Higher500ms),
			fmtCol(stat[gk].Buckets[mg.BucketRangeName1000Inf]),
			fmtCol(stat[gk].StatMax)})

		tbBuckets = append(tbBuckets, []string{fmtCol(gk),
			fmtCol(fmt.Sprintf("%d", stat[gk].RequestCount)),
			fmtCol(stat[gk].Buckets[mg.BucketRangeName200Ms]),
			fmtCol(stat[gk].Buckets[mg.BucketRangeName300Ms]),
			fmtCol(stat[gk].Buckets[mg.BucketRangeName400Ms]),
			fmtCol(stat[gk].Buckets[mg.BucketRangeName500Ms]),
			fmtCol(stat[gk].Buckets[mg.BucketRangeName600Ms]),
			fmtCol(stat[gk].Buckets[mg.BucketRangeName700Ms]),
			fmtCol(stat[gk].Buckets[mg.BucketRangeName800Ms]),
			fmtCol(stat[gk].Buckets[mg.BucketRangeName900Ms]),
			fmtCol(stat[gk].Buckets[mg.BucketRangeName1000Inf])})

		tbTimers = append(tbTimers, []string{fmtCol(gk), fmtCol(stat[gk].StatCount),
			fmtCol(stat[gk].StatMin), fmtCol(stat[gk].StatMedian),
			fmtCol(stat[gk].StatMax), fmtCol(stat[gk].StatPerc99),
			fmtCol(stat[gk].StatPerc999), fmtCol(stat[gk].StatPerc90),
			fmtCol(stat[gk].StatStddev),
		})
	}

	fmt.Printf("\n=== Summary ===\n")
	printTable(tbSummary)

	fmt.Printf("\n=== Buckets (ms) ===\n")
	printTable(tbBuckets)

	fmt.Printf("\n=== Timers ===\n")
	printTable(tbTimers)

	if !parseEtcdLogsArgs.skipErrorCounters {
		fmt.Printf("\n=== Log error counters ===\n")
		tbErrors := [][]string{{
			fmt.Sprintf("%-60s", "ERROR PATTERN"),
			fmt.Sprintf("%-s", "COUNTER"),
		}}
		for k, v := range *errCounters {
			tbErrors = append(tbErrors, []string{
				fmt.Sprintf("%-60s", k),
				fmt.Sprintf(": %d", v)})
		}
		printTable(tbErrors)
	}
}
