package api

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewJUnitXMLParser(t *testing.T) {
	// Create a fake JUnit XML file
	xmlFile := createFakeJUnitXMLFileForOpenShiftTests()
	defer removeFakeJUnitXMLFile(xmlFile)

	parser, err := NewJUnitXMLParser(xmlFile)
	assert.NoError(t, err)
	assert.NotNil(t, parser)

	// Assert the parsed test suite
	assert.Equal(t, "openshift-tests", parser.Parsed.Name)
	assert.Equal(t, 3, parser.Parsed.Tests)
	assert.Equal(t, 1, parser.Parsed.Skipped)
	assert.Equal(t, 1, parser.Parsed.Failures)
	assert.Equal(t, "TestVersion", parser.Parsed.Property.Name)
	assert.Equal(t, "4.14.0-202310201027.p0.g948001a.assembly.stream-948001a", parser.Parsed.Property.Value)

	// Assert the parsed test cases
	assert.Len(t, parser.Cases, 3)

	// Assert the pass test case
	assert.Equal(t, "test_case_name_1", parser.Cases[0].Name)
	assert.Equal(t, "test_case_time_1", parser.Cases[0].Time)
	assert.Equal(t, "test_case_system_out_1", parser.Cases[0].SystemOut)
	assert.Equal(t, TestStatusPass, parser.Cases[0].Status)

	// Assert the skipped test case
	assert.Equal(t, "test_case_name_2", parser.Cases[1].Name)
	assert.Equal(t, "test_case_time_2", parser.Cases[1].Time)
	assert.Equal(t, "test_case_system_out_2", parser.Cases[1].SystemOut)
	assert.Equal(t, TestStatusSkipped, parser.Cases[1].Status)
	assert.Equal(t, "test_case_skipped_message_2", parser.Cases[1].Skipped.Message)

	// Assert the failed test case
	assert.Equal(t, "test_case_name_3", parser.Cases[2].Name)
	assert.Equal(t, "test_case_time_3", parser.Cases[2].Time)
	assert.Equal(t, "test_case_system_out_3", parser.Cases[2].SystemOut)
	assert.Equal(t, TestStatusFail, parser.Cases[2].Status)
	assert.Equal(t, "test_case_failure_3", parser.Cases[2].Failure)

	// Assert the counters
	assert.Equal(t, 3, parser.Counters.Total)
	assert.Equal(t, 1, parser.Counters.Skipped)
	assert.Equal(t, 1, parser.Counters.Failures)
	assert.Equal(t, 1, parser.Counters.Pass)
}

func createFakeJUnitXMLFileForOpenShiftTests() string {
	// Create a temporary file
	file, err := os.CreateTemp("", "opct-e2e.xml")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Case 1: Write the fake JUnit XML content to the file
	content := `<?xml version="1.0" encoding="UTF-8"?>
	<testsuite name="openshift-tests" tests="3" skipped="1" failures="1" time="2568">
    	<property name="TestVersion" value="4.14.0-202310201027.p0.g948001a.assembly.stream-948001a"></property>
		<testcase name="test_case_name_1" time="test_case_time_1">
			<system-out>test_case_system_out_1</system-out>
		</testcase>
		<testcase name="test_case_name_2" time="test_case_time_2">
			<system-out>test_case_system_out_2</system-out>
			<skipped message="test_case_skipped_message_2"/>
		</testcase>
		<testcase name="test_case_name_3" time="test_case_time_3">
			<system-out>test_case_system_out_3</system-out>
			<failure>test_case_failure_3</failure>
		</testcase>
	</testsuite>`
	_, err = file.WriteString(content)
	if err != nil {
		panic(err)
	}

	// Return the file path
	return file.Name()
}

func removeFakeJUnitXMLFile(file string) {
	// Remove the temporary file
	err := os.Remove(file)
	if err != nil {
		panic(err)
	}
}
