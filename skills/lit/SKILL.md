---
name: lit
description: Guide for writing Go database code with the lit library (github.com/tracewayapp/lit/v2). Use when writing or reviewing queries, inserts, updates, model registration, projections, or repository code in a project that imports lit, or when debugging lit errors such as "non registered model" or "invalid column that is not found in the struct".
---

# Using lit correctly

lit is a small Go library that maps SQL rows to structs and generates INSERT and UPDATE statements for registered types. You write the SQL yourself. lit sits on top of `database/sql` and supports PostgreSQL, MySQL, SQLite, and DuckDB, plus custom drivers.

What lit deliberately does NOT do: transactions (pass your own `*sql.Tx`), migrations, connection management, query building, batch inserts, context-aware calls, or returning `RowsAffected`. When you need one of those, drop down to `database/sql` (see "Escape hatches").

## Function cheat sheet

| Task | Call |
|---|---|
| Many rows | `lit.Select[T](ex, query, args...) ([]*T, error)` |
| One row or nil | `lit.SelectSingle[T](ex, query, args...) (*T, error)` |
| Insert, int id | `lit.Insert(ex, &e) (int, error)` |
| Insert, new UUID | `lit.InsertUuid(ex, &e) (string, error)` |
| Insert, UUID already set | `lit.InsertExistingUuid(ex, &e) error` |
| Full-struct update | `lit.Update(ex, &e, "id = $1", id) error` |
| Raw update/exec | `lit.UpdateNative(ex, query, args...) error` |
| Raw delete/exec | `lit.Delete(ex, query, args...) error` |
| Named-param variants | `lit.SelectNamed[T]`, `lit.SelectSingleNamed[T]`, `lit.UpdateNamed[T]`, `lit.DeleteNamed` |
| Render `:name` to positional | `lit.ParseNamedQuery(driver, query, lit.P{...}) (string, []any, error)` |
| Int IN list (inlined literals) | `lit.JoinForIn([]int{1,2,3})` returns `"1,2,3"` |
| Placeholder IN list | `lit.JoinStringForIn[T](offset, params)` or `lit.JoinStringForInWithDriver(driver, offset, count)` |

`ex` is any `lit.Executor` (`Exec`, `Query`, `QueryRow`). Both `*sql.DB` and `*sql.Tx` satisfy it.

## Registration is mandatory

Every struct passed to a generic lit function must be registered first, including one-off projection and aggregate structs. Forgetting this fails at runtime with "non registered model".

```go
import "github.com/tracewayapp/lit/v2"

type User struct {
    Id        int
    FirstName string
    Email     string
    CreatedAt time.Time
}

func init() {
    lit.RegisterModel[User](lit.PostgreSQL) // or lit.MySQL, lit.SQLite, lit.DuckDB
}
```

You can set a default driver once and drop the argument:

```go
lit.RegisterDriver(lit.PostgreSQL)
lit.RegisterModel[User]() // uses the default driver
```

Keep registration in one predictable place per package. Two proven layouts from production codebases:

- A `lit_registration.go` per package with a single `init()` listing every model and projection struct.
- A central `models.Init(driver lit.Driver)` called at boot, so tests and alternate builds can pass a different driver.

## Naming rules

Default mapping, no tags needed:

- Fields: CamelCase to snake_case, acronyms stay together. `FirstName` -> `first_name`, `UserID` -> `user_id`, `HTTPCode` -> `http_code`.
- Tables: snake_case of the struct name plus `s`. `User` -> `users`, `UserProfile` -> `user_profiles`.

Override a single column with a tag: `LastName string `lit:"surname"``. The tag wins over any naming strategy.

For irregular table names, embed the default strategy and override only what differs:

```go
// Default pluralization would produce "escalation_policys".
type escalationPolicyNaming struct{ lit.DefaultDbNamingStrategy }

func (escalationPolicyNaming) GetTableNameFromStructName(string) string {
    return "escalation_policies"
}

lit.RegisterModelWithNaming[EscalationPolicy](lit.PostgreSQL, escalationPolicyNaming{})
```

## Reading rows

```go
users, err := lit.Select[User](tx, "SELECT id, first_name, email, created_at FROM users WHERE email = $1", email)
user, err := lit.SelectSingle[User](tx, "SELECT * FROM users WHERE id = $1", id)
```

Rules that trip people up:

- **Not found is `nil`, not an error.** `SelectSingle` returns `(nil, nil)` when no row matches. Check `if user == nil`. Never compare against `sql.ErrNoRows` on a lit call.
- **Every returned column must exist on the struct.** lit validates result columns at runtime and fails with "invalid column that is not found in the struct: <col>". Either select explicit columns or make the struct cover everything `SELECT *` returns.
- **Alias computed columns to the derived name.** For a field `MaxInDay float64` write `max(odometer)-min(odometer) AS max_in_day`. For `Count int` write `COUNT(*) AS count`.
- Custom field types work transparently if they implement `sql.Scanner` and `driver.Valuer` (common for JSONB columns and driver-specific time handling). lit just hands field pointers to `rows.Scan`.
- Row-locking is plain SQL: append `FOR UPDATE` to the query.

## Projections and DTOs

Projections are lit's main strength: map any query shape (partial selects, JOINs, aggregates) to a purpose-built struct. Declare the struct next to the query that uses it, register it like any model, and alias SQL columns to match:

```go
type CategoryStats struct {
    CategoryId   int
    ProductCount int
    AvgPrice     float64
}

stats, err := lit.Select[CategoryStats](tx, `
    SELECT c.id AS category_id, COUNT(p.id) AS product_count, AVG(p.price) AS avg_price
    FROM categories c LEFT JOIN products p ON p.category_id = c.id
    GROUP BY c.id`)
```

## Inserting

```go
user := &models.User{
    Email:     email,
    Name:      name,
    CreatedAt: time.Now().UTC(), // see rule below
}
id, err := lit.Insert(tx, user)
if err != nil {
    return err
}
user.Id = id // Insert does not set it for you
```

- **Insert writes every struct field**, so database column DEFAULTs never apply. Set `CreatedAt` and any other defaulted fields explicitly before inserting, or they land as zero values (`0001-01-01`, `""`, `0`).
- Tables must have an `id` column. An `Id int` field means auto-generated integer id (omitted from the INSERT, read back per driver). An `Id string` field means UUID: use `lit.InsertUuid` (generates the UUID and sets it on the struct before returning it) or `lit.InsertExistingUuid` (uses the id already on the struct).
- There is no batch insert. For bulk work, build a multi-row `INSERT ... VALUES (...),(...)` yourself. If it ends in `RETURNING ...`, you can run it through `lit.Select[T]` to map the returned rows; lit does not care that the statement is not a SELECT.

## Updating

`lit.Update` writes every column from the struct, then appends your WHERE condition:

```go
err := lit.Update(tx, user, "id = $1", user.Id)
err := lit.UpdateNamed(tx, user, "id = :id AND organization_id = :org", lit.P{"id": id, "org": orgId})
```

- **lit adds the `WHERE` keyword itself.** Pass only the condition. Passing `"WHERE id = $1"` produces `WHERE WHERE id = $1`.
- **Write WHERE placeholders starting at `$1`** as if they were the only parameters. lit renumbers them past the SET columns internally. Never hand-offset them, and never reference SET-clause placeholders from the WHERE.
- The `where` argument is required; an empty string is an error. There is intentionally no update-without-condition.
- Because `Update` writes the whole struct, always load-modify-save (fetch, mutate fields, update). For partial or expression updates (`SET counter = counter + 1`, `SET deleted_at = NOW()`), write the SQL yourself with `lit.UpdateNative`.

## Deleting

`lit.Delete` and `lit.DeleteNamed` execute a full query you write, so they work for any exec statement (production code uses them for soft-delete UPDATEs too; `Delete` and `UpdateNative` are interchangeable wrappers over `Exec`):

```go
err := lit.Delete(tx, "DELETE FROM users WHERE id = $1", id)
err := lit.DeleteNamed(lit.PostgreSQL, tx, "DELETE FROM users WHERE id = :id", lit.P{"id": id})
```

**Signature gotcha:** `DeleteNamed` takes the driver as the FIRST argument (it is non-generic, so the driver cannot be inferred from a model). Every other named function takes the executor first.

## Named parameters

`:name` placeholders make queries portable across drivers. lit renders them to `$1` or `?` per the driver, taking values from a `lit.P` map (`lit.P` is `map[string]any`):

```go
users, err := lit.SelectNamed[User](tx,
    "SELECT * FROM users WHERE last_name = :last_name AND created_at > :since",
    lit.P{"last_name": name, "since": since})
```

- The parser handles PostgreSQL `::` casts (`:val::text` -> `$1::text`), `:name` inside string literals (left alone), and repeated parameters. A missing map key is an error before anything executes.
- **No IN-list expansion.** `:ids` cannot bind a slice. Options: `lit.JoinForIn` for ints, placeholder helpers for strings, or a fixed number of named params when the list has a known small bound.
- Dynamic filters compose naturally: grow a WHERE string and the `lit.P` map together, then run the final query through a Named function or `ParseNamedQuery`.

Prefer named parameters when a project targets more than one driver (or might). Positional `$1`/`?` is fine for a single-driver codebase.

## IN clauses

```go
// Ints: inlined as literals, NOT placeholders. Safe only because inputs are ints.
if len(ids) == 0 {
    return []*models.User{}, nil // guard: IN () is invalid SQL
}
users, err := lit.Select[User](tx,
    "SELECT * FROM users WHERE id IN ("+lit.JoinForIn(ids)+")")

// Strings: driver-appropriate placeholders, values passed as args.
// offset = number of args that come before the list.
placeholders := lit.JoinStringForIn[User](1, names) // "$2,$3,$4" on PostgreSQL
query := "SELECT * FROM users WHERE org_id = $1 AND name IN (" + placeholders + ")"
args := append([]any{orgId}, toAnySlice(names)...)
```

Always guard against empty slices before either helper.

## Transactions and executors

lit never begins, commits, or rolls back. Repository functions should accept the executor from the caller:

```go
func (r *userRepository) FindById(tx *sql.Tx, id int) (*models.User, error) {
    return lit.SelectSingle[models.User](tx, "SELECT * FROM users WHERE id = $1", id)
}
```

- Take `tx *sql.Tx` as the first parameter for transactional stores.
- Take `ex lit.Executor` when a function must run both inside and outside a transaction.
- Passing a `*sql.DB` directly is fine for non-transactional, append-only stores.
- Wrap transaction lifecycle once, in a helper like `ExecuteTransaction(func(tx *sql.Tx) (T, error))` that commits on success and rolls back on error or panic. lit calls inside just receive the tx.

## Escape hatches

When lit's API does not cover a need, parse the query with lit and execute it with `database/sql` directly:

```go
query, args, err := lit.ParseNamedQuery(driver, namedQuery, params)
if err != nil {
    return err
}

// Context-aware execution (lit's Executor has no *Context methods)
rows, err := db.QueryContext(ctx, query, args...)

// RowsAffected (lit never returns it; needed for guarded/claim updates)
res, err := tx.Exec(query, args...)
n, _ := res.RowsAffected()
```

Also available:

- `lit.SelectMultipleNative[T](ex, mapLine, query, args...)` scans rows through your own callback, no registration needed. Useful for one-off aggregates.
- `lit.InsertNative(ex, query, args...)` runs a hand-written INSERT and returns `LastInsertId` (MySQL/SQLite style only).
- `lit.ParseNamedQueryForModel[T](query, params)` infers the driver from the model's registration.

## Driver differences

| | PostgreSQL | MySQL | SQLite | DuckDB |
|---|---|---|---|---|
| Placeholders | `$1, $2` | `?` | `?` | `?` |
| Generated id read via | `RETURNING id` | `LastInsertId()` | `LastInsertId()` | `RETURNING id` |
| Keyword escaping | `"quotes"` | `` `backticks` `` | `"quotes"` | `"quotes"` |

Reserved SQL keywords in table and column names are escaped automatically. DuckDB's Go driver has no `LastInsertId`, so auto-increment `id` columns must be sequence-backed (`DEFAULT nextval(...)`) for `lit.Insert` to read the id back. For other databases, implement the `lit.Driver` interface and register models against your custom driver.

## Recommended project conventions

Patterns proven in production codebases built on lit:

- SQL lives only in repository files (`<entity>.repository.go`). Controllers and services never touch lit directly.
- Repository shape: unexported struct, methods on it, one exported package-level singleton:

  ```go
  type userRepository struct{}

  func (r *userRepository) FindById(tx *sql.Tx, id int) (*models.User, error) { ... }

  var UserRepository = userRepository{}
  ```

- Method naming: `FindById`, `FindAllByX`, `FindByIdForUpdate` (row locking), `Create`, `Update`, `Delete`.
- `Create` assigns the generated id back: `id, err := lit.Insert(tx, e); e.Id = id`.
- Hoist shared column lists into a package const (`const userColumns = "id, email, name, created_at"`).
- Set timestamps in Go (`time.Now().UTC()`), never rely on DB defaults, because Insert writes every field.
- Multi-driver projects: keep one package-level `var Driver lit.Driver`, set it at boot, thread it into every `ParseNamedQuery` and `DeleteNamed` call, and use `:name` params everywhere so repository code stays dialect-neutral.
- Structs used only as query results get registered right next to their models, in the same registration file, so nothing is forgotten.

## Pitfall checklist

Scan this before writing or reviewing lit code:

1. Every struct used with a generic function is registered, projections included.
2. Not found means `nil`, never `sql.ErrNoRows`.
3. `Insert` writes every field: `CreatedAt` and other defaulted columns are set explicitly.
4. The generated id is assigned back to the struct after `Insert` (`InsertUuid` does it for you).
5. `Update` prepends `WHERE` itself: pass `"id = $1"`, never `"WHERE id = $1"`.
6. `Update` WHERE placeholders start at `$1` and are self-contained; lit renumbers them.
7. Partial updates go through `UpdateNative`, not through `Update` with a sparsely-filled struct.
8. `DeleteNamed` takes the driver first, executor second.
9. Empty slices are guarded before `JoinForIn` or `JoinStringForIn` (`IN ()` is invalid SQL).
10. Computed and aggregate columns are aliased to match struct fields (`COUNT(*) AS count`).
11. Placeholders match the driver: `$N` for PostgreSQL, `?` for MySQL/SQLite/DuckDB. `:name` params are portable.
12. Context, `RowsAffected`, and batch inserts need the `ParseNamedQuery` + `database/sql` escape hatch.
13. Tables have an `id` column when `Insert`/`Update` are used on them.
14. New model names respect the naming rules (or carry `lit:` tags), or generated SQL will not match the schema.
