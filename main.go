package main

import (
	"fmt"
	"net/http"
	"quest_maker/handlers"
)

func main() {
	rootHandler := &handlers.RootHandler{}
	http.Handle("/", rootHandler)
	fmt.Println("starting server at :8080")
	http.ListenAndServe(":8080", nil)
}
