package report

// TODO(mtulio):
// - create single interface to create report
// - move ConsolidatedSummary actions to here
// - report must extract the data from the extractor (consolidated summary)
// - report must validate the data from the extractor (consolidated summary)
// - report must create the report from the data from the extractor (consolidated summary)
// - report must save the report to the filesystem
// - report must serve the report to the user
// - report must have a way to be tested
//
// The report must be able to output in different formats (html, json, cli, etc)
// ETL strategy:
// - Extract: read test resultsfrom artifacts and save it in memory
// - Transform: apply rules to summarize to create the data layer
// - Load: process the data collected to outputs: (json, cli, html, etc)
