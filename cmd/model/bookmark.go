package model

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/url"

	"io/ioutil"
	"os"
)

type Bookmark struct {
	Id          int64
	Url         string
	Hash        string
	Position    int64
	needsCreate bool
}

func (bm *Bookmark) Exists() bool {
	return !bm.needsCreate
}

func getFileSchemeBookmark(db *sql.DB, url *url.URL) (*Bookmark, error) {
	// Identified by the hash
	f, err := os.Open(url.Path)
	if err != nil {
		// TODO: relax the requirement that the file must exist
		return nil, err
	}
	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(contents)
	hexDigest := fmt.Sprintf("%x", sum)

	stmt, err := db.Prepare(`select id, position from bookmarks where hash = ?;`)
	if err != nil {
		return nil, err
	}
	bm := Bookmark{Url: url.String(), Hash: hexDigest}

	row := stmt.QueryRow(hexDigest)
	err = row.Scan(&bm.Id, &bm.Position)
	if err == sql.ErrNoRows {
		bm.needsCreate = true
	} else if err != nil {
		return nil, err
	}

	return &bm, nil
}

func ListBookmarks(db *sql.DB) ([]Bookmark, error) {
	var bookmarks []Bookmark
	rows, err := db.Query(`select id, url, hash, position from bookmarks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		bm := Bookmark{}
		err = rows.Scan(&bm.Id, &bm.Url, &bm.Hash, &bm.Position)
		if err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, bm)
	}

	return bookmarks, nil
}

func GetBookmark(db *sql.DB, xesamUrl string) (*Bookmark, error) {
	url, err := url.Parse(xesamUrl)
	if err != nil {
		return nil, err
	}
	if url.Scheme == "file" {
		return getFileSchemeBookmark(db, url)
	} else {
		// TODO: Identified simply by the url
		msg := fmt.Sprintf("Unsupported scheme: %s", url.Scheme)
		return nil, errors.New(msg)
	}
}

func createBookmark(bm *Bookmark, db *sql.DB) error {
	stmt, err := db.Prepare(`insert into bookmarks (url, hash, position) values(?, ?, ?);`)
	if err != nil {
		return err
	}
	result, err := stmt.Exec(bm.Url, bm.Hash, bm.Position)
	if err != nil {
		return err
	}
	bmId, err := result.LastInsertId()
	if err != nil {
		return err
	}
	bm.Id = bmId
	bm.needsCreate = false
	return nil
}

func updateBookmark(bm *Bookmark, db *sql.DB) error {
	stmt, err := db.Prepare(`
    update bookmarks
    set url = ?, hash = ?, position = ?
    where id = ?;
    `)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(bm.Url, bm.Hash, bm.Position, bm.Id)
	return err
}

func (bm *Bookmark) Save(db *sql.DB) error {
	if bm.needsCreate {
		return createBookmark(bm, db)
	} else {
		return updateBookmark(bm, db)
	}
}

func (bm *Bookmark) Delete(db *sql.DB) error {
	if bm.needsCreate {
		// nothing to do
		return nil
	}
	stmt, err := db.Prepare(`delete from bookmarks where id = ?;`)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(bm.Id)
	bm.Id = 0
	bm.needsCreate = true
	return nil
}
