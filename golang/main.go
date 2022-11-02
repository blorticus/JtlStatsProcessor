package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/blorticus-go/jtl"
)

// view this as man format using: pod2man <this_file> | nroff -man | less
/*
=head1 NAME

jtl-stats-processor - provide summary statistics from a JMeter jtl file

=head1 SYNOPSIS

 jtl-stats-processor /path/to/jtl/file [-o /path/to/summary/output/file] [-t /directory/to/write/ts/files] [-m] [-h]

=head1 DESCRIPTION

B<jtl-stats-processor> reads a JMeter JTL raw data file and from it, produces summary statistics.
Specifically, it produces aggregate statistics for all request/response pairs in the file, and
summary statistics based on each request method+URI-path, based on each request message body size, and based on
each response message size.  For each target, it generates summary statistics for time-to-first byte
and time-to-last byte.  For each of those dimensions, it provides mean, median, population standard deviation,
minimum, maximum, 5th percentile and 95th percentile.  It also outputs how many requests matching the type/key
were completed and how many failed.

Each type/key pair generates a single row.  For example, for each unique request method+URI-path, a line as follows in prdouced:

 method+uripath,GET /this/that/here,100,5,2.5,2,0.78,1,4,1,9,4.1,3,3.2,0,40,1,8,-

The first column is the type, which is one of:

=over 4

=item method+uripath
=item responseCode
=item responseSizeInBytes
=item requestBodyInBytes

=back

The second column is a unique key for that type.  In this example, it is a unique method+uripath.

The columns from third onward are:

 3. total requests matching type/key
 4. requests matching that failed
 5. time-to-first byte (ttfb) mean
 6. time-to-first byte (ttfb) median
 7. time-to-first byte (ttfb) population stdev
 8. time-to-first byte (ttfb) minimum
 9. time-to-first byte (ttfb) maximum
 10. time-to-first byte (ttfb) 5th percentile value
 11. time-to-first byte (ttfb) 95th percentile value
 12. time-to-last byte (ttlb) mean
 13. time-to-last byte (ttlb) median
 14. time-to-last byte (ttlb) population stdev
 15. time-to-last byte (ttlb) minimum
 16. time-to-last byte (ttlb) maximum
 17. time-to-last byte (ttlb) 5th percentile value
 18. time-to-last byte (ttlb) 95th percentile value
 19. overall TPS rate

The first row is the aggregate summary (i.e., the summary of values across all requests in the jtl file).  It has no
key, so it appears as follows:

 aggregate,,10000,3,...

The last column (overall TPS rate) applies only to the aggregate.  It is number of entries in the jtl divided by the
difference between the timestamp of the last and first entries.  That is, it is the average number of transactions
per-second.  For all rows except aggregate, this column has the value '-'.

If I<-m> is provided, then the moving TPS rate statistics are also included.  In this case, the following additional
columns are added:

 20. Moving TPS mean
 20. Moving TPS median
 20. Moving TPS population stdev
 20. Moving TPS minimum
 20. Moving TPS maximum
 20. Moving TPS 5th percentile value
 20. Moving TPS 95th percentile value

The "moving TPS" computes the number of entries in each discrete one second period based on the entry timestamp.
Then, for each one second interval, it counts the number of entries with a timestamp within that interval.
This is the TPS rate for each second.  Treating this set (i.e., the TPS for each one second period from the first
to the last in the source file) as the source, it computes the mean, media, stdev, and so forth.

Once again, this is applied only to the aggregate, so the value for this column in '-' for all other summary rows.

By default, the CSV results are printed to STDOUT.  If I<-o> is provided, the results are written to the named
file instead.

If I<-t> is provided, then in the named directory, a file called "start.ts" and a file called "end.ts" are generated.
start.ts contains the timestamp of the first entry in the jtl file, rounded to the floor second.  end.ts contains
the same, but for the last timestamp.  In other words, if the first timestamp is "1665666163199" and the last timestamp
is "1665666762869", the start.ts will contain the value 1665666163 while end.ts will contain the value 1665666762.
JTL timestamps are milliseconds since the unix epoch.

=head1 BLAME

 Vernon Wells (v.wells@f5.com)

=cut

*/

func main() {
	cliArgs, err := ProcessCommandLineOptions()
	DieIfError(err)

	jtlFile, err := os.Open(cliArgs.PathToJtlSourceCsvFile)
	DieIfError(err)

	jtlDataSource, dataRowsThatCannotBeProcessed, fatalError := jtl.NewDataSourceFromCsv(jtlFile)
	DieIfError(fatalError)

	LogAnyRowsThatCannotBeProcessed(dataRowsThatCannotBeProcessed)

	summarizer := jtl.NewSummarizerForDataSource(jtlDataSource)

	if cliArgs.IncludeMovingTPSSummary {
		err = summarizer.PreComputeAggregateSummaryAndSummariesForColumns(jtl.Column.RequestURL, jtl.Column.ResponseCodeOrErrorMessage, jtl.Column.RequestBodySizeInBytes, jtl.Column.ResponseBytesReceived, jtl.MetaColumn.MovingTransactionsPerSecond)
		DieIfError(err)
	} else {
		err = summarizer.PreComputeAggregateSummaryAndSummariesForColumns(jtl.Column.RequestURL, jtl.Column.ResponseCodeOrErrorMessage, jtl.Column.RequestBodySizeInBytes, jtl.Column.ResponseBytesReceived)
		DieIfError(err)
	}

	if cliArgs.PathToDirectoryForTimestampFiles != "" {
		err := WriteTimestampFiles(cliArgs.PathToDirectoryForTimestampFiles, summarizer)
		DieIfError(err)
	}

	summaryText := GenerateSummaryOutputText(summarizer, cliArgs.IncludeMovingTPSSummary)

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
	IncludeMovingTPSSummary          bool
}

func ProcessCommandLineOptions() (*CommandLineArguments, error) {
	args := &CommandLineArguments{}

	flag.StringVar(&args.PathToSummaryOutputCsvFile, "o", "", "Path to file for summary output")
	flag.StringVar(&args.PathToDirectoryForTimestampFiles, "t", "", "Path to directory into which timestamp files should be written")
	flag.BoolVar(&args.IncludeMovingTPSSummary, "m", false, "Include moving TPS summary statistics")

	flag.Parse()

	if flag.NArg() == 0 {
		return nil, fmt.Errorf("the path to the JTL source CSV file is required")
	}
	if flag.NArg() > 1 {
		return nil, fmt.Errorf("expected only one non-flag argument, got (%d)", len(flag.Args()))
	}

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

func GenerateSummaryOutputText(summarizer *jtl.Summarizer, includeMovingTPS bool) string {
	textBuffer := &bytes.Buffer{}

	textBuffer.WriteString("Category,Key,Total Requests Made,Failed Requests," +
		"TTFB Mean,TTFB Median,TTFB Stdev,TTFB Minimum,TTFB Maximum,TTFB 5th Percentile,TTFB 95th Percentile," +
		"TTLB Mean,TTLB Median,TTLB Stdev,TTLB Minimum,TTLB Maximum,TTLB 5th Percentile,TTLB 95th Percentile," +
		"Overall TPS")

	if includeMovingTPS {
		textBuffer.WriteString(",Moving TPS Mean,Moving TPS Median,Moving TPS Stdev,Moving TPS Minimum,Moving TPS Maximum,Moving TPS 5th Percentile,Moving TPS 95th Percentile")
	}

	textBuffer.WriteRune('\n')

	aggregateStats, _ := summarizer.AggregateSummary()

	ttfb := aggregateStats.TimeToFirstByteStatistics
	ttlb := aggregateStats.TimeToLastByteStatistics

	textBuffer.WriteString(fmt.Sprintf("Aggregate,,%d,%d,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f",
		aggregateStats.NumberOfMatchingRequests, aggregateStats.NumberOfMatchingRequests-uint(aggregateStats.NumberOfSuccessfulRequests),
		ttfb.Mean, ttfb.Mean, ttfb.PopulationStandardDeviation, ttfb.Minimum, ttfb.Maximum, ttfb.ValueAt5thPercentile, ttfb.ValueAt95thPercentile,
		ttlb.Mean, ttlb.Mean, ttlb.PopulationStandardDeviation, ttlb.Minimum, ttlb.Maximum, ttlb.ValueAt5thPercentile, ttlb.ValueAt95thPercentile,
		aggregateStats.AverageTPSRate))

	if includeMovingTPS {
		movingTPSSummaryStats, _ := summarizer.SummaryForTheMetaColumn(jtl.MetaColumn.MovingTransactionsPerSecond)
		textBuffer.WriteString(fmt.Sprintf(",%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f",
			movingTPSSummaryStats.Mean, movingTPSSummaryStats.Median, movingTPSSummaryStats.PopulationStandardDeviation,
			movingTPSSummaryStats.Minimum, movingTPSSummaryStats.Maximum, movingTPSSummaryStats.ValueAt5thPercentile,
			movingTPSSummaryStats.ValueAt95thPercentile))
	}

	textBuffer.WriteRune('\n')

	statsByURLs, _ := summarizer.SummariesForTheColumn(jtl.Column.ResultLabel)
	for _, s := range statsByURLs {
		textBuffer.WriteString(GenerateSummaryTextForColumnValue(s, "method+uripath", includeMovingTPS))
	}

	statsByResponseCode, _ := summarizer.SummariesForTheColumn(jtl.Column.ResponseCodeOrErrorMessage)
	for _, s := range statsByResponseCode {
		textBuffer.WriteString(GenerateSummaryTextForColumnValue(s, "responseCode", includeMovingTPS))
	}

	statsByResponseSize, _ := summarizer.SummariesForTheColumn(jtl.Column.ResponseBytesReceived)
	for _, s := range statsByResponseSize {
		textBuffer.WriteString(GenerateSummaryTextForColumnValue(s, "responseSizeInBytes", includeMovingTPS))
	}

	statsByRequestSize, _ := summarizer.SummariesForTheColumn(jtl.Column.RequestBodySizeInBytes)
	for _, s := range statsByRequestSize {
		textBuffer.WriteString(GenerateSummaryTextForColumnValue(s, "requestBodyInBytes", includeMovingTPS))
	}

	return textBuffer.String()
}

func GenerateSummaryTextForColumnValue(s *jtl.ColumnUniqueValueSummary, labelForCategory string, includeDashesForMovingTPS bool) string {
	ttfb := s.TimeToFirstByteStatistics
	ttlb := s.TimeToLastByteStatistics

	fmtString := "%s,%s,%d,%d,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,%0.2f,-"

	if includeDashesForMovingTPS {
		fmtString += ",-,-,-,-,-,-,-"
	}

	return fmt.Sprintf(fmtString+"\n",
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
