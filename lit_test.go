package lit

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestUser struct {
	Id        int
	FirstName string
	LastName  string
	Email     string
}

type TestProduct struct {
	Id    string
	Name  string
	Price int
}

type TestUserWithTags struct {
	Id        int    `lit:"id"`
	FirstName string `lit:"first_name"`
	LastName  string `lit:"surname"` // Different from default snake_case
	Email     string `lit:"email_address"`
}

type TestMixedTags struct {
	Id          int
	FirstName   string `lit:"given_name"`
	LastName    string // Will use default snake_case (last_name)
	PhoneNumber string `lit:"phone"`
}

func TestDriverName(t *testing.T) {
	assert.Equal(t, "PostgreSQL", PostgreSQL.Name())
	assert.Equal(t, "MySQL", MySQL.Name())
	assert.Equal(t, "SQLite", SQLite.Name())
	assert.Equal(t, "DuckDB", DuckDB.Name())
}

func TestRegisterDriver(t *testing.T) {
	originalDriver := defaultDriver
	defer func() { defaultDriver = originalDriver }()

	RegisterDriver(PostgreSQL)
	assert.NotNil(t, defaultDriver)
	assert.Equal(t, PostgreSQL, defaultDriver)

	RegisterDriver(MySQL)
	assert.NotNil(t, defaultDriver)
	assert.Equal(t, MySQL, defaultDriver)
}

func TestDefaultDbNamingStrategy_GetTableNameFromStructName(t *testing.T) {
	ns := DefaultDbNamingStrategy{}
	tests := []struct {
		input    string
		expected string
	}{
		{"User", "users"},
		{"UserProfile", "user_profiles"},
		{"HTTPRequest", "http_requests"},
		{"A", "as"},
		{"UserID", "user_ids"},
		{"XMLParser", "xml_parsers"},
		{"SimpleHTTP", "simple_https"},
		{"ID", "ids"},
		{"APIKey", "api_keys"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ns.GetTableNameFromStructName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultDbNamingStrategy_GetColumnNameFromStructName(t *testing.T) {
	ns := DefaultDbNamingStrategy{}
	tests := []struct {
		input    string
		expected string
	}{
		{"Id", "id"},
		{"FirstName", "first_name"},
		{"email", "email"},
		{"HTTPCode", "http_code"},
		{"UserID", "user_id"},
		{"XMLData", "xml_data"},
		{"APIKey", "api_key"},
		{"ID", "id"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ns.GetColumnNameFromStructName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegisterModel_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())

	RegisterModel[TestUser](PostgreSQL)

	fieldMap, err := GetFieldMap(reflect.TypeFor[TestUser]())
	require.NoError(t, err)
	require.NotNil(t, fieldMap)

	assert.True(t, fieldMap.HasIntId)
	assert.Equal(t, PostgreSQL, fieldMap.Driver)
	assert.Contains(t, fieldMap.ColumnKeys, "id")
	assert.Contains(t, fieldMap.ColumnKeys, "first_name")
	assert.Contains(t, fieldMap.ColumnKeys, "last_name")
	assert.Contains(t, fieldMap.ColumnKeys, "email")

	assert.Contains(t, fieldMap.InsertQuery, "RETURNING id")
	assert.Contains(t, fieldMap.InsertQuery, "$1")
}

func TestRegisterModel_WithDefaultDriver_PostgreSQL(t *testing.T) {
	originalDriver := defaultDriver
	defer func() { defaultDriver = originalDriver }()

	RegisterDriver(PostgreSQL)
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())

	RegisterModel[TestUser]()

	fieldMap, err := GetFieldMap(reflect.TypeFor[TestUser]())
	require.NoError(t, err)
	require.NotNil(t, fieldMap)

	assert.True(t, fieldMap.HasIntId)
	assert.Equal(t, PostgreSQL, fieldMap.Driver)
	assert.Contains(t, fieldMap.InsertQuery, "RETURNING id")
	assert.Contains(t, fieldMap.InsertQuery, "$1")
}

func TestRegisterModel_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())

	RegisterModel[TestUser](MySQL)

	fieldMap, err := GetFieldMap(reflect.TypeFor[TestUser]())
	require.NoError(t, err)
	require.NotNil(t, fieldMap)

	assert.True(t, fieldMap.HasIntId)
	assert.Equal(t, MySQL, fieldMap.Driver)

	assert.NotContains(t, fieldMap.InsertQuery, "RETURNING")
	assert.Contains(t, fieldMap.InsertQuery, "?")
	assert.NotContains(t, fieldMap.InsertQuery, "$")
}

func TestRegisterModel_WithDefaultDriver_MySQL(t *testing.T) {
	originalDriver := defaultDriver
	defer func() { defaultDriver = originalDriver }()

	RegisterDriver(MySQL)
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())

	RegisterModel[TestUser]()

	fieldMap, err := GetFieldMap(reflect.TypeFor[TestUser]())
	require.NoError(t, err)
	require.NotNil(t, fieldMap)

	assert.True(t, fieldMap.HasIntId)
	assert.Equal(t, MySQL, fieldMap.Driver)
	assert.NotContains(t, fieldMap.InsertQuery, "RETURNING")
	assert.Contains(t, fieldMap.InsertQuery, "?")
	assert.NotContains(t, fieldMap.InsertQuery, "$")
}

func TestRegisterModel_NoDriver_Panics(t *testing.T) {
	originalDriver := defaultDriver
	defer func() { defaultDriver = originalDriver }()

	defaultDriver = nil
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())

	assert.Panics(t, func() {
		RegisterModel[TestUser]()
	}, "Expected panic when no driver provided and no default driver set")
}

func TestGetFieldMap_NotRegistered(t *testing.T) {
	type UnregisteredType struct {
		Id int
	}
	delete(StructToFieldMap, reflect.TypeFor[UnregisteredType]())

	fieldMap, err := GetFieldMap(reflect.TypeFor[UnregisteredType]())
	assert.Error(t, err)
	assert.Nil(t, fieldMap)
	assert.Contains(t, err.Error(), "non registered model")
}

func TestPgInsertUpdateQueryGenerator_GenerateInsertQuery(t *testing.T) {
	gen := PgInsertUpdateQueryGenerator{}

	tests := []struct {
		name             string
		tableName        string
		columnKeys       []string
		hasIntId         bool
		expectedContains []string
		expectedColumns  []string
	}{
		{
			name:             "with int id",
			tableName:        "users",
			columnKeys:       []string{"id", "first_name", "last_name"},
			hasIntId:         true,
			expectedContains: []string{"INSERT INTO", "users", "DEFAULT", "RETURNING id", "$1", "$2"},
			expectedColumns:  []string{"first_name", "last_name"},
		},
		{
			name:             "without int id",
			tableName:        "products",
			columnKeys:       []string{"product_id", "name", "price"},
			hasIntId:         false,
			expectedContains: []string{"INSERT INTO", "products", "$1", "$2", "$3", "RETURNING id"},
			expectedColumns:  []string{"product_id", "name", "price"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, columns := gen.GenerateInsertQuery(tt.tableName, tt.columnKeys, tt.hasIntId)

			for _, s := range tt.expectedContains {
				assert.Contains(t, query, s)
			}
			assert.Equal(t, tt.expectedColumns, columns)
		})
	}
}

func TestPgInsertUpdateQueryGenerator_GenerateUpdateQuery(t *testing.T) {
	gen := PgInsertUpdateQueryGenerator{}

	columnKeys := []string{"id", "first_name", "last_name"}
	query := gen.GenerateUpdateQuery("users", columnKeys)

	assert.Contains(t, query, "UPDATE users")
	assert.Contains(t, query, "SET")
	assert.Contains(t, query, "id = $1")
	assert.Contains(t, query, "first_name = $2")
	assert.Contains(t, query, "last_name = $3")
	assert.Contains(t, query, "WHERE")
}

func TestMySqlInsertUpdateQueryGenerator_GenerateInsertQuery(t *testing.T) {
	gen := MySqlInsertUpdateQueryGenerator{}

	tests := []struct {
		name             string
		tableName        string
		columnKeys       []string
		hasIntId         bool
		expectedContains []string
		expectedColumns  []string
	}{
		{
			name:             "with int id",
			tableName:        "users",
			columnKeys:       []string{"id", "first_name", "last_name"},
			hasIntId:         true,
			expectedContains: []string{"INSERT INTO", "users", "NULL", "?"},
			expectedColumns:  []string{"first_name", "last_name"},
		},
		{
			name:             "without int id",
			tableName:        "products",
			columnKeys:       []string{"product_id", "name", "price"},
			hasIntId:         false,
			expectedContains: []string{"INSERT INTO", "products", "?"},
			expectedColumns:  []string{"product_id", "name", "price"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, columns := gen.GenerateInsertQuery(tt.tableName, tt.columnKeys, tt.hasIntId)

			for _, s := range tt.expectedContains {
				assert.Contains(t, query, s)
			}
			assert.NotContains(t, query, "RETURNING")
			assert.Equal(t, tt.expectedColumns, columns)
		})
	}
}

func TestMySqlInsertUpdateQueryGenerator_GenerateUpdateQuery(t *testing.T) {
	gen := MySqlInsertUpdateQueryGenerator{}

	columnKeys := []string{"id", "first_name", "last_name"}
	query := gen.GenerateUpdateQuery("users", columnKeys)

	assert.Contains(t, query, "UPDATE users")
	assert.Contains(t, query, "SET")
	assert.Contains(t, query, "id = ?")
	assert.Contains(t, query, "first_name = ?")
	assert.Contains(t, query, "last_name = ?")
	assert.Contains(t, query, "WHERE")
	assert.NotContains(t, query, "$")
}

func TestJoinForIn(t *testing.T) {
	tests := []struct {
		name     string
		ids      []int
		expected string
	}{
		{"empty", []int{}, ""},
		{"single", []int{1}, "1"},
		{"multiple", []int{1, 2, 3}, "1,2,3"},
		{"negative", []int{-1, 0, 1}, "-1,0,1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinForIn(tt.ids)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinStringForIn_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](PostgreSQL)

	tests := []struct {
		name     string
		offset   int
		params   []string
		expected string
	}{
		{"empty", 0, []string{}, ""},
		{"no offset", 0, []string{"a", "b"}, "$1,$2"},
		{"with offset", 2, []string{"a", "b"}, "$3,$4"},
		{"large offset", 10, []string{"x"}, "$11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinStringForIn[TestUser](tt.offset, tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinStringForIn_MySQL(t *testing.T) {
	type MySQLUser struct {
		Id   int
		Name string
	}
	delete(StructToFieldMap, reflect.TypeFor[MySQLUser]())
	RegisterModel[MySQLUser](MySQL)

	tests := []struct {
		name     string
		offset   int
		params   []string
		expected string
	}{
		{"empty", 0, []string{}, ""},
		{"ignores offset", 5, []string{"a", "b"}, "?,?"},
		{"multiple", 0, []string{"x", "y", "z"}, "?,?,?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinStringForIn[MySQLUser](tt.offset, tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinStringForInWithDriver(t *testing.T) {
	assert.Equal(t, "$1,$2", JoinStringForInWithDriver(PostgreSQL, 0, 2))
	assert.Equal(t, "$3,$4,$5", JoinStringForInWithDriver(PostgreSQL, 2, 3))

	assert.Equal(t, "?,?", JoinStringForInWithDriver(MySQL, 0, 2))
	assert.Equal(t, "?,?,?", JoinStringForInWithDriver(MySQL, 999, 3))
}

func TestPgRenumberPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		where    string
		offset   int
		expected string
	}{
		{"no placeholders", "id = 5", 3, "id = 5"},
		{"single placeholder", "id = $1", 3, "id = $4"},
		{"multiple placeholders", "id = $1 AND status = $2", 5, "id = $6 AND status = $7"},
		{"placeholder at end", "name = $1", 2, "name = $3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pgRenumberPlaceholders(tt.where, tt.offset)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSelect_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com").
		AddRow(2, "Jane", "Smith", "jane@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users").WillReturnRows(rows)

	users, err := Select[TestUser](db, "SELECT * FROM test_users")
	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, "John", users[0].FirstName)
	assert.Equal(t, "Jane", users[1].FirstName)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelect_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com").
		AddRow(2, "Jane", "Smith", "jane@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users").WillReturnRows(rows)

	users, err := Select[TestUser](db, "SELECT * FROM test_users")
	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, "John", users[0].FirstName)
	assert.Equal(t, "Jane", users[1].FirstName)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelectSingle_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(rows)

	user, err := SelectSingle[TestUser](db, "SELECT * FROM test_users WHERE id = $1", 1)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "John", user.FirstName)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelectSingle_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users WHERE id = \\?").
		WithArgs(1).
		WillReturnRows(rows)

	user, err := SelectSingle[TestUser](db, "SELECT * FROM test_users WHERE id = ?", 1)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "John", user.FirstName)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelectSingle_NoResults_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"})

	mock.ExpectQuery("SELECT \\* FROM test_users WHERE id = \\$1").
		WithArgs(999).
		WillReturnRows(rows)

	user, err := SelectSingle[TestUser](db, "SELECT * FROM test_users WHERE id = $1", 999)
	require.NoError(t, err)
	assert.Nil(t, user)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelectSingle_NoResults_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"})

	mock.ExpectQuery("SELECT \\* FROM test_users WHERE id = \\?").
		WithArgs(999).
		WillReturnRows(rows)

	user, err := SelectSingle[TestUser](db, "SELECT * FROM test_users WHERE id = ?", 999)
	require.NoError(t, err)
	assert.Nil(t, user)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow(42)

	mock.ExpectQuery("INSERT INTO test_users").
		WithArgs("John", "Doe", "john@example.com").
		WillReturnRows(rows)

	user := &TestUser{FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	id, err := Insert[TestUser](db, user)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO test_users").
		WithArgs("John", "Doe", "john@example.com").
		WillReturnResult(sqlmock.NewResult(42, 1))

	user := &TestUser{FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	id, err := Insert[TestUser](db, user)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("UPDATE test_users SET").
		WithArgs(1, "John", "Doe", "john@example.com", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	user := &TestUser{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	err = Update[TestUser](db, user, "id = $1", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("UPDATE test_users SET").
		WithArgs(1, "John", "Doe", "john@example.com", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	user := &TestUser{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	err = Update[TestUser](db, user, "id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_NoWhere(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](PostgreSQL)

	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	user := &TestUser{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	err = Update[TestUser](db, user, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "where")
}

func TestDelete_PostgreSQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("DELETE FROM test_users WHERE id = \\$1").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = Delete(db, "DELETE FROM test_users WHERE id = $1", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDelete_MySQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("DELETE FROM test_users WHERE id = \\?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = Delete(db, "DELETE FROM test_users WHERE id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExecutorWithTransaction_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users").WillReturnRows(rows)
	mock.ExpectCommit()

	tx, err := db.Begin()
	require.NoError(t, err)

	users, err := Select[TestUser](tx, "SELECT * FROM test_users")
	require.NoError(t, err)
	assert.Len(t, users, 1)

	err = tx.Commit()
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExecutorWithTransaction_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users").WillReturnRows(rows)
	mock.ExpectCommit()

	tx, err := db.Begin()
	require.NoError(t, err)

	users, err := Select[TestUser](tx, "SELECT * FROM test_users")
	require.NoError(t, err)
	assert.Len(t, users, 1)

	err = tx.Commit()
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertUuid_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestProduct]())
	RegisterModel[TestProduct](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO test_products").
		WillReturnResult(sqlmock.NewResult(0, 1))

	product := &TestProduct{Name: "Widget", Price: 100}
	uuid, err := InsertUuid[TestProduct](db, product)
	require.NoError(t, err)
	assert.NotEmpty(t, uuid)
	assert.Equal(t, uuid, product.Id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertUuid_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestProduct]())
	RegisterModel[TestProduct](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO test_products").
		WillReturnResult(sqlmock.NewResult(0, 1))

	product := &TestProduct{Name: "Widget", Price: 100}
	uuid, err := InsertUuid[TestProduct](db, product)
	require.NoError(t, err)
	assert.NotEmpty(t, uuid)
	assert.Equal(t, uuid, product.Id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertExistingUuid_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestProduct]())
	RegisterModel[TestProduct](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO test_products").
		WithArgs("existing-uuid-123", "Widget", 100).
		WillReturnResult(sqlmock.NewResult(0, 1))

	product := &TestProduct{Id: "existing-uuid-123", Name: "Widget", Price: 100}
	err = InsertExistingUuid[TestProduct](db, product)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertExistingUuid_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestProduct]())
	RegisterModel[TestProduct](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO test_products").
		WithArgs("existing-uuid-123", "Widget", 100).
		WillReturnResult(sqlmock.NewResult(0, 1))

	product := &TestProduct{Id: "existing-uuid-123", Name: "Widget", Price: 100}
	err = InsertExistingUuid[TestProduct](db, product)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRegisterModel_WithLitTags_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())

	RegisterModel[TestUserWithTags](PostgreSQL)

	fieldMap, err := GetFieldMap(reflect.TypeFor[TestUserWithTags]())
	require.NoError(t, err)
	require.NotNil(t, fieldMap)

	// Verify column names from lit tags are used
	assert.Contains(t, fieldMap.ColumnKeys, "id")
	assert.Contains(t, fieldMap.ColumnKeys, "first_name")
	assert.Contains(t, fieldMap.ColumnKeys, "surname")       // Custom tag, not "last_name"
	assert.Contains(t, fieldMap.ColumnKeys, "email_address") // Custom tag, not "email"

	// Verify ColumnsMap maps to correct field indices
	assert.Equal(t, 0, fieldMap.ColumnsMap["id"])
	assert.Equal(t, 1, fieldMap.ColumnsMap["first_name"])
	assert.Equal(t, 2, fieldMap.ColumnsMap["surname"])
	assert.Equal(t, 3, fieldMap.ColumnsMap["email_address"])

	// Verify INSERT query uses custom column names
	assert.Contains(t, fieldMap.InsertQuery, "surname")
	assert.Contains(t, fieldMap.InsertQuery, "email_address")
	assert.NotContains(t, fieldMap.InsertQuery, "last_name")
}

func TestRegisterModel_WithMixedTags_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestMixedTags]())

	RegisterModel[TestMixedTags](PostgreSQL)

	fieldMap, err := GetFieldMap(reflect.TypeFor[TestMixedTags]())
	require.NoError(t, err)
	require.NotNil(t, fieldMap)

	// Verify mixed usage: some from tags, some from naming strategy
	assert.Contains(t, fieldMap.ColumnKeys, "id")         // Default
	assert.Contains(t, fieldMap.ColumnKeys, "given_name") // From tag
	assert.Contains(t, fieldMap.ColumnKeys, "last_name")  // Default snake_case
	assert.Contains(t, fieldMap.ColumnKeys, "phone")      // From tag

	// Verify naming strategy default is NOT used when tag is present
	assert.NotContains(t, fieldMap.ColumnKeys, "first_name")   // Would be default
	assert.NotContains(t, fieldMap.ColumnKeys, "phone_number") // Would be default
}

func TestInsert_WithLitTags_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow(42)

	// Expect INSERT with custom column names from lit tags
	mock.ExpectQuery("INSERT INTO test_user_with_tagss \\(id,first_name,surname,email_address\\)").
		WithArgs("John", "Doe", "john@example.com").
		WillReturnRows(rows)

	user := &TestUserWithTags{FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	id, err := Insert[TestUserWithTags](db, user)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_WithLitTags_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect INSERT with custom column names from lit tags
	mock.ExpectExec("INSERT INTO test_user_with_tagss \\(id,first_name,surname,email_address\\)").
		WithArgs("John", "Doe", "john@example.com").
		WillReturnResult(sqlmock.NewResult(42, 1))

	user := &TestUserWithTags{FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	id, err := Insert[TestUserWithTags](db, user)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_WithLitTags_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect UPDATE with custom column names from lit tags
	mock.ExpectExec("UPDATE test_user_with_tagss SET id = \\$1,first_name = \\$2,surname = \\$3,email_address = \\$4 WHERE").
		WithArgs(1, "John", "Doe", "john@example.com", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	user := &TestUserWithTags{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	err = Update[TestUserWithTags](db, user, "id = $1", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_WithLitTags_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect UPDATE with custom column names from lit tags
	mock.ExpectExec("UPDATE test_user_with_tagss SET id = \\?,first_name = \\?,surname = \\?,email_address = \\? WHERE").
		WithArgs(1, "John", "Doe", "john@example.com", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	user := &TestUserWithTags{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	err = Update[TestUserWithTags](db, user, "id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelect_WithLitTags_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Return rows with custom column names (as they would be in the database)
	rows := sqlmock.NewRows([]string{"id", "first_name", "surname", "email_address"}).
		AddRow(1, "John", "Doe", "john@example.com").
		AddRow(2, "Jane", "Smith", "jane@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_user_with_tagss").WillReturnRows(rows)

	users, err := Select[TestUserWithTags](db, "SELECT * FROM test_user_with_tagss")
	require.NoError(t, err)
	assert.Len(t, users, 2)

	// Verify data is correctly mapped to struct fields
	assert.Equal(t, 1, users[0].Id)
	assert.Equal(t, "John", users[0].FirstName)
	assert.Equal(t, "Doe", users[0].LastName)
	assert.Equal(t, "john@example.com", users[0].Email)

	assert.Equal(t, 2, users[1].Id)
	assert.Equal(t, "Jane", users[1].FirstName)
	assert.Equal(t, "Smith", users[1].LastName)
	assert.Equal(t, "jane@example.com", users[1].Email)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelect_WithLitTags_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Return rows with custom column names (as they would be in the database)
	rows := sqlmock.NewRows([]string{"id", "first_name", "surname", "email_address"}).
		AddRow(1, "John", "Doe", "john@example.com").
		AddRow(2, "Jane", "Smith", "jane@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_user_with_tagss").WillReturnRows(rows)

	users, err := Select[TestUserWithTags](db, "SELECT * FROM test_user_with_tagss")
	require.NoError(t, err)
	assert.Len(t, users, 2)

	// Verify data is correctly mapped to struct fields
	assert.Equal(t, 1, users[0].Id)
	assert.Equal(t, "John", users[0].FirstName)
	assert.Equal(t, "Doe", users[0].LastName)
	assert.Equal(t, "john@example.com", users[0].Email)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Tests for pgEscapeReserved function
func TestPgEscapeReserved(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Non-reserved names should pass through unchanged
		{"non-reserved name", "users", "users"},
		{"non-reserved name with underscore", "user_profiles", "user_profiles"},
		{"non-reserved name camelCase", "firstName", "firstName"},

		// Reserved keywords should be quoted (case-insensitive)
		{"reserved keyword uppercase", "SELECT", `"SELECT"`},
		{"reserved keyword lowercase", "select", `"select"`},
		{"reserved keyword mixed case", "Select", `"Select"`},

		// Common reserved keywords
		{"reserved ORDER", "ORDER", `"ORDER"`},
		{"reserved order lowercase", "order", `"order"`},
		{"reserved GROUP", "GROUP", `"GROUP"`},
		{"reserved TABLE", "TABLE", `"TABLE"`},
		{"reserved INDEX", "INDEX", `"INDEX"`},
		{"reserved KEY", "KEY", `"KEY"`},
		{"reserved USER", "USER", `"USER"`},
		{"reserved user lowercase", "user", `"user"`},

		// Name with double quote (not reserved, so no quoting but escaping would happen if reserved)
		{"non-reserved with quote", `my"column`, `my"column`},

		// Edge cases
		{"empty string", "", ""},
		{"single char non-reserved", "x", "x"},
		{"reserved AS", "AS", `"AS"`},
		{"reserved FROM", "FROM", `"FROM"`},
		{"reserved WHERE", "WHERE", `"WHERE"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pgEscapeReserved(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test that reserved keywords with embedded quotes are properly escaped
func TestPgEscapeReserved_WithQuotes(t *testing.T) {
	// If a reserved keyword somehow contains a double quote, it should be escaped
	// This is an edge case but tests the quote escaping logic
	// Note: The escaping happens but since we check the original value for reserved status,
	// a name like `SEL"ECT` won't match the reserved keyword `SELECT`
	result := pgEscapeReserved(`my"table`)
	assert.Equal(t, `my"table`, result) // Not reserved, so unchanged
}

func TestSelectSingle_WithLitTags_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "surname", "email_address"}).
		AddRow(1, "John", "Doe", "john@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_user_with_tagss WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(rows)

	user, err := SelectSingle[TestUserWithTags](db, "SELECT * FROM test_user_with_tagss WHERE id = $1", 1)
	require.NoError(t, err)
	require.NotNil(t, user)

	assert.Equal(t, 1, user.Id)
	assert.Equal(t, "John", user.FirstName)
	assert.Equal(t, "Doe", user.LastName)
	assert.Equal(t, "john@example.com", user.Email)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Tests for mysqlEscapeReserved function
func TestMysqlEscapeReserved(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Non-reserved names should pass through unchanged
		{"non-reserved name", "users", "users"},
		{"non-reserved name with underscore", "user_profiles", "user_profiles"},
		{"non-reserved name camelCase", "firstName", "firstName"},

		// Reserved keywords should be quoted with backticks (case-insensitive)
		{"reserved keyword uppercase", "SELECT", "`SELECT`"},
		{"reserved keyword lowercase", "select", "`select`"},
		{"reserved keyword mixed case", "Select", "`Select`"},

		// Common reserved keywords
		{"reserved ORDER", "ORDER", "`ORDER`"},
		{"reserved order lowercase", "order", "`order`"},
		{"reserved GROUP", "GROUP", "`GROUP`"},
		{"reserved TABLE", "TABLE", "`TABLE`"},
		{"reserved INDEX", "INDEX", "`INDEX`"},
		{"reserved KEY", "KEY", "`KEY`"},
		{"reserved USER", "USER", "`USER`"},
		{"reserved user lowercase", "user", "`user`"},

		// Name with backtick (not reserved, so no quoting)
		{"non-reserved with backtick", "my`column", "my`column"},

		// Edge cases
		{"empty string", "", ""},
		{"single char non-reserved", "x", "x"},
		{"reserved AS", "AS", "`AS`"},
		{"reserved FROM", "FROM", "`FROM`"},
		{"reserved WHERE", "WHERE", "`WHERE`"},

		// MySQL-specific reserved keywords
		{"reserved DUAL", "DUAL", "`DUAL`"},
		{"reserved FULLTEXT", "FULLTEXT", "`FULLTEXT`"},
		{"reserved KILL", "KILL", "`KILL`"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mysqlEscapeReserved(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test that non-reserved names with backticks are unchanged
func TestMysqlEscapeReserved_WithBackticks(t *testing.T) {
	result := mysqlEscapeReserved("my`table")
	assert.Equal(t, "my`table", result) // Not reserved, so unchanged
}

// Test struct with reserved keyword column names
type TestReservedKeywordModel struct {
	Id    int
	Order int    `lit:"order"` // Reserved keyword
	Group string `lit:"group"` // Reserved keyword
	Name  string
}

// Integration tests for PostgreSQL query generation with reserved keywords
func TestPgInsertUpdateQueryGenerator_ReservedKeywords(t *testing.T) {
	gen := PgInsertUpdateQueryGenerator{}

	t.Run("INSERT with reserved keyword columns", func(t *testing.T) {
		columnKeys := []string{"id", "order", "group", "name"}
		query, columns := gen.GenerateInsertQuery("test_table", columnKeys, true)

		// Reserved keywords should be quoted (NAME is also reserved in PostgreSQL)
		assert.Contains(t, query, `"order"`)
		assert.Contains(t, query, `"group"`)
		assert.Contains(t, query, `"name"`)
		// Non-reserved should not be quoted
		assert.Contains(t, query, "id")
		assert.Equal(t, []string{"order", "group", "name"}, columns)
	})

	t.Run("INSERT with reserved keyword table name", func(t *testing.T) {
		columnKeys := []string{"id", "value"}
		query, _ := gen.GenerateInsertQuery("user", columnKeys, true)

		// Reserved table name should be quoted
		assert.Contains(t, query, `INSERT INTO "user"`)
	})

	t.Run("UPDATE with reserved keyword columns", func(t *testing.T) {
		columnKeys := []string{"id", "order", "group", "name"}
		query := gen.GenerateUpdateQuery("test_table", columnKeys)

		// Reserved keywords should be quoted (NAME is also reserved in PostgreSQL)
		assert.Contains(t, query, `"order" = $2`)
		assert.Contains(t, query, `"group" = $3`)
		assert.Contains(t, query, `"name" = $4`)
		// Non-reserved should not be quoted
		assert.Contains(t, query, "id = $1")
	})

	t.Run("UPDATE with reserved keyword table name", func(t *testing.T) {
		columnKeys := []string{"id", "value"}
		query := gen.GenerateUpdateQuery("order", columnKeys)

		// Reserved table name should be quoted
		assert.Contains(t, query, `UPDATE "order"`)
	})
}

// Integration tests for MySQL query generation with reserved keywords
func TestMySqlInsertUpdateQueryGenerator_ReservedKeywords(t *testing.T) {
	gen := MySqlInsertUpdateQueryGenerator{}

	t.Run("INSERT with reserved keyword columns", func(t *testing.T) {
		columnKeys := []string{"id", "order", "group", "name"}
		query, columns := gen.GenerateInsertQuery("test_table", columnKeys, true)

		// Reserved keywords should be quoted with backticks (NAME is also reserved in MySQL)
		assert.Contains(t, query, "`order`")
		assert.Contains(t, query, "`group`")
		assert.Contains(t, query, "`name`")
		// Non-reserved should not be quoted
		assert.Contains(t, query, "id")
		assert.Equal(t, []string{"order", "group", "name"}, columns)
	})

	t.Run("INSERT with reserved keyword table name", func(t *testing.T) {
		columnKeys := []string{"id", "value"}
		query, _ := gen.GenerateInsertQuery("user", columnKeys, true)

		// Reserved table name should be quoted with backticks
		assert.Contains(t, query, "INSERT INTO `user`")
	})

	t.Run("UPDATE with reserved keyword columns", func(t *testing.T) {
		columnKeys := []string{"id", "order", "group", "name"}
		query := gen.GenerateUpdateQuery("test_table", columnKeys)

		// Reserved keywords should be quoted with backticks (NAME is also reserved in MySQL)
		assert.Contains(t, query, "`order` = ?")
		assert.Contains(t, query, "`group` = ?")
		assert.Contains(t, query, "`name` = ?")
		// Non-reserved should not be quoted
		assert.Contains(t, query, "id = ?")
	})

	t.Run("UPDATE with reserved keyword table name", func(t *testing.T) {
		columnKeys := []string{"id", "value"}
		query := gen.GenerateUpdateQuery("order", columnKeys)

		// Reserved table name should be quoted with backticks
		assert.Contains(t, query, "UPDATE `order`")
	})
}

// Full integration test: Insert with reserved keyword columns using PostgreSQL
func TestInsert_WithReservedKeywords_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow(42)

	// Expect INSERT with escaped reserved keywords (NAME is also reserved in PostgreSQL)
	mock.ExpectQuery(`INSERT INTO test_reserved_keyword_models \(id,"order","group","name"\)`).
		WithArgs(10, "TestGroup", "TestName").
		WillReturnRows(rows)

	model := &TestReservedKeywordModel{Order: 10, Group: "TestGroup", Name: "TestName"}
	id, err := Insert[TestReservedKeywordModel](db, model)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Full integration test: Insert with reserved keyword columns using MySQL
func TestInsert_WithReservedKeywords_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect INSERT with escaped reserved keywords using backticks (NAME is also reserved in MySQL)
	mock.ExpectExec("INSERT INTO test_reserved_keyword_models \\(id,`order`,`group`,`name`\\)").
		WithArgs(10, "TestGroup", "TestName").
		WillReturnResult(sqlmock.NewResult(42, 1))

	model := &TestReservedKeywordModel{Order: 10, Group: "TestGroup", Name: "TestName"}
	id, err := Insert[TestReservedKeywordModel](db, model)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Full integration test: Update with reserved keyword columns using PostgreSQL
func TestUpdate_WithReservedKeywords_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect UPDATE with escaped reserved keywords (NAME is also reserved in PostgreSQL)
	mock.ExpectExec(`UPDATE test_reserved_keyword_models SET id = \$1,"order" = \$2,"group" = \$3,"name" = \$4 WHERE`).
		WithArgs(1, 10, "TestGroup", "TestName", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	model := &TestReservedKeywordModel{Id: 1, Order: 10, Group: "TestGroup", Name: "TestName"}
	err = Update[TestReservedKeywordModel](db, model, "id = $1", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Full integration test: Update with reserved keyword columns using MySQL
func TestUpdate_WithReservedKeywords_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect UPDATE with escaped reserved keywords using backticks (NAME is also reserved in MySQL)
	mock.ExpectExec("UPDATE test_reserved_keyword_models SET id = \\?,`order` = \\?,`group` = \\?,`name` = \\? WHERE").
		WithArgs(1, 10, "TestGroup", "TestName", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	model := &TestReservedKeywordModel{Id: 1, Order: 10, Group: "TestGroup", Name: "TestName"}
	err = Update[TestReservedKeywordModel](db, model, "id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Full integration test: Select with reserved keyword columns using PostgreSQL
func TestSelect_WithReservedKeywords_PostgreSQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Return rows with reserved keyword column names
	rows := sqlmock.NewRows([]string{"id", "order", "group", "name"}).
		AddRow(1, 10, "GroupA", "Name1").
		AddRow(2, 20, "GroupB", "Name2")

	mock.ExpectQuery("SELECT \\* FROM test_reserved_keyword_models").WillReturnRows(rows)

	models, err := Select[TestReservedKeywordModel](db, "SELECT * FROM test_reserved_keyword_models")
	require.NoError(t, err)
	assert.Len(t, models, 2)

	assert.Equal(t, 1, models[0].Id)
	assert.Equal(t, 10, models[0].Order)
	assert.Equal(t, "GroupA", models[0].Group)
	assert.Equal(t, "Name1", models[0].Name)

	assert.Equal(t, 2, models[1].Id)
	assert.Equal(t, 20, models[1].Order)
	assert.Equal(t, "GroupB", models[1].Group)
	assert.Equal(t, "Name2", models[1].Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Full integration test: Select with reserved keyword columns using MySQL
func TestSelect_WithReservedKeywords_MySQL(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](MySQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Return rows with reserved keyword column names
	rows := sqlmock.NewRows([]string{"id", "order", "group", "name"}).
		AddRow(1, 10, "GroupA", "Name1").
		AddRow(2, 20, "GroupB", "Name2")

	mock.ExpectQuery("SELECT \\* FROM test_reserved_keyword_models").WillReturnRows(rows)

	models, err := Select[TestReservedKeywordModel](db, "SELECT * FROM test_reserved_keyword_models")
	require.NoError(t, err)
	assert.Len(t, models, 2)

	assert.Equal(t, 1, models[0].Id)
	assert.Equal(t, 10, models[0].Order)
	assert.Equal(t, "GroupA", models[0].Group)
	assert.Equal(t, "Name1", models[0].Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ==================== SQLite Tests ====================

func TestRegisterModel_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())

	RegisterModel[TestUser](SQLite)

	fieldMap, err := GetFieldMap(reflect.TypeFor[TestUser]())
	require.NoError(t, err)
	require.NotNil(t, fieldMap)

	assert.True(t, fieldMap.HasIntId)
	assert.Equal(t, SQLite, fieldMap.Driver)
	assert.Contains(t, fieldMap.ColumnKeys, "id")
	assert.Contains(t, fieldMap.ColumnKeys, "first_name")
	assert.Contains(t, fieldMap.ColumnKeys, "last_name")
	assert.Contains(t, fieldMap.ColumnKeys, "email")

	assert.NotContains(t, fieldMap.InsertQuery, "RETURNING")
	assert.Contains(t, fieldMap.InsertQuery, "?")
	assert.NotContains(t, fieldMap.InsertQuery, "$")
}

func TestSqliteInsertUpdateQueryGenerator_GenerateInsertQuery(t *testing.T) {
	gen := SqliteInsertUpdateQueryGenerator{}

	tests := []struct {
		name             string
		tableName        string
		columnKeys       []string
		hasIntId         bool
		expectedContains []string
		expectedColumns  []string
	}{
		{
			name:             "with int id",
			tableName:        "users",
			columnKeys:       []string{"id", "first_name", "last_name"},
			hasIntId:         true,
			expectedContains: []string{"INSERT INTO", "users", "NULL", "?"},
			expectedColumns:  []string{"first_name", "last_name"},
		},
		{
			name:             "without int id",
			tableName:        "products",
			columnKeys:       []string{"product_id", "name", "price"},
			hasIntId:         false,
			expectedContains: []string{"INSERT INTO", "products", "?"},
			expectedColumns:  []string{"product_id", "name", "price"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, columns := gen.GenerateInsertQuery(tt.tableName, tt.columnKeys, tt.hasIntId)

			for _, s := range tt.expectedContains {
				assert.Contains(t, query, s)
			}
			assert.NotContains(t, query, "RETURNING")
			assert.Equal(t, tt.expectedColumns, columns)
		})
	}
}

func TestSqliteInsertUpdateQueryGenerator_GenerateUpdateQuery(t *testing.T) {
	gen := SqliteInsertUpdateQueryGenerator{}

	columnKeys := []string{"id", "first_name", "last_name"}
	query := gen.GenerateUpdateQuery("users", columnKeys)

	assert.Contains(t, query, "UPDATE users")
	assert.Contains(t, query, "SET")
	assert.Contains(t, query, "id = ?")
	assert.Contains(t, query, "first_name = ?")
	assert.Contains(t, query, "last_name = ?")
	assert.Contains(t, query, "WHERE")
	assert.NotContains(t, query, "$")
}

func TestSelect_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com").
		AddRow(2, "Jane", "Smith", "jane@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users").WillReturnRows(rows)

	users, err := Select[TestUser](db, "SELECT * FROM test_users")
	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, "John", users[0].FirstName)
	assert.Equal(t, "Jane", users[1].FirstName)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelectSingle_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users WHERE id = \\?").
		WithArgs(1).
		WillReturnRows(rows)

	user, err := SelectSingle[TestUser](db, "SELECT * FROM test_users WHERE id = ?", 1)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "John", user.FirstName)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelectSingle_NoResults_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"})

	mock.ExpectQuery("SELECT \\* FROM test_users WHERE id = \\?").
		WithArgs(999).
		WillReturnRows(rows)

	user, err := SelectSingle[TestUser](db, "SELECT * FROM test_users WHERE id = ?", 999)
	require.NoError(t, err)
	assert.Nil(t, user)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO test_users").
		WithArgs("John", "Doe", "john@example.com").
		WillReturnResult(sqlmock.NewResult(42, 1))

	user := &TestUser{FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	id, err := Insert[TestUser](db, user)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("UPDATE test_users SET").
		WithArgs(1, "John", "Doe", "john@example.com", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	user := &TestUser{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	err = Update[TestUser](db, user, "id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDelete_SQLite(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("DELETE FROM test_users WHERE id = \\?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = Delete(db, "DELETE FROM test_users WHERE id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExecutorWithTransaction_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users").WillReturnRows(rows)
	mock.ExpectCommit()

	tx, err := db.Begin()
	require.NoError(t, err)

	users, err := Select[TestUser](tx, "SELECT * FROM test_users")
	require.NoError(t, err)
	assert.Len(t, users, 1)

	err = tx.Commit()
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertUuid_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestProduct]())
	RegisterModel[TestProduct](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO test_products").
		WillReturnResult(sqlmock.NewResult(0, 1))

	product := &TestProduct{Name: "Widget", Price: 100}
	uuid, err := InsertUuid[TestProduct](db, product)
	require.NoError(t, err)
	assert.NotEmpty(t, uuid)
	assert.Equal(t, uuid, product.Id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertExistingUuid_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestProduct]())
	RegisterModel[TestProduct](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO test_products").
		WithArgs("existing-uuid-123", "Widget", 100).
		WillReturnResult(sqlmock.NewResult(0, 1))

	product := &TestProduct{Id: "existing-uuid-123", Name: "Widget", Price: 100}
	err = InsertExistingUuid[TestProduct](db, product)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_WithLitTags_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect INSERT with custom column names from lit tags
	mock.ExpectExec("INSERT INTO test_user_with_tagss \\(id,first_name,surname,email_address\\)").
		WithArgs("John", "Doe", "john@example.com").
		WillReturnResult(sqlmock.NewResult(42, 1))

	user := &TestUserWithTags{FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	id, err := Insert[TestUserWithTags](db, user)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_WithLitTags_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect UPDATE with custom column names from lit tags
	mock.ExpectExec("UPDATE test_user_with_tagss SET id = \\?,first_name = \\?,surname = \\?,email_address = \\? WHERE").
		WithArgs(1, "John", "Doe", "john@example.com", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	user := &TestUserWithTags{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	err = Update[TestUserWithTags](db, user, "id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelect_WithLitTags_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Return rows with custom column names (as they would be in the database)
	rows := sqlmock.NewRows([]string{"id", "first_name", "surname", "email_address"}).
		AddRow(1, "John", "Doe", "john@example.com").
		AddRow(2, "Jane", "Smith", "jane@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_user_with_tagss").WillReturnRows(rows)

	users, err := Select[TestUserWithTags](db, "SELECT * FROM test_user_with_tagss")
	require.NoError(t, err)
	assert.Len(t, users, 2)

	// Verify data is correctly mapped to struct fields
	assert.Equal(t, 1, users[0].Id)
	assert.Equal(t, "John", users[0].FirstName)
	assert.Equal(t, "Doe", users[0].LastName)
	assert.Equal(t, "john@example.com", users[0].Email)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinStringForIn_SQLite(t *testing.T) {
	type SQLiteUser struct {
		Id   int
		Name string
	}
	delete(StructToFieldMap, reflect.TypeFor[SQLiteUser]())
	RegisterModel[SQLiteUser](SQLite)

	tests := []struct {
		name     string
		offset   int
		params   []string
		expected string
	}{
		{"empty", 0, []string{}, ""},
		{"ignores offset", 5, []string{"a", "b"}, "?,?"},
		{"multiple", 0, []string{"x", "y", "z"}, "?,?,?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinStringForIn[SQLiteUser](tt.offset, tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinStringForInWithDriver_SQLite(t *testing.T) {
	assert.Equal(t, "?,?", JoinStringForInWithDriver(SQLite, 0, 2))
	assert.Equal(t, "?,?,?", JoinStringForInWithDriver(SQLite, 999, 3))
}

func TestSqliteEscapeReserved(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Non-reserved names should pass through unchanged
		{"non-reserved name", "users", "users"},
		{"non-reserved name with underscore", "user_profiles", "user_profiles"},
		{"non-reserved name camelCase", "firstName", "firstName"},

		// Reserved keywords should be quoted with double quotes (case-insensitive)
		{"reserved keyword uppercase", "SELECT", `"SELECT"`},
		{"reserved keyword lowercase", "select", `"select"`},
		{"reserved keyword mixed case", "Select", `"Select"`},

		// Common reserved keywords
		{"reserved ORDER", "ORDER", `"ORDER"`},
		{"reserved order lowercase", "order", `"order"`},
		{"reserved GROUP", "GROUP", `"GROUP"`},
		{"reserved TABLE", "TABLE", `"TABLE"`},
		{"reserved INDEX", "INDEX", `"INDEX"`},
		{"reserved KEY", "KEY", `"KEY"`},

		// Edge cases
		{"empty string", "", ""},
		{"single char non-reserved", "x", "x"},
		{"reserved AS", "AS", `"AS"`},
		{"reserved FROM", "FROM", `"FROM"`},
		{"reserved WHERE", "WHERE", `"WHERE"`},

		// SQLite-specific reserved keywords
		{"reserved AUTOINCREMENT", "AUTOINCREMENT", `"AUTOINCREMENT"`},
		{"reserved GLOB", "GLOB", `"GLOB"`},
		{"reserved PRAGMA", "PRAGMA", `"PRAGMA"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sqliteEscapeReserved(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSqliteInsertUpdateQueryGenerator_ReservedKeywords(t *testing.T) {
	gen := SqliteInsertUpdateQueryGenerator{}

	t.Run("INSERT with reserved keyword columns", func(t *testing.T) {
		columnKeys := []string{"id", "order", "group", "name"}
		query, columns := gen.GenerateInsertQuery("test_table", columnKeys, true)

		// Reserved keywords should be quoted with double quotes
		assert.Contains(t, query, `"order"`)
		assert.Contains(t, query, `"group"`)
		// "name" is not a SQLite reserved keyword
		assert.Contains(t, query, "name")
		// Non-reserved should not be quoted
		assert.Contains(t, query, "id")
		assert.Equal(t, []string{"order", "group", "name"}, columns)
	})

	t.Run("INSERT with reserved keyword table name", func(t *testing.T) {
		columnKeys := []string{"id", "value"}
		query, _ := gen.GenerateInsertQuery("table", columnKeys, true)

		// Reserved table name should be quoted with double quotes
		assert.Contains(t, query, `INSERT INTO "table"`)
	})

	t.Run("UPDATE with reserved keyword columns", func(t *testing.T) {
		columnKeys := []string{"id", "order", "group", "name"}
		query := gen.GenerateUpdateQuery("test_table", columnKeys)

		// Reserved keywords should be quoted with double quotes
		assert.Contains(t, query, `"order" = ?`)
		assert.Contains(t, query, `"group" = ?`)
		// Non-reserved should not be quoted
		assert.Contains(t, query, "id = ?")
	})

	t.Run("UPDATE with reserved keyword table name", func(t *testing.T) {
		columnKeys := []string{"id", "value"}
		query := gen.GenerateUpdateQuery("order", columnKeys)

		// Reserved table name should be quoted with double quotes
		assert.Contains(t, query, `UPDATE "order"`)
	})
}

func TestInsert_WithReservedKeywords_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// "order" and "group" are SQLite reserved keywords, "name" is not
	mock.ExpectExec(`INSERT INTO test_reserved_keyword_models \(id,"order","group",name\)`).
		WithArgs(10, "TestGroup", "TestName").
		WillReturnResult(sqlmock.NewResult(42, 1))

	model := &TestReservedKeywordModel{Order: 10, Group: "TestGroup", Name: "TestName"}
	id, err := Insert[TestReservedKeywordModel](db, model)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_WithReservedKeywords_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// "order" and "group" are SQLite reserved keywords, "name" is not
	mock.ExpectExec(`UPDATE test_reserved_keyword_models SET id = \?,\"order\" = \?,\"group\" = \?,name = \? WHERE`).
		WithArgs(1, 10, "TestGroup", "TestName", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	model := &TestReservedKeywordModel{Id: 1, Order: 10, Group: "TestGroup", Name: "TestName"}
	err = Update[TestReservedKeywordModel](db, model, "id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelect_WithReservedKeywords_SQLite(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](SQLite)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Return rows with reserved keyword column names
	rows := sqlmock.NewRows([]string{"id", "order", "group", "name"}).
		AddRow(1, 10, "GroupA", "Name1").
		AddRow(2, 20, "GroupB", "Name2")

	mock.ExpectQuery("SELECT \\* FROM test_reserved_keyword_models").WillReturnRows(rows)

	models, err := Select[TestReservedKeywordModel](db, "SELECT * FROM test_reserved_keyword_models")
	require.NoError(t, err)
	assert.Len(t, models, 2)

	assert.Equal(t, 1, models[0].Id)
	assert.Equal(t, 10, models[0].Order)
	assert.Equal(t, "GroupA", models[0].Group)
	assert.Equal(t, "Name1", models[0].Name)

	assert.Equal(t, 2, models[1].Id)
	assert.Equal(t, 20, models[1].Order)
	assert.Equal(t, "GroupB", models[1].Group)
	assert.Equal(t, "Name2", models[1].Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ==================== DuckDB Tests ====================

func TestRegisterModel_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())

	RegisterModel[TestUser](DuckDB)

	fieldMap, err := GetFieldMap(reflect.TypeFor[TestUser]())
	require.NoError(t, err)
	require.NotNil(t, fieldMap)

	assert.True(t, fieldMap.HasIntId)
	assert.Equal(t, DuckDB, fieldMap.Driver)
	assert.Contains(t, fieldMap.ColumnKeys, "id")
	assert.Contains(t, fieldMap.ColumnKeys, "first_name")
	assert.Contains(t, fieldMap.ColumnKeys, "last_name")
	assert.Contains(t, fieldMap.ColumnKeys, "email")

	// DuckDB uses ? placeholders (no $N) but reads the generated id back via RETURNING.
	assert.Contains(t, fieldMap.InsertQuery, "RETURNING id")
	assert.Contains(t, fieldMap.InsertQuery, "?")
	assert.NotContains(t, fieldMap.InsertQuery, "$")
}

func TestDuckdbInsertUpdateQueryGenerator_GenerateInsertQuery(t *testing.T) {
	gen := duckdbDriver{}

	tests := []struct {
		name             string
		tableName        string
		columnKeys       []string
		hasIntId         bool
		expectedContains []string
		expectedColumns  []string
	}{
		{
			name:             "with int id",
			tableName:        "users",
			columnKeys:       []string{"id", "first_name", "last_name"},
			hasIntId:         true,
			expectedContains: []string{"INSERT INTO", "users", "DEFAULT", "?", "RETURNING id"},
			expectedColumns:  []string{"first_name", "last_name"},
		},
		{
			name:             "without int id",
			tableName:        "products",
			columnKeys:       []string{"product_id", "name", "price"},
			hasIntId:         false,
			expectedContains: []string{"INSERT INTO", "products", "?", "RETURNING id"},
			expectedColumns:  []string{"product_id", "name", "price"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, columns := gen.GenerateInsertQuery(tt.tableName, tt.columnKeys, tt.hasIntId)

			for _, s := range tt.expectedContains {
				assert.Contains(t, query, s)
			}
			assert.Contains(t, query, "RETURNING id")
			assert.NotContains(t, query, "$")
			assert.Equal(t, tt.expectedColumns, columns)
		})
	}
}

func TestDuckdbInsertUpdateQueryGenerator_GenerateUpdateQuery(t *testing.T) {
	gen := duckdbDriver{}

	columnKeys := []string{"id", "first_name", "last_name"}
	query := gen.GenerateUpdateQuery("users", columnKeys)

	assert.Contains(t, query, "UPDATE users")
	assert.Contains(t, query, "SET")
	assert.Contains(t, query, "id = ?")
	assert.Contains(t, query, "first_name = ?")
	assert.Contains(t, query, "last_name = ?")
	assert.Contains(t, query, "WHERE")
	assert.NotContains(t, query, "$")
}

func TestSelect_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com").
		AddRow(2, "Jane", "Smith", "jane@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users").WillReturnRows(rows)

	users, err := Select[TestUser](db, "SELECT * FROM test_users")
	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, "John", users[0].FirstName)
	assert.Equal(t, "Jane", users[1].FirstName)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelectSingle_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users WHERE id = \\?").
		WithArgs(1).
		WillReturnRows(rows)

	user, err := SelectSingle[TestUser](db, "SELECT * FROM test_users WHERE id = ?", 1)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "John", user.FirstName)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelectSingle_NoResults_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"})

	mock.ExpectQuery("SELECT \\* FROM test_users WHERE id = \\?").
		WithArgs(999).
		WillReturnRows(rows)

	user, err := SelectSingle[TestUser](db, "SELECT * FROM test_users WHERE id = ?", 999)
	require.NoError(t, err)
	assert.Nil(t, user)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// DuckDB reads the generated id back via RETURNING + QueryRow (Postgres-style).
	rows := sqlmock.NewRows([]string{"id"}).AddRow(42)

	mock.ExpectQuery("INSERT INTO test_users").
		WithArgs("John", "Doe", "john@example.com").
		WillReturnRows(rows)

	user := &TestUser{FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	id, err := Insert[TestUser](db, user)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("UPDATE test_users SET").
		WithArgs(1, "John", "Doe", "john@example.com", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	user := &TestUser{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	err = Update[TestUser](db, user, "id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDelete_DuckDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("DELETE FROM test_users WHERE id = \\?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = Delete(db, "DELETE FROM test_users WHERE id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExecutorWithTransaction_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()

	rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
		AddRow(1, "John", "Doe", "john@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_users").WillReturnRows(rows)
	mock.ExpectCommit()

	tx, err := db.Begin()
	require.NoError(t, err)

	users, err := Select[TestUser](tx, "SELECT * FROM test_users")
	require.NoError(t, err)
	assert.Len(t, users, 1)

	err = tx.Commit()
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertUuid_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestProduct]())
	RegisterModel[TestProduct](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO test_products").
		WillReturnResult(sqlmock.NewResult(0, 1))

	product := &TestProduct{Name: "Widget", Price: 100}
	uuid, err := InsertUuid[TestProduct](db, product)
	require.NoError(t, err)
	assert.NotEmpty(t, uuid)
	assert.Equal(t, uuid, product.Id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertExistingUuid_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestProduct]())
	RegisterModel[TestProduct](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO test_products").
		WithArgs("existing-uuid-123", "Widget", 100).
		WillReturnResult(sqlmock.NewResult(0, 1))

	product := &TestProduct{Id: "existing-uuid-123", Name: "Widget", Price: 100}
	err = InsertExistingUuid[TestProduct](db, product)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_WithLitTags_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow(42)

	// Expect INSERT with custom column names from lit tags
	mock.ExpectQuery("INSERT INTO test_user_with_tagss \\(id,first_name,surname,email_address\\)").
		WithArgs("John", "Doe", "john@example.com").
		WillReturnRows(rows)

	user := &TestUserWithTags{FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	id, err := Insert[TestUserWithTags](db, user)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_WithLitTags_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect UPDATE with custom column names from lit tags
	mock.ExpectExec("UPDATE test_user_with_tagss SET id = \\?,first_name = \\?,surname = \\?,email_address = \\? WHERE").
		WithArgs(1, "John", "Doe", "john@example.com", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	user := &TestUserWithTags{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
	err = Update[TestUserWithTags](db, user, "id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelect_WithLitTags_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUserWithTags]())
	RegisterModel[TestUserWithTags](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Return rows with custom column names (as they would be in the database)
	rows := sqlmock.NewRows([]string{"id", "first_name", "surname", "email_address"}).
		AddRow(1, "John", "Doe", "john@example.com").
		AddRow(2, "Jane", "Smith", "jane@example.com")

	mock.ExpectQuery("SELECT \\* FROM test_user_with_tagss").WillReturnRows(rows)

	users, err := Select[TestUserWithTags](db, "SELECT * FROM test_user_with_tagss")
	require.NoError(t, err)
	assert.Len(t, users, 2)

	// Verify data is correctly mapped to struct fields
	assert.Equal(t, 1, users[0].Id)
	assert.Equal(t, "John", users[0].FirstName)
	assert.Equal(t, "Doe", users[0].LastName)
	assert.Equal(t, "john@example.com", users[0].Email)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinStringForIn_DuckDB(t *testing.T) {
	type DuckDBUser struct {
		Id   int
		Name string
	}
	delete(StructToFieldMap, reflect.TypeFor[DuckDBUser]())
	RegisterModel[DuckDBUser](DuckDB)

	tests := []struct {
		name     string
		offset   int
		params   []string
		expected string
	}{
		{"empty", 0, []string{}, ""},
		{"ignores offset", 5, []string{"a", "b"}, "?,?"},
		{"multiple", 0, []string{"x", "y", "z"}, "?,?,?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinStringForIn[DuckDBUser](tt.offset, tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinStringForInWithDriver_DuckDB(t *testing.T) {
	assert.Equal(t, "?,?", JoinStringForInWithDriver(DuckDB, 0, 2))
	assert.Equal(t, "?,?,?", JoinStringForInWithDriver(DuckDB, 999, 3))
}

func TestDuckdbEscapeReserved(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Non-reserved names should pass through unchanged
		{"non-reserved name", "users", "users"},
		{"non-reserved name with underscore", "user_profiles", "user_profiles"},
		{"non-reserved name camelCase", "firstName", "firstName"},

		// Reserved keywords should be quoted with double quotes (case-insensitive)
		{"reserved keyword uppercase", "SELECT", `"SELECT"`},
		{"reserved keyword lowercase", "select", `"select"`},
		{"reserved keyword mixed case", "Select", `"Select"`},

		// Common reserved keywords
		{"reserved ORDER", "ORDER", `"ORDER"`},
		{"reserved order lowercase", "order", `"order"`},
		{"reserved GROUP", "GROUP", `"GROUP"`},
		{"reserved TABLE", "TABLE", `"TABLE"`},

		// Edge cases
		{"empty string", "", ""},
		{"single char non-reserved", "x", "x"},
		{"reserved AS", "AS", `"AS"`},
		{"reserved FROM", "FROM", `"FROM"`},
		{"reserved WHERE", "WHERE", `"WHERE"`},

		// DuckDB-specific reserved keywords
		{"reserved QUALIFY", "QUALIFY", `"QUALIFY"`},
		{"reserved PIVOT", "PIVOT", `"PIVOT"`},
		{"reserved DESCRIBE", "DESCRIBE", `"DESCRIBE"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := duckdbEscapeReserved(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDuckdbInsertUpdateQueryGenerator_ReservedKeywords(t *testing.T) {
	gen := duckdbDriver{}

	t.Run("INSERT with reserved keyword columns", func(t *testing.T) {
		columnKeys := []string{"id", "order", "group", "name"}
		query, columns := gen.GenerateInsertQuery("test_table", columnKeys, true)

		// Reserved keywords should be quoted with double quotes
		assert.Contains(t, query, `"order"`)
		assert.Contains(t, query, `"group"`)
		// "name" is not a DuckDB reserved keyword
		assert.Contains(t, query, "name")
		// Non-reserved should not be quoted
		assert.Contains(t, query, "id")
		// The int id column uses DEFAULT and is read back via RETURNING
		assert.Contains(t, query, "DEFAULT")
		assert.Contains(t, query, "RETURNING id")
		assert.Equal(t, []string{"order", "group", "name"}, columns)
	})

	t.Run("INSERT with reserved keyword table name", func(t *testing.T) {
		columnKeys := []string{"id", "value"}
		query, _ := gen.GenerateInsertQuery("table", columnKeys, true)

		// Reserved table name should be quoted with double quotes
		assert.Contains(t, query, `INSERT INTO "table"`)
	})

	t.Run("UPDATE with reserved keyword columns", func(t *testing.T) {
		columnKeys := []string{"id", "order", "group", "name"}
		query := gen.GenerateUpdateQuery("test_table", columnKeys)

		// Reserved keywords should be quoted with double quotes
		assert.Contains(t, query, `"order" = ?`)
		assert.Contains(t, query, `"group" = ?`)
		// Non-reserved should not be quoted
		assert.Contains(t, query, "id = ?")
	})

	t.Run("UPDATE with reserved keyword table name", func(t *testing.T) {
		columnKeys := []string{"id", "value"}
		query := gen.GenerateUpdateQuery("order", columnKeys)

		// Reserved table name should be quoted with double quotes
		assert.Contains(t, query, `UPDATE "order"`)
	})
}

func TestInsert_WithReservedKeywords_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow(42)

	// "order" and "group" are DuckDB reserved keywords, "name" is not
	mock.ExpectQuery(`INSERT INTO test_reserved_keyword_models \(id,"order","group",name\)`).
		WithArgs(10, "TestGroup", "TestName").
		WillReturnRows(rows)

	model := &TestReservedKeywordModel{Order: 10, Group: "TestGroup", Name: "TestName"}
	id, err := Insert[TestReservedKeywordModel](db, model)
	require.NoError(t, err)
	assert.Equal(t, 42, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_WithReservedKeywords_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// "order" and "group" are DuckDB reserved keywords, "name" is not
	mock.ExpectExec(`UPDATE test_reserved_keyword_models SET id = \?,\"order\" = \?,\"group\" = \?,name = \? WHERE`).
		WithArgs(1, 10, "TestGroup", "TestName", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	model := &TestReservedKeywordModel{Id: 1, Order: 10, Group: "TestGroup", Name: "TestName"}
	err = Update[TestReservedKeywordModel](db, model, "id = ?", 1)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSelect_WithReservedKeywords_DuckDB(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestReservedKeywordModel]())
	RegisterModel[TestReservedKeywordModel](DuckDB)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Return rows with reserved keyword column names
	rows := sqlmock.NewRows([]string{"id", "order", "group", "name"}).
		AddRow(1, 10, "GroupA", "Name1").
		AddRow(2, 20, "GroupB", "Name2")

	mock.ExpectQuery("SELECT \\* FROM test_reserved_keyword_models").WillReturnRows(rows)

	models, err := Select[TestReservedKeywordModel](db, "SELECT * FROM test_reserved_keyword_models")
	require.NoError(t, err)
	assert.Len(t, models, 2)

	assert.Equal(t, 1, models[0].Id)
	assert.Equal(t, 10, models[0].Order)
	assert.Equal(t, "GroupA", models[0].Group)
	assert.Equal(t, "Name1", models[0].Name)

	assert.Equal(t, 2, models[1].Id)
	assert.Equal(t, 20, models[1].Order)
	assert.Equal(t, "GroupB", models[1].Group)
	assert.Equal(t, "Name2", models[1].Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ==================== Custom Driver Tests ====================

// mockDriver is a custom driver implementation for testing extensibility
type mockDriver struct{}

func (d *mockDriver) Name() string { return "MockDB" }

func (d *mockDriver) GenerateInsertQuery(tableName string, columnKeys []string, hasIntId bool) (string, []string) {
	var q []byte
	q = append(q, "INSERT INTO "...)
	q = append(q, tableName...)
	q = append(q, " ("...)
	insertColumns := []string{}
	for i, k := range columnKeys {
		q = append(q, k...)
		if i != len(columnKeys)-1 {
			q = append(q, ',')
		}
	}
	q = append(q, ") VALUES ("...)
	for i, k := range columnKeys {
		if hasIntId && k == "id" {
			q = append(q, "NULL"...)
		} else {
			insertColumns = append(insertColumns, k)
			q = append(q, '?')
		}
		if i != len(columnKeys)-1 {
			q = append(q, ',')
		}
	}
	q = append(q, ')')
	return string(q), insertColumns
}

func (d *mockDriver) GenerateUpdateQuery(tableName string, columnKeys []string) string {
	var q []byte
	q = append(q, "UPDATE "...)
	q = append(q, tableName...)
	q = append(q, " SET "...)
	for i, k := range columnKeys {
		q = append(q, k...)
		q = append(q, " = ?"...)
		if i != len(columnKeys)-1 {
			q = append(q, ',')
		}
	}
	q = append(q, " WHERE "...)
	return string(q)
}

func (d *mockDriver) InsertAndGetId(ex Executor, query string, args ...any) (int, error) {
	result, err := ex.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (d *mockDriver) Placeholder(argIndex int) string              { return "?" }
func (d *mockDriver) SupportsBackslashEscape() bool                { return false }
func (d *mockDriver) RenumberWhereClause(w string, o int) string   { return w }
func (d *mockDriver) JoinStringForIn(offset int, count int) string { return mysqlJoinStringForIn(count) }

func TestCustomDriver_RegisterAndInsert(t *testing.T) {
	type CustomUser struct {
		Id    int
		Name  string
		Email string
	}
	delete(StructToFieldMap, reflect.TypeFor[CustomUser]())

	custom := &mockDriver{}
	RegisterModel[CustomUser](custom)

	fieldMap, err := GetFieldMap(reflect.TypeFor[CustomUser]())
	require.NoError(t, err)
	assert.Equal(t, custom, fieldMap.Driver)
	assert.Equal(t, "MockDB", fieldMap.Driver.Name())

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO custom_users").
		WithArgs("Alice", "alice@example.com").
		WillReturnResult(sqlmock.NewResult(1, 1))

	user := &CustomUser{Name: "Alice", Email: "alice@example.com"}
	id, err := Insert[CustomUser](db, user)
	require.NoError(t, err)
	assert.Equal(t, 1, id)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCustomDriver_ParseNamedQuery(t *testing.T) {
	custom := &mockDriver{}
	q, args, err := ParseNamedQuery(custom, "SELECT * FROM users WHERE id = :id", map[string]any{"id": 42})
	require.NoError(t, err)
	assert.Equal(t, "SELECT * FROM users WHERE id = ?", q)
	assert.Equal(t, []any{42}, args)
}
