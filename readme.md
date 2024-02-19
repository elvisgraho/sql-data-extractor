# SQL Dump Data Extractor

## Description

The SQL Dump Data Extractor is a command-line utility designed to parse SQL dump files and extract data from specified tables. The extracted data can be outputted in two formats: a JSON format for general use, and a simplified text format suitable for Hashcat.

### install

Using go get (deprecated for installing binaries from Go 1.17):

```bash
go get github.com/elvisgraho/sql_data_extractor
```

Using go install (preferred method for Go 1.16 and later):

```bash
go install github.com/elvisgraho/sql_data_extractor@latest
```

### Flags

**-file** to specify the path to the SQL dump file.

**-table** to specify the table name from which to extract data.

**-column** (optional) to specify a comma-separated list of column names to include in the output. If omitted, all columns will be included.

**-hashcat** (optional) to format the output for Hashcat, using ':' as a delimiter between column values. If omitted, the output will be in JSON format.

### Examples

To extract **user_email** and **user_pass** from the **users** table in **dump.sql** for Hashcat, use:

```bash
go run ./sql_data_extractor -file dump.sql -table users -column user_email,user_pass -hashcat
```

To extract all columns from the 'products' table in 'dump.sql' in JSON format, use:

```bash
go run ./sql_data_extractor -file dump.sql -table products
```
