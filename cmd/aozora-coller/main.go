package main

import (
	"fmt"
	"log"
)

type Entry struct {
	AuthorID string
	Author   string
	TitleID  string
	Title    string
	InfoURL  string
	ZipURL   string
}

func findEntities(siteURL string) ([]Entry, error) {
	// 処理
	return nil, nil
}

func main() {
	listURL := "https://www.aozora.gr.jp/index_pages/person1257.html"
	entries, err := findEntities(listURL)
	if err != nil {
		log.Fatal(err)
	}
	for _, entry := range entries {
		fmt.Println(entry.Title, entry.ZipURL)
	}
}
