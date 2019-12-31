package model

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"syscall"
)

type Bookmark struct {
	Id          int64
	Url         string
	Hash        string
	Position    int64
	Length      int64
	Mtime       int64
	Finished    bool
	Inode       string
	needsCreate bool
}

func (bm *Bookmark) Exists() bool {
	return !bm.needsCreate
}

func sha256sum(file *os.File) (string, error) {
	hash := sha256.New()
	_, err := io.Copy(hash, file)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func getFileSchemeBookmark(db *sql.DB, url *url.URL) (*Bookmark, error) {
	log.Printf("[DEBUG] getting bookmark from file scheme path")
	// Identified by the hash with filesystem heuristics to avoid reading the
	// whole file when not necessary
	var stat syscall.Stat_t
	err := syscall.Stat(url.Path, &stat)
	if err != nil {
		// TODO: relax the requirement that the file must exist
		return nil, err
	}

	bm := Bookmark{
		Url:   url.String(),
		Inode: fmt.Sprintf("%d", stat.Ino),
		Mtime: stat.Mtim.Nano(),
	}

	// First try: inode and mtime should approximately identify a file on the
	// file system without reading the file
	stmt, err := db.Prepare(`
    select id, position, hash
    from bookmarks
    where inode = ? and mtime = ?;
    `)
	if err != nil {
		return nil, err
	}
	row := stmt.QueryRow(bm.Inode, bm.Mtime)
	err = row.Scan(&bm.Id, &bm.Position, &bm.Hash)
	if err == nil {
		log.Printf("[DEBUG] got bookmark from inode/mtime")
		return &bm, nil
	} else if err == sql.ErrNoRows {
		// Second try: read the file and try to find it by the hash
		f, err := os.Open(url.Path)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		hash, err := sha256sum(f)
		if err != nil {
			return nil, err
		}
		bm.Hash = hash

		stmt, err = db.Prepare(`select id, position from bookmarks where hash = ?;`)
		if err != nil {
			return nil, err
		}

		row = stmt.QueryRow(hash)
		err = row.Scan(&bm.Id, &bm.Position)
		if err == sql.ErrNoRows {
			log.Printf("[DEBUG] this is a new bookmark")
			bm.needsCreate = true
		} else if err != nil {
			return nil, err
		} else {
			log.Printf("[DEBUG] got bookmark from hash")
		}
	}

	return &bm, nil
}

func ListBookmarks(db *sql.DB) ([]Bookmark, error) {
	var bookmarks []Bookmark
	rows, err := db.Query(`select id, url, position, hash, inode, mtime from bookmarks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		bm := Bookmark{}
		err = rows.Scan(&bm.Id, &bm.Url, &bm.Position, &bm.Hash, &bm.Inode, &bm.Mtime)
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
	stmt, err := db.Prepare(`insert into bookmarks (url, position, hash, inode, mtime) values(?, ?, ?, ?, ?);`)
	if err != nil {
		return err
	}
	result, err := stmt.Exec(bm.Url, bm.Position, bm.Hash, bm.Inode, bm.Mtime)
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
    set url = ?, position = ?, hash = ?, inode = ?, mtime = ?
    where id = ?;
    `)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(bm.Url, bm.Position, bm.Hash, bm.Inode, bm.Mtime, bm.Id)
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
