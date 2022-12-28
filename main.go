package main

import (
	"database/sql"
	"io"
	"log"
	"net/http"
	"os"

	"regexp"

	"github.com/mattn/go-sqlite3"
	"goji.io"
	"goji.io/pat"
)

func main() {
	log.Print("Application startup")
	mux := goji.NewMux()
	regexFn := func(re, s string) (bool, error) {
		b, e := regexp.MatchString(re, s)
		return b, e
	}

	sql.Register("sqlite3_regexp",
		&sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				return conn.RegisterFunc("regexp", regexFn, true)
			},
		})

	db, err := sql.Open("sqlite3_regexp", "/opt/app/data/timeseries.sqlite")

	if err != nil {
		log.Fatalf("there was an error opening the sqlite database: %s", err)
	}

	createTableIfNotExist(db)
	rwWriter := NewWriter(db)
	go rwWriter.Start()

	mux.HandleFunc(pat.Get("/healthcheck"), func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		writer.Write([]byte("hello!"))
		log.Print("received healthcheck!")
	})

	mux.HandleFunc(pat.Post("/api/v1/remote_write"), remoteWriterHandler(rwWriter))
	mux.HandleFunc(pat.Post("/api/v1/remote_read"), remoteReadHandler(db))

	log.Print("starting server")
	if err := http.ListenAndServe("0.0.0.0:8080", mux); err != nil {
		log.Fatalf("there was an error starting server: %s", err)
	}
}

func createTableIfNotExist(db *sql.DB) {
	_, check := db.Query("select * from samples limit 1")
	if check == nil {
		return
	}

	f, err := os.Open("/opt/app/ddl.sql")
	if err != nil {
		log.Fatalf("could not create schema (bad file open): %s", err)
	}

	sql, err := io.ReadAll(f)
	if err != nil {
		log.Fatalf("could not create schema (bad readall): %s", err)
	}

	_, err = db.Exec(string(sql))
	if err != nil {
		log.Fatalf("could not create DDL (exec): %s", err)
	}
}
