/*
SQL Dump Data Extractor

This application processes SQL dump files to extract data from specified tables and outputs the data in either JSON or .txt formats.

Usage:
    ./sql_data_extractor -file <path_to_sql_dump> -table <table_name> [options]

Options:
  -file       The path to the SQL dump file to be processed. (required)
  -table      The name of the table from which to extract data. (required)
  -column     Comma-separated list of column names to include in the output. If omitted, all columns will be included.
  -hashcat    When set, formats the output for Hashcat - value1:value2. Otherwise, outputs in JSON format.

Example:
    Extract 'user_email' and 'user_pass' from the 'users' table in 'dump.sql' for Hashcat:
    ./sql_data_extractor -file dump.sql -table users -column user_email,user_pass -hashcat

    Extract all columns from the 'products' table in 'dump.sql' in JSON format:
    ./sql_data_extractor -file dump.sql -table products
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Function to parse and validate command-line flags.
func parseFlags() (filename string, tableName string, includeColumns string, hashcat bool, err error) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `SQL Dump Data Extractor Usage:
  This application processes SQL dump files to extract data from specified tables and outputs the data in JSON format or a format suitable for Hashcat.

Usage:
 ./sql_data_extractor -file <path_to_sql_dump> -table <table_name> [options]

Options:
  -file       The path to the SQL dump file to be processed. (required)
  -table      The name of the table from which to extract data. (required)
  -column     Comma-separated list of column names to include in the output. If omitted, all columns will be included.
  -hashcat    When set, formats the output for Hashcat - value1:value2. Otherwise, outputs in JSON format.
`)
	}

	filenamePtr := flag.String("file", "", "Path to the SQL dump file")
	tableNamePtr := flag.String("table", "", "Name of the table to extract data from")
	includeColumnsPtr := flag.String("column", "", "Comma-separated list of column names to include in the output")
	hashcatPtr := flag.Bool("hashcat", false, "Format output for Hashcat")

	flag.Parse()

	// Check for mandatory flags and if not present, print usage and exit
	if *filenamePtr == "" || *tableNamePtr == "" {
		flag.Usage()
		err = fmt.Errorf("both -file and -table flags are required")
		return
	}

	// Assigning values from pointers to return variables
	filename = *filenamePtr
	tableName = *tableNamePtr
	includeColumns = *includeColumnsPtr
	hashcat = *hashcatPtr

	return
}

func main() {
	filename, tableName, includeColumns, hashcat, err := parseFlags()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading file: %s\n", err)
		os.Exit(1)
	}

	tableContent, err := findTableContent(string(content), tableName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	columns, err := extractColumnDefinitions(tableContent)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	includedColumns := parseIncludedColumns(includeColumns)

	records := processInsertStatements(tableContent, tableName, columns, includedColumns, hashcat)

	if err := writeToFile(filename, tableName, records, hashcat); err != nil {
		fmt.Printf("Error writing JSON file: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Data successfully written to %s_%s.json\n", strings.TrimSuffix(filename, ".sql"), tableName)
}

func findTableContent(dump, tableName string) (string, error) {
	// Adjusted regex to match CREATE TABLE block more accurately
	tableRegexPattern := fmt.Sprintf(
		`(?is)CREATE TABLE %s.*?;\s*(.*?)(?:UNLOCK TABLES;|DROP TABLE IF EXISTS|CREATE TABLE)`,
		regexp.QuoteMeta("`"+tableName+"`"),
	)
	tableRegex := regexp.MustCompile(tableRegexPattern)

	// Searching for the first occurrence since subsequent CREATE TABLE or DROP TABLE indicates a new table
	matches := tableRegex.FindStringSubmatch(dump)
	if len(matches) == 0 {
		return "", fmt.Errorf("table %s not found in the dump", tableName)
	}

	// Reconstructing the table section including CREATE TABLE statement and subsequent content up to but not including the next table's section
	tableSection := matches[0]
	if strings.Contains(tableSection, "UNLOCK TABLES;") {
		tableSection = strings.Split(tableSection, "UNLOCK TABLES;")[0] + "UNLOCK TABLES;"
	}

	return tableSection, nil
}

func extractColumnDefinitions(tableContent string) ([]string, error) {
	// First, extract only the column definition portion from the CREATE TABLE block
	// by stopping at the first line that doesn't start with a backtick, indicating the start of keys or other table-level definitions.
	columnSectionRegex := regexp.MustCompile(`(?is)CREATE TABLE.*?\((.*?)(?:,\s*(?:PRIMARY KEY|KEY|UNIQUE KEY|CONSTRAINT)|\)\s*ENGINE)`)
	columnSectionMatch := columnSectionRegex.FindStringSubmatch(tableContent)
	if len(columnSectionMatch) < 2 {
		return nil, fmt.Errorf("unable to extract column definitions from table content")
	}
	columnSection := columnSectionMatch[1]

	// Then, within this column definition portion, match only the column names.
	columnRegex := regexp.MustCompile("`([a-zA-Z0-9_]+)`\\s+[a-zA-Z]")
	matches := columnRegex.FindAllStringSubmatch(columnSection, -1)

	var columns []string
	for _, match := range matches {
		columns = append(columns, match[1])
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("no columns found in table section")
	}
	return columns, nil
}

func parseIncludedColumns(includeColumnsStr string) map[string]bool {
	includedColumns := make(map[string]bool)
	if includeColumnsStr != "" {
		for _, col := range strings.Split(includeColumnsStr, ",") {
			includedColumns[col] = true
		}
	}
	return includedColumns
}

// This function processes a single match and returns a slice of cleaned values.
func processSingleMatch(match string, columns []string, includedColumns map[string]bool) []string {
	values := regexp.MustCompile(`'(?:[^'\\]|\\.)*'|[^,]+`).FindAllString(match, -1)
	var record []string
	for i, value := range values {
		if i < len(columns) {
			columnName := columns[i]
			if includedColumns[columnName] || len(includedColumns) == 0 {
				cleanValue := strings.Trim(value, "'")
				record = append(record, cleanValue)
			}
		}
	}
	return record
}

func processInsertStatements(tableContent, tableName string, columns []string, includedColumns map[string]bool, hashcat bool) interface{} {
	insertRegex := regexp.MustCompile(`INSERT INTO .*? VALUES \((.*?)\);`)
	insertMatches := insertRegex.FindAllString(tableContent, -1)
	valueRegex := regexp.MustCompile(`\((.*?)\)`)

	var allValues []string
	for _, queries := range insertMatches {
		matches := valueRegex.FindAllString(queries, -1)
		for _, match := range matches {
			allValues = append(allValues, match[1:len(match)-1])
		}
	}

	if hashcat {
		var hashcatOutput []string
		for _, match := range allValues {
			record := processSingleMatch(match, columns, includedColumns)
			hashcatOutput = append(hashcatOutput, strings.Join(record, ":"))
		}
		return strings.Join(hashcatOutput, "\n")
	} else {
		var jsonRecords []map[string]interface{}
		for _, match := range allValues {
			record := processSingleMatch(match, columns, includedColumns)
			recordMap := make(map[string]interface{})
			for i, value := range record {
				if i < len(columns) {
					recordMap[columns[i]] = value
				}
			}
			jsonRecords = append(jsonRecords, recordMap)
		}
		return jsonRecords
	}
}

func writeToFile(baseFilename string, tableName string, data interface{}, hashcat bool) error {
	var outputData []byte
	var err error

	// Determine the file extension
	extension := ".json"
	if hashcat {
		extension = ".txt"
	}

	outputFilename := fmt.Sprintf("%s_%s%s", strings.TrimSuffix(baseFilename, ".sql"), tableName, extension)

	// Format the data based on the hashcat flag
	if hashcat {
		// For Hashcat, the data is expected to be a single string with records separated by newlines
		if strData, ok := data.(string); ok {
			outputData = []byte(strData)
		} else {
			return fmt.Errorf("hashcat data format error: expected a single string")
		}
	} else {
		// For JSON, marshal the data into JSON format
		outputData, err = json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
	}

	// Write the formatted data to the file
	return os.WriteFile(outputFilename, outputData, 0644)
}
