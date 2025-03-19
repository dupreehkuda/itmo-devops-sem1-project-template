package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)

type response struct {
	TotalItems      int     `json:"total_items"`
	TotalCategories int     `json:"total_categories"`
	TotalPrice      float64 `json:"total_price"`
}

var (
	hostPsql     = os.Getenv("PSQL_HOST")
	portPsql     = os.Getenv("PSQL_PORT")
	userPsql     = os.Getenv("PSQL_USER")
	passwordPsql = os.Getenv("PSQL_PASSWORD")
	dbnameDbPsql = os.Getenv("PSQL_DB_NAME")
)

var postgresDb *sql.DB

func setupDatabase() (*sql.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		hostPsql, portPsql, userPsql, passwordPsql, dbnameDbPsql)

	postgresDb, err := sql.Open("postgres", psqlInfo)

	if err != nil {
		return nil, err
	}

	err = postgresDb.Ping()
	if err != nil {
		return nil, err
	}

	fmt.Println("Successfully connected to database!")
	return postgresDb, nil
}

func handlerRequests(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Handle POST request
		getZipRequest(w, r)
	case http.MethodPost:
		// Handle GET request
		postZipRequest(w, r)
	default:
		// Method not allowed
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	var err error
	postgresDb, err = setupDatabase()
	if err != nil {
		panic(err)
	}
	defer postgresDb.Close()

	http.HandleFunc("/api/v0/prices", handlerRequests)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
