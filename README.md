<p align="center">
  <a href="https://lit.tracewayapp.com">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="logo-white.png" />
      <source media="(prefers-color-scheme: light)" srcset="logo-black.png" />
      <img src="logo-black.png" alt="lit logo" width="150" />
    </picture>
  </a>
</p>

# lit

`lit` is a lightweight, high-performance database interaction library for Go. It is designed to be slim, fast, and easy to use, especially when working with projections and Data Transfer Objects (DTOs).

The project is currently used in a **production environment**.

## Key Features

- **Unified API**: The `lit` package provides a single API for PostgreSQL, MySQL, SQLite, DuckDB, and custom database drivers, with driver-specific optimizations handled internally.
- **Lightweight Projections**: The biggest advantage of `lit` is its ability to load DTOs and projections with minimal effort. Regardless of your table structure, mapping a query result to a Go struct is straightforward and clean.
- **MySQL, PostgreSQL, SQLite, and DuckDB Support**: Register your models with the appropriate driver and the library handles query generation and driver-specific optimizations.
- **Generic CRUD Operations**: Automatic generation of `INSERT` and `UPDATE` queries for registered types.
- **Works with DB and Tx**: All operations accept both `*sql.DB` and `*sql.Tx` via the `Executor` interface.
- **Minimal Dependencies**: Keeps your project slim and focused.

## Docs

Documentation is available at https://lit.tracewayapp.com

## AI Agent Setup

lit ships a skill that teaches AI coding agents (Claude Code, Cursor, Codex, and similar) the full API and its pitfalls, so they write correct lit code in your project.

- **Claude Code plugin**: `/plugin marketplace add tracewayapp/lit`, then `/plugin install lit@lit`
- **Copy into your repo**: put [skills/lit/SKILL.md](skills/lit/SKILL.md) at `.claude/skills/lit/SKILL.md` in your project
- **CLAUDE.md / AGENTS.md**: paste the condensed section from the [AI Agent Setup guide](https://lit.tracewayapp.com/guides/ai-setup)
- **llms.txt**: a machine-readable API summary is served at https://lit.tracewayapp.com/llms.txt

See the [AI Agent Setup guide](https://lit.tracewayapp.com/guides/ai-setup) for details.

## Usage Limitations

- **ID Column Requirement**: For automatic `Insert` and `Update` operations, the library requires your tables to have an `id` column. It does not support tables without a primary `id` field for these specific automatic operations.

## Installation

```bash
go get github.com/tracewayapp/lit/v2
```

## Configuration & Usage

### 1. Registration

Every model you intend to use with generic functions must be registered with a specific driver.

```go
import "github.com/tracewayapp/lit/v2"

type User struct {
    Id        int
    FirstName string
    LastName  string
    Email     string
}

func init() {
    // Register for PostgreSQL
    lit.RegisterModel[User](lit.PostgreSQL)
    // OR Register for MySQL
    lit.RegisterModel[User](lit.MySQL)
    // OR Register for SQLite
    lit.RegisterModel[User](lit.SQLite)
    // OR Register for DuckDB
    lit.RegisterModel[User](lit.DuckDB)
}
```

**Placeholder Syntax:** PostgreSQL uses `$1, $2, $3...` placeholders while MySQL, SQLite, and DuckDB use `?` placeholders. The examples below use PostgreSQL syntax.

> **DuckDB note:** DuckDB uses `?` placeholders like SQLite/MySQL, but its Go driver does not support `LastInsertId()`. lit reads generated ids back with a `RETURNING id` clause (as it does for PostgreSQL), so auto-increment `id` columns should be sequence-backed (`DEFAULT nextval(...)`).

### 2. Basic Usage

```go
import (
    "github.com/tracewayapp/lit/v2"
    _ "github.com/jackc/pgx/v5/stdlib"
)

func example(db *sql.DB) {
    // Insert - returns auto-generated ID
    id, _ := lit.Insert(db, &User{FirstName: "Jane", LastName: "Smith"})

    // Select Single
    user, _ := lit.SelectSingle[User](db, "SELECT * FROM users WHERE id = $1", id)

    // Select Multiple
    users, _ := lit.Select[User](db, "SELECT * FROM users WHERE last_name = $1", "Smith")

    // Update
    user.Email = "jane@example.com"
    _ = lit.Update(db, user, "id = $1", user.Id)

    // Delete
    _ = lit.Delete(db, "DELETE FROM users WHERE id = $1", user.Id)
}
```

### 3. Working with Transactions

All operations work with both `*sql.DB` and `*sql.Tx`:

```go
func exampleWithTx(db *sql.DB) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // All operations accept tx
    id, err := lit.Insert(tx, &User{FirstName: "John"})
    if err != nil {
        return err
    }

    user, err := lit.SelectSingle[User](tx, "SELECT * FROM users WHERE id = $1", id)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

### 4. UUID Support

For models with string ID fields, use UUID-specific insert functions:

```go
type Product struct {
    Id    string
    Name  string
    Price int
}

func init() {
    lit.RegisterModel[Product](lit.PostgreSQL)
}

func example(db *sql.DB) {
    // Auto-generate UUID
    uuid, _ := lit.InsertUuid(db, &Product{Name: "Widget", Price: 100})

    // Use existing UUID
    product := &Product{Id: "my-custom-uuid", Name: "Gadget", Price: 200}
    _ = lit.InsertExistingUuid(db, product)
}
```

### 5. Named Parameters

Write portable queries with `:name` placeholders. lit automatically converts them to the correct driver syntax (`$1` for PostgreSQL, `?` for MySQL/SQLite/DuckDB):

```go
// Use lit.P as a shorthand for map[string]any
users, _ := lit.SelectNamed[User](db,
    "SELECT * FROM users WHERE last_name = :last_name AND email = :email",
    lit.P{"last_name": "Doe", "email": "john@example.com"})

// Single result
user, _ := lit.SelectSingleNamed[User](db,
    "SELECT * FROM users WHERE id = :id",
    lit.P{"id": 1})

// Update with named WHERE clause
user.Email = "jane@example.com"
_ = lit.UpdateNamed(db, user,
    "id = :id",
    lit.P{"id": 1})

// Delete (requires explicit driver since Delete is non-generic)
_ = lit.DeleteNamed(lit.PostgreSQL, db,
    "DELETE FROM users WHERE id = :id",
    lit.P{"id": 1})
```

For advanced use, you can parse named queries manually:

```go
// Using model's registered driver
query, args, err := lit.ParseNamedQueryForModel[User](
    "SELECT * FROM users WHERE id = :id", lit.P{"id": 1})

// Using explicit driver
query, args, err := lit.ParseNamedQuery(lit.PostgreSQL,
    "SELECT * FROM users WHERE id = :id", lit.P{"id": 1})
```

The parser handles PostgreSQL `::` type casts, string literals, and repeated parameters correctly.

### 6. Helper Functions

```go
// JoinForIn - for integer IN clauses
ids := []int{1, 2, 3}
query := fmt.Sprintf("SELECT * FROM users WHERE id IN (%s)", lit.JoinForIn(ids))

// JoinStringForIn - generates driver-appropriate placeholders
// PostgreSQL: $1,$2,$3 (with offset support)
// MySQL: ?,?,?
names := []string{"a", "b", "c"}
placeholders := lit.JoinStringForIn[User](0, names)

// Or specify driver explicitly
placeholders := lit.JoinStringForInWithDriver(lit.PostgreSQL, 0, 3) // "$1,$2,$3"
placeholders := lit.JoinStringForInWithDriver(lit.MySQL, 0, 3)      // "?,?,?"
```

### 7. Column Naming

By default, `lit` converts Go struct field names from CamelCase to snake_case for database column names:

| Struct Field | Database Column |
| ------------ | --------------- |
| `Id`         | `id`            |
| `FirstName`  | `first_name`    |
| `LastName`   | `last_name`     |
| `Email`      | `email`         |

#### Custom Column Names with `lit` Tags

You can override the default naming by using the `lit` struct tag:

```go
type User struct {
    Id        int    `lit:"id"`
    FirstName string `lit:"first_name"`
    LastName  string `lit:"surname"`       // Maps to "surname" instead of "last_name"
    Email     string `lit:"email_address"` // Maps to "email_address" instead of "email"
}
```

The `lit` tag is used for:

- **INSERT queries**: Column names in the generated INSERT statement
- **UPDATE queries**: Column names in the generated UPDATE statement
- **SELECT queries**: Mapping database columns back to struct fields

#### Mixing Tagged and Untagged Fields

You can use `lit` tags on only some fields. Fields without tags use the default snake_case conversion:

```go
type User struct {
    Id          int                       // Uses default: "id"
    FirstName   string `lit:"given_name"` // Uses tag: "given_name"
    LastName    string                    // Uses default: "last_name"
    PhoneNumber string `lit:"phone"`      // Uses tag: "phone"
}
```

#### Custom Naming Strategy

For more control over naming conventions, you can implement the `DbNamingStrategy` interface and use `RegisterModelWithNaming`:

```go
type MyNamingStrategy struct{}

func (m MyNamingStrategy) GetTableNameFromStructName(name string) string {
    return strings.ToLower(name) // e.g., "User" -> "user"
}

func (m MyNamingStrategy) GetColumnNameFromStructName(name string) string {
    return strings.ToLower(name) // e.g., "FirstName" -> "firstname"
}

func init() {
    lit.RegisterModelWithNaming[User](lit.PostgreSQL, MyNamingStrategy{})
}
```

Note: The `lit` tag takes precedence over any naming strategy.

### 8. Custom Drivers

The `Driver` type is an interface, so you can implement your own driver for databases not built in. Your driver must implement all methods of the `Driver` interface:

```go
type cockroachDriver struct{}

func (d *cockroachDriver) Name() string                        { return "CockroachDB" }
func (d *cockroachDriver) Placeholder(argIndex int) string     { return fmt.Sprintf("$%d", argIndex) }
func (d *cockroachDriver) SupportsBackslashEscape() bool       { return false }
// ... implement remaining methods

var CockroachDB lit.Driver = &cockroachDriver{}
```

Then register models with your custom driver:

```go
lit.RegisterModel[User](CockroachDB)
```

See the [Custom Drivers guide](https://lit.tracewayapp.com/guides/custom-drivers) for the full interface definition and a complete example.

## Contributions

We welcome all contributions to the lit project. You can open issues or PR and we will review and promptly merge them.

## Roadmap

- [x] ~~Named query parameters~~
- [x] ~~Add a project homepage~~
- [ ] Add support for composite primary keys
- [x] ~~Escaping SQL keywords for field names and table names~~
- [x] ~~Add support for ClickHouse - we're not doing this as clickhouse has a driver that is basically already doing this~~
- [x] ~~Add more examples - the usercrud example is mostly complete - we'll add more if we get a git issue filed to do so~~
- [x] ~~Add project docs~~
- [x] ~~Add support for named fields `lit:"column_name"`~~

## Project Philosophy

- **Developer Written**: All core logic and architectural decisions were made and implemented by an actual developer.
- **AI Assisted Testing**: AI was utilized to help generate a comprehensive test suite as well as help out with documentation.

## License

MIT
