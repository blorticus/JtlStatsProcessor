# JtlStatsProcessor

## NAME

jtl-stats-processor - provide summary statistics from a JMeter jtl file

## SYNOPSIS

```bash
jtl-stats-processor /path/to/jtl/file [-o /path/to/summary/output/file] [-t /directory/to/write/ts/files] [-m] [-h]
```

## BUILDING

```bash
git clone https://github.com/blorticus/JtlStatsProcessor.git
cd JtlStatsProcessor
go build -o jtl-stats-processor .
```

## DESCRIPTION

`jtl-stats-processor` reads a JMeter JTL raw data file and from it, produces summary statistics.
Specifically, it produces aggregate statistics for all request/response pairs in the file, and
summary statistics based on each request method+URI-path, based on each request message body size, and based on
each response message size.  For each target, it generates summary statistics for time-to-first byte
and time-to-last byte.  For each of those dimensions, it provides mean, median, population standard deviation,
minimum, maximum, 5th percentile and 95th percentile.  It also outputs how many requests matching the type/key
were completed and how many failed.

Each type/key pair generates a single row.  For example, for each unique request method+URI-path, a line as follows in prdouced:

```
method+uripath,GET /this/that/here,100,5,2.5,2,0.78,1,4,1,9,4.1,3,3.2,0,40,1,8,-
```

The first column is the type, which is one of:

* method+uripath
* responseCode
* responseSizeInBytes
* requestBodyInBytes

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

```
aggregate,,10000,3,...
```

The last column (overall TPS rate) applies only to the aggregate.  It is number of entries in the jtl divided by the
difference between the timestamp of the last and first entries.  That is, it is the average number of transactions
per-second.  For all rows except aggregate, this column has the value '-'.

If `-m` is provided, then the moving TPS rate statistics are also included.  In this case, the following additional
columns are added:

20. Moving TPS mean
21. Moving TPS median
22. Moving TPS population stdev
23. Moving TPS minimum
24. Moving TPS maximum
25. Moving TPS 5th percentile value
26. Moving TPS 95th percentile value

The "moving TPS" computes the number of entries in each discrete one second period based on the entry timestamp.
Then, for each one second interval, it counts the number of entries with a timestamp within that interval.
This is the TPS rate for each second.  Treating this set (i.e., the TPS for each one second period from the first
to the last in the source file) as the source, it computes the mean, media, stdev, and so forth.

Once again, this is applied only to the aggregate, so the value for this column in '-' for all other summary rows.

By default, the CSV results are printed to STDOUT.  If `-o` is provided, the results are written to the named
file instead.

If `-t` is provided, then in the named directory, a file called "start.ts" and a file called "end.ts" are generated.
start.ts contains the timestamp of the first entry in the jtl file, rounded to the floor second.  end.ts contains
the same, but for the last timestamp.  In other words, if the first timestamp is "1665666163199" and the last timestamp
is "1665666762869", the start.ts will contain the value 1665666163 while end.ts will contain the value 1665666762.
JTL timestamps are milliseconds since the unix epoch.
