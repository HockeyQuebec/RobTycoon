package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("Testing go-update...")
	resp, err := http.Get("https://github.com/inconshreveable/go-update")
	if err != nil {
		fmt.Println("Failed to fetch:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Library imported and HTTP request succeeded!")
}
