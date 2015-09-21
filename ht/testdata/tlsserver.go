package main

// +build ignore

import (
	"io"
	"log"
	"net/http"
)

func helloHandler(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "Hello world")
}

func main() {
	http.HandleFunc("/hello", helloHandler)
	err := http.ListenAndServeTLS(":4443", "cert.pem", "key.pem", nil)
	if err != nil {
		log.Fatal("ListenAndServeTLS: ", err)
	}
}
