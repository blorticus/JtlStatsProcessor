package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/blorticus-go/jtl"
)

// JtlStatsProcessor /path/to/jtl/file -o /path/to/summary/output/file -t /directory/to/write/timestamp/files

func main() {
	cliArgs, err := ProcessCommandLineOptions()
	DieIfError(err)

	jtlFile, err := os.Open(cliArgs.PathToJtlSourceCsvFile)
	DieIfError(err)

	jtlDataSource, dataRowsThatCannotBeProcessed, fatalError := jtl.NewDataSourceFromCsv(jtlFile)
	DieIfError(fatalError)

	LogAnyRowsThatCannotBeProcessed(dataRowsThatCannotBeProcessed)

	summarizer := jtl.NewSummarizerForDataSource(jtlDataSource)
	err = summarizer.PreComputeAggregateSummaryAndSummariesForColumns(jtl.Column.RequestURL, jtl.Column.ResponseCodeOrErrorMessage, jtl.Column.RequestBodySizeInBytes, jtl.Column.ResponseBytesReceived)
	DieIfError(err)

	if cliArgs.PathToDirectoryForTimestampFiles != "" {
		err := WriteTimestampFiles(cliArgs.PathToDirectoryForTimestampFiles, summarizer)
		DieIfError(err)
	}

	summaryText := GenerateSummaryOutputText(summarizer)

	if cliArgs.PathToSummaryOutputCsvFile != "" {
		err := WriteSummaryToFile(cliArgs.PathToSummaryOutputCsvFile, summaryText)
		DieIfError(err)
	} else {
		fmt.Print(summaryText)
	}
}

func DieIfError(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

type CommandLineArguments struct {
	PathToJtlSourceCsvFile           string
	PathToSummaryOutputCsvFile       string
	PathToDirectoryForTimestampFiles string
}

func ProcessCommandLineOptions() (*CommandLineArguments, error) {
	args := &CommandLineArguments{}

	flag.StringVar(&args.PathToSummaryOutputCsvFile, "o", "", "Path to file for summary output")
	flag.StringVar(&args.PathToDirectoryForTimestampFiles, "t", "", "Path to directory into which timestamp files should be written")

	flag.Parse()

	// if flag.NArg() == 0 {
	// 	return nil, fmt.Errorf("the path to the JTL source CSV file is required")
	// }
	// if flag.NArg() > 1 {
	// 	return nil, fmt.Errorf("expected only one non-flag argument, got (%d)", len(flag.Args()))
	// }

	args.PathToJtlSourceCsvFile = flag.Arg(0)

	return args, nil
}

func LogAnyRowsThatCannotBeProcessed(descriptors []*jtl.CsvDataRowError) {
	for _, rowError := range descriptors {
		fmt.Fprintf(os.Stderr, "[WARNING] ignoring CSV source file line (%d): %s\n", rowError.LineNumber, rowError.Error)
	}
}

func WriteTimestampFiles(pathToTimestampFilesDirectory string, summarizer *jtl.Summarizer) error {
	startTimestampFile, err := os.Create(pathToTimestampFilesDirectory + "/start.ts")
	if err != nil {
		return fmt.Errorf("on attempt to write to (%s)/start.ts: %s", pathToTimestampFilesDirectory, err.Error())
	}
	defer startTimestampFile.Close()

	endTimestampFile, err := os.Create(pathToTimestampFilesDirectory + "/end.ts")
	if err != nil {
		return fmt.Errorf("on attempt to write to (%s)/end.ts: %s", pathToTimestampFilesDirectory, err.Error())
	}
	defer endTimestampFile.Close()

	aggregateStats, _ := summarizer.AggregateSummary()

	startTimestampInSeconds := aggregateStats.TimestampOfFirstDataEntryAsUnixEpochMs / 1000
	endTimestampInSeconds := aggregateStats.TimestampOfLastDataEntryAsUnixEpochMs / 1000

	if _, err = startTimestampFile.WriteString(fmt.Sprintf("%d", startTimestampInSeconds)); err != nil {
		return fmt.Errorf("on attempt to write to (%s)/start.ts: %s", pathToTimestampFilesDirectory, err.Error())
	}

	if _, err = endTimestampFile.WriteString(fmt.Sprintf("%d", endTimestampInSeconds)); err != nil {
		return fmt.Errorf("on attempt to write to (%s)/end.ts: %s", pathToTimestampFilesDirectory, err.Error())
	}

	return nil
}

func GenerateSummaryOutputText(summarizer *jtl.Summarizer) string {
	textBuffer := &bytes.Buffer{}

	textBuffer.WriteString("Category,Key,Total Requests Made,Failed Requests," +
		"TTFB Mean,TTFB Median,TTFB Stdev,TTFB Minimum,TTFB Maximum,TTFB 5th Percentile,TTFB 95th Percentile," +
		"TTLB Mean,TTLB Median,TTLB Stdev,TTLB Minimum,TTLB Maximum,TTLB 5th Percentile,TTLB 95th Percentile," +
		"Overall TPS\n")

	aggregateStats, _ := summarizer.AggregateSummary()

	ttfb := aggregateStats.TimeToFirstByteStatistics
	ttlb := aggregateStats.TimeToLastByteStatistics

	textBuffer.WriteString(fmt.Sprintf("Aggregate,,%d,%d,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f\n",
		aggregateStats.NumberOfMatchingRequests, aggregateStats.NumberOfMatchingRequests-uint(aggregateStats.NumberOfSuccessfulRequests),
		ttfb.Mean, ttfb.Mean, ttfb.PopulationStandardDeviation, ttfb.Minimum, ttfb.Maximum, ttfb.ValueAt5thPercentile, ttfb.ValueAt95thPercentile,
		ttlb.Mean, ttlb.Mean, ttlb.PopulationStandardDeviation, ttlb.Minimum, ttlb.Maximum, ttlb.ValueAt5thPercentile, ttlb.ValueAt95thPercentile,
		aggregateStats.AverageTPSRate))

	statsByURLs, _ := summarizer.SummariesForTheColumn(jtl.Column.ResultLabel)
	for _, s := range statsByURLs {
		textBuffer.WriteString(GenerateSummaryTextForColumnValue(s, "method+uripath"))
	}

	statsByResponseCode, _ := summarizer.SummariesForTheColumn(jtl.Column.ResponseCodeOrErrorMessage)
	for _, s := range statsByResponseCode {
		textBuffer.WriteString(GenerateSummaryTextForColumnValue(s, "responseCode"))
	}

	statsByResponseSize, _ := summarizer.SummariesForTheColumn(jtl.Column.ResponseBytesReceived)
	for _, s := range statsByResponseSize {
		textBuffer.WriteString(GenerateSummaryTextForColumnValue(s, "responseSizeInBytes"))
	}

	statsByRequestSize, _ := summarizer.SummariesForTheColumn(jtl.Column.RequestBodySizeInBytes)
	for _, s := range statsByRequestSize {
		textBuffer.WriteString(GenerateSummaryTextForColumnValue(s, "requestBodyInBytes"))
	}

	return textBuffer.String()
}

func GenerateSummaryTextForColumnValue(s *jtl.ColumnUniqueValueSummary, labelForCategory string) string {
	ttfb := s.TimeToFirstByteStatistics
	ttlb := s.TimeToLastByteStatistics

	return fmt.Sprintf("%s,%s,%d,%d,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,-\n",
		labelForCategory, s.KeyAsAString(), s.NumberOfMatchingRequests, s.NumberOfMatchingRequests-uint(s.NumberOfSuccessfulRequests),
		ttfb.Mean, ttfb.Mean, ttfb.PopulationStandardDeviation, ttfb.Minimum, ttfb.Maximum, ttfb.ValueAt5thPercentile, ttfb.ValueAt95thPercentile,
		ttlb.Mean, ttlb.Mean, ttlb.PopulationStandardDeviation, ttlb.Minimum, ttlb.Maximum, ttlb.ValueAt5thPercentile, ttlb.ValueAt95thPercentile)
}

func WriteSummaryToFile(pathToOutputFile string, summaryText string) error {
	outputFile, err := os.Create(pathToOutputFile)
	if err != nil {
		return fmt.Errorf("on attempt to write to (%s): %s", pathToOutputFile, err.Error())
	}
	defer outputFile.Close()

	if _, err := outputFile.WriteString(summaryText); err != nil {
		return fmt.Errorf("on attempt to write to (%s): %s", pathToOutputFile, err.Error())
	}

	return nil
}
