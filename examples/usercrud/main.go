package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"usercrud/connections"
	"usercrud/controllers"
	"usercrud/models"

	"github.com/tracewayapp/lit/v2"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/marcboeker/go-duckdb/v2"
)

func main() {
	driver := os.Getenv("DB_DRIVER")
	dsn := os.Getenv("DB_DSN")

	if driver == "" {
		driver = "pgx"
	}
	if dsn == "" {
		dsn = "postgres://trux:@localhost:5432/testing?sslmode=disable"
	}

	if driver == "mysql" {
		lit.RegisterModel[models.User](lit.MySQL)
	} else if driver == "duckdb" {
		lit.RegisterModel[models.User](lit.DuckDB)
	} else {
		lit.RegisterModel[models.User](lit.PostgreSQL)
	}

	connections.InitDB(driver, dsn)
	defer connections.CleanupDB()

	http.HandleFunc("GET /users", controllers.UserController.ListUsers)
	http.HandleFunc("GET /users/{id}", controllers.UserController.GetUser)
	http.HandleFunc("POST /users", controllers.UserController.CreateUser)
	http.HandleFunc("PUT /users/{id}", controllers.UserController.UpdateUser)
	http.HandleFunc("DELETE /users/{id}", controllers.UserController.DeleteUser)

	fmt.Println("Server starting on :8080")
	fmt.Println("Endpoints:")
	fmt.Println("  GET    /users      - List all users")
	fmt.Println("  GET    /users/{id} - Get user by ID")
	fmt.Println("  POST   /users      - Create user")
	fmt.Println("  PUT    /users/{id} - Update user")
	fmt.Println("  DELETE /users/{id} - Delete user")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
