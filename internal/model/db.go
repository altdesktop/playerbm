package model

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

func InitDb(path string) (*sql.DB, error) {
	log.Printf("[DEBUG] connecting to database at: %s", path)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	versionRow := db.QueryRow("PRAGMA user_version")
	if err != nil {
		return nil, err
	}

	var version int
	versionRow.Scan(&version)
	log.Printf("[DEBUG] database version: %d", version)

	if version > 1 {
		msg := fmt.Sprintf("Got unknown database version: %d", version)
		return nil, errors.New(msg)
	}

	if version == 0 {
		log.Printf("[DEBUG] initializing database for the first time")
		createStmt := `
        CREATE TABLE bookmarks (
            id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
            url TEXT,
            position INTEGER,
            length INTEGER,
            hash TEXT,
            inode TEXT, -- uint64
            mtime INTEGER,
            finished INTEGER, -- boolean
            created INTEGER,
            updated INTEGER
        );
        PRAGMA user_version = 1;
        `
		_, err = db.Exec(createStmt)
		if err != nil {
			return nil, err
		}
		version = 1
	}

	return db, nil
}
