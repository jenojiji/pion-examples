package main

import (
	"fmt"
	"log"
	"net/http"
)


func main() {
	server := NewServer()
	http.HandleFunc("/ws", server.HandleWS)
	fmt.Println("Server started")
	log.Fatal(http.ListenAndServe(":9091", nil))
}
