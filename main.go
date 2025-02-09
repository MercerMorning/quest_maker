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
	makeQuestHandler := &handlers.MakeQuestHandler{DB: db}
	makePlayThroughHandler := &handlers.MakePlayThroughHandler{DB: db}
	getStepHandler := &handlers.GetCurrentStepHandler{DB: db}
	makeChoiceHandler := &handlers.MakeChoiceHandler{DB: db}
	http.Handle("/", rootHandler)
	http.Handle("/make_quest", makeQuestHandler)
	http.Handle("/make_playthrough", makePlayThroughHandler)
	http.Handle("/get_step", getStepHandler)
	http.Handle("/make_choice", makeChoiceHandler)
	fmt.Println("starting server at :8080")
	http.ListenAndServe(":8080", nil)
}
