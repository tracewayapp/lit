package models

/*
PostgreSQL:
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL
	);

DuckDB (auto-increment id is sequence-backed; lit reads it back via RETURNING id):
	CREATE SEQUENCE IF NOT EXISTS seq_users_id START 1;
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY DEFAULT nextval('seq_users_id'),
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL
	);
*/

type User struct {
	Id        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}
