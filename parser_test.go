package lit

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNamedQuery(t *testing.T) {
	t.Run("basic per-driver", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(PostgreSQL, "SELECT * FROM users WHERE id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = $1", q)
		assert.Equal(t, []any{1}, args)

		q, args, err = ParseNamedQuery(MySQL, "SELECT * FROM users WHERE id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = ?", q)
		assert.Equal(t, []any{1}, args)

		q, args, err = ParseNamedQuery(SQLite, "SELECT * FROM users WHERE id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = ?", q)
		assert.Equal(t, []any{1}, args)

		q, args, err = ParseNamedQuery(DuckDB, "SELECT * FROM users WHERE id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = ?", q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("multiple params", func(t *testing.T) {
		params := map[string]any{"id": 1, "email": "john@example.com"}

		q, args, err := ParseNamedQuery(PostgreSQL,
			"SELECT * FROM users WHERE id = :id AND email = :email", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = $1 AND email = $2", q)
		assert.Equal(t, []any{1, "john@example.com"}, args)

		q, args, err = ParseNamedQuery(MySQL,
			"SELECT * FROM users WHERE id = :id AND email = :email", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = ? AND email = ?", q)
		assert.Equal(t, []any{1, "john@example.com"}, args)
	})

	t.Run("repeated params", func(t *testing.T) {
		params := map[string]any{"id": 42}

		q, args, err := ParseNamedQuery(PostgreSQL,
			"SELECT * FROM users WHERE id = :id OR parent_id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = $1 OR parent_id = $2", q)
		assert.Equal(t, []any{42, 42}, args)

		q, args, err = ParseNamedQuery(MySQL,
			"SELECT * FROM users WHERE id = :id OR parent_id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = ? OR parent_id = ?", q)
		assert.Equal(t, []any{42, 42}, args)
	})

	t.Run("string literals not replaced", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(PostgreSQL,
			"SELECT * FROM users WHERE name = ':not_a_param' AND id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE name = ':not_a_param' AND id = $1", q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("escaped quotes in string literals", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(PostgreSQL,
			"SELECT * FROM users WHERE name = 'it''s :val' AND id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE name = 'it''s :val' AND id = $1", q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("PG type casts", func(t *testing.T) {
		params := map[string]any{"val": "123"}

		q, args, err := ParseNamedQuery(PostgreSQL,
			"SELECT :val::text", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT $1::text", q)
		assert.Equal(t, []any{"123"}, args)

		q, args, err = ParseNamedQuery(PostgreSQL,
			"SELECT :val::numeric::text", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT $1::numeric::text", q)
		assert.Equal(t, []any{"123"}, args)

		// Double colon without param
		q, _, err = ParseNamedQuery(PostgreSQL,
			"SELECT name::text FROM users", nil)
		require.NoError(t, err)
		assert.Equal(t, "SELECT name::text FROM users", q)
	})

	t.Run("missing param error", func(t *testing.T) {
		params := map[string]any{"id": 1}

		_, _, err := ParseNamedQuery(PostgreSQL,
			"SELECT * FROM users WHERE id = :id AND email = :email", params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing parameter: email")

		// Nil map with named params
		_, _, err = ParseNamedQuery(PostgreSQL,
			"SELECT * FROM users WHERE id = :id", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing parameter: id")
	})

	t.Run("bare colons", func(t *testing.T) {
		// Colon followed by space
		q, args, err := ParseNamedQuery(PostgreSQL, "SELECT 1 : 2", nil)
		require.NoError(t, err)
		assert.Equal(t, "SELECT 1 : 2", q)
		assert.Empty(t, args)

		// Colon at end of string
		q, args, err = ParseNamedQuery(PostgreSQL, "SELECT 1:", nil)
		require.NoError(t, err)
		assert.Equal(t, "SELECT 1:", q)
		assert.Empty(t, args)

		// Colon followed by digit (not a param)
		q, args, err = ParseNamedQuery(PostgreSQL, "SELECT :1abc", nil)
		require.NoError(t, err)
		assert.Equal(t, "SELECT :1abc", q)
		assert.Empty(t, args)
	})

	t.Run("complex mixed", func(t *testing.T) {
		params := map[string]any{"id": 1, "name": "test"}

		q, args, err := ParseNamedQuery(PostgreSQL,
			"SELECT * FROM users WHERE name = ':skip' AND id = :id AND label::text = :name::text", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE name = ':skip' AND id = $1 AND label::text = $2::text", q)
		assert.Equal(t, []any{1, "test"}, args)
	})

	t.Run("param boundaries", func(t *testing.T) {
		params := map[string]any{"id": 1, "name": "test"}

		// Param adjacent to closing paren
		q, args, err := ParseNamedQuery(PostgreSQL,
			"WHERE id IN (:id)", params)
		require.NoError(t, err)
		assert.Equal(t, "WHERE id IN ($1)", q)
		assert.Equal(t, []any{1}, args)

		// Param adjacent to comma
		q, args, err = ParseNamedQuery(PostgreSQL,
			"VALUES (:id,:name)", params)
		require.NoError(t, err)
		assert.Equal(t, "VALUES ($1,$2)", q)
		assert.Equal(t, []any{1, "test"}, args)

		// Param at end of string
		q, args, err = ParseNamedQuery(PostgreSQL,
			"WHERE id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "WHERE id = $1", q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("empty and nil inputs", func(t *testing.T) {
		q, args, err := ParseNamedQuery(PostgreSQL, "", nil)
		require.NoError(t, err)
		assert.Equal(t, "", q)
		assert.Empty(t, args)

		q, args, err = ParseNamedQuery(PostgreSQL, "SELECT 1", nil)
		require.NoError(t, err)
		assert.Equal(t, "SELECT 1", q)
		assert.Empty(t, args)
	})

	t.Run("nil driver", func(t *testing.T) {
		_, _, err := ParseNamedQuery(nil, "SELECT :id", map[string]any{"id": 1})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "driver is nil")
	})
}

func TestParseNamedQueryForModel(t *testing.T) {
	t.Run("PostgreSQL", func(t *testing.T) {
		delete(StructToFieldMap, reflect.TypeFor[TestUser]())
		RegisterModel[TestUser](PostgreSQL)

		q, args, err := ParseNamedQueryForModel[TestUser](
			"SELECT * FROM users WHERE id = :id", map[string]any{"id": 1})
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = $1", q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("MySQL", func(t *testing.T) {
		delete(StructToFieldMap, reflect.TypeFor[TestUser]())
		RegisterModel[TestUser](MySQL)

		q, args, err := ParseNamedQueryForModel[TestUser](
			"SELECT * FROM users WHERE id = :id", map[string]any{"id": 1})
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = ?", q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("SQLite", func(t *testing.T) {
		delete(StructToFieldMap, reflect.TypeFor[TestUser]())
		RegisterModel[TestUser](SQLite)

		q, args, err := ParseNamedQueryForModel[TestUser](
			"SELECT * FROM users WHERE id = :id", map[string]any{"id": 1})
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = ?", q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("DuckDB", func(t *testing.T) {
		delete(StructToFieldMap, reflect.TypeFor[TestUser]())
		RegisterModel[TestUser](DuckDB)

		q, args, err := ParseNamedQueryForModel[TestUser](
			"SELECT * FROM users WHERE id = :id", map[string]any{"id": 1})
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = ?", q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("unregistered model", func(t *testing.T) {
		type Unregistered struct{ Id int }
		delete(StructToFieldMap, reflect.TypeFor[Unregistered]())

		_, _, err := ParseNamedQueryForModel[Unregistered](
			"SELECT * FROM x WHERE id = :id", map[string]any{"id": 1})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "non registered model")
	})
}

func TestSelectNamed(t *testing.T) {
	t.Run("PostgreSQL", func(t *testing.T) {
		delete(StructToFieldMap, reflect.TypeFor[TestUser]())
		RegisterModel[TestUser](PostgreSQL)

		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
			AddRow(1, "John", "Doe", "john@example.com").
			AddRow(2, "Jane", "Smith", "jane@example.com")

		mock.ExpectQuery("SELECT \\* FROM test_users WHERE last_name = \\$1").
			WithArgs("Doe").
			WillReturnRows(rows)

		users, err := SelectNamed[TestUser](db,
			"SELECT * FROM test_users WHERE last_name = :last_name",
			map[string]any{"last_name": "Doe"})
		require.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, "John", users[0].FirstName)
		assert.Equal(t, "Jane", users[1].FirstName)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("MySQL", func(t *testing.T) {
		delete(StructToFieldMap, reflect.TypeFor[TestUser]())
		RegisterModel[TestUser](MySQL)

		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
			AddRow(1, "John", "Doe", "john@example.com")

		mock.ExpectQuery("SELECT \\* FROM test_users WHERE last_name = \\?").
			WithArgs("Doe").
			WillReturnRows(rows)

		users, err := SelectNamed[TestUser](db,
			"SELECT * FROM test_users WHERE last_name = :last_name",
			map[string]any{"last_name": "Doe"})
		require.NoError(t, err)
		assert.Len(t, users, 1)
		assert.Equal(t, "John", users[0].FirstName)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SQLite", func(t *testing.T) {
		delete(StructToFieldMap, reflect.TypeFor[TestUser]())
		RegisterModel[TestUser](SQLite)

		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
			AddRow(1, "John", "Doe", "john@example.com")

		mock.ExpectQuery("SELECT \\* FROM test_users WHERE last_name = \\?").
			WithArgs("Doe").
			WillReturnRows(rows)

		users, err := SelectNamed[TestUser](db,
			"SELECT * FROM test_users WHERE last_name = :last_name",
			map[string]any{"last_name": "Doe"})
		require.NoError(t, err)
		assert.Len(t, users, 1)
		assert.Equal(t, "John", users[0].FirstName)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DuckDB", func(t *testing.T) {
		delete(StructToFieldMap, reflect.TypeFor[TestUser]())
		RegisterModel[TestUser](DuckDB)

		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		rows := sqlmock.NewRows([]string{"id", "first_name", "last_name", "email"}).
			AddRow(1, "John", "Doe", "john@example.com")

		mock.ExpectQuery("SELECT \\* FROM test_users WHERE last_name = \\?").
			WithArgs("Doe").
			WillReturnRows(rows)

		users, err := SelectNamed[TestUser](db,
			"SELECT * FROM test_users WHERE last_name = :last_name",
			map[string]any{"last_name": "Doe"})
		require.NoError(t, err)
		assert.Len(t, users, 1)
		assert.Equal(t, "John", users[0].FirstName)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSelectSingleNamed(t *testing.T) {
	t.Run("PostgreSQL", func(t *testing.T) {
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

		user, err := SelectSingleNamed[TestUser](db,
			"SELECT * FROM test_users WHERE id = :id",
			map[string]any{"id": 1})
		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, 1, user.Id)
		assert.Equal(t, "John", user.FirstName)
		assert.Equal(t, "Doe", user.LastName)
		assert.Equal(t, "john@example.com", user.Email)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSelectNamed_Error(t *testing.T) {
	delete(StructToFieldMap, reflect.TypeFor[TestUser]())
	RegisterModel[TestUser](PostgreSQL)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	users, err := SelectNamed[TestUser](db,
		"SELECT * FROM test_users WHERE id = :id AND email = :email",
		map[string]any{"id": 1})
	assert.Error(t, err)
	assert.Nil(t, users)
	assert.Contains(t, err.Error(), "missing parameter: email")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteNamed(t *testing.T) {
	t.Run("PostgreSQL", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("DELETE FROM test_users WHERE id = \\$1").
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err = DeleteNamed(PostgreSQL, db,
			"DELETE FROM test_users WHERE id = :id",
			map[string]any{"id": 1})
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("MySQL", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("DELETE FROM test_users WHERE id = \\?").
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err = DeleteNamed(MySQL, db,
			"DELETE FROM test_users WHERE id = :id",
			map[string]any{"id": 1})
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SQLite", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("DELETE FROM test_users WHERE id = \\?").
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err = DeleteNamed(SQLite, db,
			"DELETE FROM test_users WHERE id = :id",
			map[string]any{"id": 1})
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DuckDB", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("DELETE FROM test_users WHERE id = \\?").
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err = DeleteNamed(DuckDB, db,
			"DELETE FROM test_users WHERE id = :id",
			map[string]any{"id": 1})
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDeleteNamed_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	err = DeleteNamed(PostgreSQL, db,
		"DELETE FROM test_users WHERE id = :id AND email = :email",
		map[string]any{"id": 1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing parameter: email")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateNamed(t *testing.T) {
	t.Run("PostgreSQL", func(t *testing.T) {
		delete(StructToFieldMap, reflect.TypeFor[TestUser]())
		RegisterModel[TestUser](PostgreSQL)

		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("UPDATE test_users SET").
			WithArgs(1, "John", "Doe", "john@example.com", 1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		user := &TestUser{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
		err = UpdateNamed[TestUser](db, user, "id = :id", map[string]any{"id": 1})
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("MySQL", func(t *testing.T) {
		delete(StructToFieldMap, reflect.TypeFor[TestUser]())
		RegisterModel[TestUser](MySQL)

		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec("UPDATE test_users SET").
			WithArgs(1, "John", "Doe", "john@example.com", 1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		user := &TestUser{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
		err = UpdateNamed[TestUser](db, user, "id = :id", map[string]any{"id": 1})
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Error", func(t *testing.T) {
		delete(StructToFieldMap, reflect.TypeFor[TestUser]())
		RegisterModel[TestUser](PostgreSQL)

		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		user := &TestUser{Id: 1, FirstName: "John", LastName: "Doe", Email: "john@example.com"}
		err = UpdateNamed[TestUser](db, user, "id = :id AND email = :email",
			map[string]any{"id": 1})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing parameter: email")

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestParseNamedQuery_QuotingEdgeCases(t *testing.T) {
	t.Run("double quoted string MySQL", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(MySQL,
			`SELECT * FROM users WHERE id = :id AND name = "has :param inside"`, params)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM users WHERE id = ? AND name = "has :param inside"`, q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("double quoted identifier PG", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(PostgreSQL,
			`SELECT * FROM "my:table" WHERE id = :id`, params)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM "my:table" WHERE id = $1`, q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("double quoted escape", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(PostgreSQL,
			`SELECT * FROM "col""name:fake" WHERE id = :id`, params)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM "col""name:fake" WHERE id = $1`, q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("backtick identifier MySQL", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(MySQL,
			"SELECT * FROM `col:name` WHERE id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM `col:name` WHERE id = ?", q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("backtick escape", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(MySQL,
			"SELECT * FROM `col``name:fake` WHERE id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM `col``name:fake` WHERE id = ?", q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("backslash escape MySQL", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(MySQL,
			`SELECT * FROM users WHERE name = 'it\'s :param' AND id = :id`, params)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM users WHERE name = 'it\'s :param' AND id = ?`, q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("backslash escape PG uses SQL standard", func(t *testing.T) {
		// In PostgreSQL, backslash is not an escape character by default.
		// 'it\'  is a complete string (ending at \'), then s is outside, then :param is a param.
		params := map[string]any{"param": "val"}

		q, args, err := ParseNamedQuery(PostgreSQL,
			`SELECT 'it\' || :param`, params)
		require.NoError(t, err)
		assert.Equal(t, `SELECT 'it\' || $1`, q)
		assert.Equal(t, []any{"val"}, args)
	})

	t.Run("mixed quoting", func(t *testing.T) {
		params := map[string]any{"id": 1, "name": "test"}

		q, args, err := ParseNamedQuery(MySQL,
			"SELECT * FROM `my:table` WHERE name = ':skip' AND label = \"has :fake\" AND id = :id AND val = :name", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM `my:table` WHERE name = ':skip' AND label = \"has :fake\" AND id = ? AND val = ?", q)
		assert.Equal(t, []any{1, "test"}, args)
	})

	t.Run("backslash escaped double quote in double quoted string MySQL", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(MySQL,
			`SELECT * FROM users WHERE name = "say \"hello\" :param" AND id = :id`, params)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM users WHERE name = "say \"hello\" :param" AND id = ?`, q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("backslash backslash in double quoted string MySQL", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(MySQL,
			`SELECT * FROM users WHERE name = "\\" AND id = :id`, params)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM users WHERE name = "\\" AND id = ?`, q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("four single quotes", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(PostgreSQL,
			"SELECT * FROM users WHERE name = '''' AND id = :id", params)
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE name = '''' AND id = $1", q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("single quote inside double quotes", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(MySQL,
			`SELECT * FROM users WHERE id = :id AND name = "it's :param"`, params)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM users WHERE id = ? AND name = "it's :param"`, q)
		assert.Equal(t, []any{1}, args)
	})

	t.Run("double quote inside single quotes", func(t *testing.T) {
		params := map[string]any{"id": 1}

		q, args, err := ParseNamedQuery(PostgreSQL,
			`SELECT * FROM users WHERE id = :id AND name = 'say "hello :param"'`, params)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM users WHERE id = $1 AND name = 'say "hello :param"'`, q)
		assert.Equal(t, []any{1}, args)
	})
}

func TestTypeP(t *testing.T) {
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

	// Use P instead of map[string]any
	users, err := SelectNamed[TestUser](db,
		"SELECT * FROM test_users WHERE id = :id",
		P{"id": 1})
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "John", users[0].FirstName)

	assert.NoError(t, mock.ExpectationsWereMet())
}
