package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"

	_ "github.com/mattn/go-sqlite3"
)

type Entry struct {
	AuthorID string
	Author   string
	TitleID  string
	Title    string
	SiteURL  string
	ZipURL   string
}

func findEntities(siteURL string) ([]Entry, error) {
	doc, err := goquery.NewDocument(siteURL)
	if err != nil {
		return nil, err
	}
	pat := regexp.MustCompile(`.*/cards/([0-9]+)/card([0-9]+).html$`)
	entries := []Entry{}
	doc.Find("ol li a").Each(func(n int, elem *goquery.Selection) {
		token := pat.FindStringSubmatch(elem.AttrOr("href", ""))
		if len(token) != 3 {
			return
		}
		pageURL := fmt.Sprintf("https://www.aozora.gr.jp/cards/%s/card%s.html", token[1], token[2])
		author, zipURL := findAuthorAndZIP(pageURL)
		if zipURL != "" {
			entries = append(entries, Entry{
				AuthorID: token[1],
				Author:   author,
				TitleID:  token[2],
				SiteURL:  siteURL,
				ZipURL:   zipURL,
			})
		}
	})
	// 処理
	return entries, nil
}

func findAuthorAndZIP(siteURL string) (string, string) {
	doc, err := goquery.NewDocument(siteURL)
	if err != nil {
		return "", ""
	}
	author := doc.Find("table[summary=作家データ] tr:nth-child(1) td:nth-child(2)").Text()
	zipURL := ""
	doc.Find("table.download a").Each(func(n int, elem *goquery.Selection) {
		href := elem.AttrOr("href", "")
		if strings.HasSuffix(href, ".zip") {
			zipURL = href
		}
	})

	if zipURL == "" {
		return author, ""
	}

	if strings.HasPrefix(zipURL, "http://") || strings.HasPrefix(zipURL, "https://") {
		return author, zipURL
	}

	u, err := url.Parse(siteURL)
	if err != nil {
		return author, ""
	}
	u.Path = path.Join(path.Dir(u.Path), zipURL)
	return author, u.String()
}

func main() {
	// listURL := "https://www.aozora.gr.jp/index_pages/person1257.html"
	// entries, err := findEntities(listURL)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// for _, entry := range entries {
	// 	content, err := extractText(entry.ZipURL)
	// 	if err != nil {
	// 		log.Println(err)
	// 		continue
	// 	}
	// 	fmt.Println(entry.SiteURL)
	// 	fmt.Println(content)
	// }

	db, err := sql.Open("sqlite3", "database.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	queries := []string{
		`DROP TABLE IF EXISTS authors`,
		`DROP TABLE IF EXISTS contents`,
		`DROP TABLE IF EXISTS contents_fts`,
		`CREATE TABLE IF NOT EXISTS authors (author_id TEXT, author TEXT, PRIMARY KEY(author_id))`,
		`CREATE TABLE IF NOT EXISTS contents (author_id TEXT, title_id TEXT, title TEXT, content TEXT, PRIMARY KEY(author_id, title_id))`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS contents_fts USING fts4(words)`,
	}
	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			log.Fatal(err)
		}
	}
	content := "abc kuro aba"

	res, err := db.Exec(`INSERT INTO contents(author_id, title_id, title, content) VALUES (?,?,?,?)`,
		"000879",
		"14",
		"あばばばば",
		content,
	)
	if err != nil {
		log.Fatalf("insert into contents error : %v\n", err)
	}
	docID, err := res.LastInsertId()
	if err != nil {
		log.Fatalf("last insert id error : %v\n", err)
	}

	t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		log.Fatalf("tokenizer error : %v\n", err)
	}
	seg := t.Wakati(content)
	_, err = db.Exec(`
	INSERT INTO contents_fts(docid, words) values(?,?)
	`,
		docID,
		strings.Join(seg, " "),
	)
	if err != nil {
		log.Fatalf("insert into contents_fts error : %v\n", err)
	}

	query := "kuro AND aba"
	rows, err := db.Query(`
	SELECT
		a.author
		, c.title
	FROM
		contents c
	INNER JOIN authors a
		ON a.author_id = c.author_id
	INNER JOIN contents_fts f
		ON c.rowid = f.docid
		AND words MATCH ?
	`, query)

	if err != nil {
		log.Fatalf("select error : %v\n", err)
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var author, title string
		if err := rows.Scan(&author, &title); err != nil {
			log.Fatalf("scan error : %v\n", err)
			log.Fatal(err)
		}
		fmt.Println(author, title)
	}
}

func extractText(zipURL string) (string, error) {
	resp, err := http.Get(zipURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return "", err
	}
	for _, file := range r.File {
		if path.Ext(file.Name) == ".txt" {
			f, err := file.Open()
			if err != nil {
				return "", err
			}
			defer f.Close()
			b, err := ioutil.ReadAll(f)
			if err != nil {
				return "", err
			}
			return string(b), nil
		}
	}
	return "", errors.New("contents not found")
}
