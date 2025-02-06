package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"net/http"
	"quest_maker/handlers"
)

func main() {
	dsn := "user=user password=password dbname=quest host=postgres port=5432 sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(10)

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	rows, err := db.Query("SELECT 1")

	if err != nil {
		panic(err)
	}

	fmt.Println(rows)

	rootHandler := &handlers.RootHandler{}
	http.Handle("/", rootHandler)
	fmt.Println("starting server at :8080")
	http.ListenAndServe(":8080", nil)
}
