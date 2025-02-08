package main

import (
	"database/sql"
	"embed"
	"fmt"
	_ "github.com/lib/pq"
	"net/http"
	"quest_maker/handlers"
)

const migrationsDir = "migrations"

//go:embed migrations/*.sql
var MigrationsFS embed.FS

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

	//defer db.Close()

	rows, err := db.Query("SELECT 1")

	if err != nil {
		panic(err)
	}

	fmt.Println(rows)

	//migrator := migrator.MustGetNewMigrator(MigrationsFS, migrationsDir)

	//err = migrator.ApplyMigrations(db)
	//if err != nil {
	//	panic(err)
	//}

	//fmt.Printf("Migrations applied!!")

	rootHandler := &handlers.RootHandler{}
	questHandler := &handlers.QuestHandler{DB: db}
	http.Handle("/", rootHandler)
	http.Handle("/make_quest", questHandler)
	fmt.Println("starting server at :8080")
	http.ListenAndServe(":8080", nil)
}
