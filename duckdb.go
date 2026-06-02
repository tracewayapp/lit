package lit

import (
	"fmt"
	"strings"
)

type duckdbDriver struct{}

var DuckDB Driver = &duckdbDriver{}

func (d *duckdbDriver) Name() string { return "DuckDB" }

func (d *duckdbDriver) String() string { return d.Name() }

func (d *duckdbDriver) GenerateInsertQuery(tableName string, columnKeys []string, hasIntId bool) (string, []string) {
	var insertQuery strings.Builder

	insertQuery.WriteString("INSERT INTO ")
	insertQuery.WriteString(duckdbEscapeReserved(tableName))
	insertQuery.WriteString(" (")

	totalKeys := len(columnKeys)
	for i, k := range columnKeys {
		insertQuery.WriteString(duckdbEscapeReserved(k))
		if i != totalKeys-1 {
			insertQuery.WriteString(",")
		}
	}

	insertQuery.WriteString(") VALUES (")

	insertColumns := []string{}
	for i, k := range columnKeys {
		// DuckDB has no AUTO_INCREMENT; auto-id columns are sequence-backed and
		// populated with DEFAULT, just like PostgreSQL.
		if hasIntId && k == "id" {
			insertQuery.WriteString("DEFAULT")
		} else {
			insertColumns = append(insertColumns, k)
			insertQuery.WriteString("?")
		}
		if i != totalKeys-1 {
			insertQuery.WriteString(",")
		}
	}
	// DuckDB's Go driver does not support LastInsertId(); the generated id is read
	// back via RETURNING + QueryRow (see InsertAndGetId).
	insertQuery.WriteString(") RETURNING id")

	return insertQuery.String(), insertColumns
}

func (d *duckdbDriver) GenerateUpdateQuery(tableName string, columnKeys []string) string {
	var updateQuery strings.Builder
	updateQuery.WriteString("UPDATE ")
	updateQuery.WriteString(duckdbEscapeReserved(tableName))
	updateQuery.WriteString(" SET ")

	totalKeys := len(columnKeys)
	for i, k := range columnKeys {
		updateQuery.WriteString(duckdbEscapeReserved(k))
		updateQuery.WriteString(" = ?")
		if i != totalKeys-1 {
			updateQuery.WriteString(",")
		}
	}

	updateQuery.WriteString(" WHERE ")

	return updateQuery.String()
}

func (d *duckdbDriver) InsertAndGetId(ex Executor, query string, args ...any) (int, error) {
	row := ex.QueryRow(query, args...)
	var id int
	err := row.Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (d *duckdbDriver) Placeholder(argIndex int) string {
	return "?"
}

func (d *duckdbDriver) SupportsBackslashEscape() bool { return false }

func (d *duckdbDriver) RenumberWhereClause(where string, offset int) string {
	return where
}

func (d *duckdbDriver) JoinStringForIn(offset int, count int) string {
	return duckdbJoinStringForIn(count)
}

func duckdbJoinStringForIn(count int) string {
	var sb strings.Builder
	for i := 0; i < count; i++ {
		sb.WriteString("?")
		if i < count-1 {
			sb.WriteString(",")
		}
	}
	return sb.String()
}

func duckdbEscapeReserved(tableOrColumn string) string {
	escaped := strings.ReplaceAll(tableOrColumn, `"`, `""`)

	if _, exists := duckdbReservedKeywords[strings.ToUpper(tableOrColumn)]; exists {
		return `"` + escaped + `"`
	}
	return tableOrColumn
}

// ensure duckdbDriver implements Driver at compile time
var _ Driver = (*duckdbDriver)(nil)
var _ fmt.Stringer = (*duckdbDriver)(nil)

// duckdbReservedKeywords lists DuckDB's reserved keywords. Identifiers matching one of
// these are double-quoted by duckdbEscapeReserved.
var duckdbReservedKeywords = map[string]struct{}{
	"ALL":               {},
	"ANALYSE":           {},
	"ANALYZE":           {},
	"AND":               {},
	"ANY":               {},
	"ARRAY":             {},
	"AS":                {},
	"ASC":               {},
	"ASYMMETRIC":        {},
	"BOTH":              {},
	"CASE":              {},
	"CAST":              {},
	"CHECK":             {},
	"COLLATE":           {},
	"COLUMN":            {},
	"CONSTRAINT":        {},
	"CREATE":            {},
	"CURRENT_CATALOG":   {},
	"CURRENT_DATE":      {},
	"CURRENT_ROLE":      {},
	"CURRENT_SCHEMA":    {},
	"CURRENT_TIME":      {},
	"CURRENT_TIMESTAMP": {},
	"CURRENT_USER":      {},
	"DEFAULT":           {},
	"DEFERRABLE":        {},
	"DESC":              {},
	"DESCRIBE":          {},
	"DISTINCT":          {},
	"DO":                {},
	"ELSE":              {},
	"END":               {},
	"EXCEPT":            {},
	"FALSE":             {},
	"FETCH":             {},
	"FOR":               {},
	"FOREIGN":           {},
	"FROM":              {},
	"GRANT":             {},
	"GROUP":             {},
	"HAVING":            {},
	"IN":                {},
	"INITIALLY":         {},
	"INTERSECT":         {},
	"INTO":              {},
	"LATERAL":           {},
	"LEADING":           {},
	"LIMIT":             {},
	"LOCALTIME":         {},
	"LOCALTIMESTAMP":    {},
	"NOT":               {},
	"NULL":              {},
	"OFFSET":            {},
	"ON":                {},
	"ONLY":              {},
	"OR":                {},
	"ORDER":             {},
	"PIVOT":             {},
	"PIVOT_LONGER":      {},
	"PIVOT_WIDER":       {},
	"PLACING":           {},
	"PRIMARY":           {},
	"QUALIFY":           {},
	"REFERENCES":        {},
	"RETURNING":         {},
	"SELECT":            {},
	"SESSION_USER":      {},
	"SHOW":              {},
	"SOME":              {},
	"SYMMETRIC":         {},
	"TABLE":             {},
	"THEN":              {},
	"TO":                {},
	"TRAILING":          {},
	"TRUE":              {},
	"UNION":             {},
	"UNIQUE":            {},
	"UNPIVOT":           {},
	"USING":             {},
	"VARIADIC":          {},
	"WHEN":              {},
	"WHERE":             {},
	"WINDOW":            {},
	"WITH":              {},
}
