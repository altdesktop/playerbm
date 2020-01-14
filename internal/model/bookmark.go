package model

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"
	"time"
)

var finishedThreshold = int64(1e+7)

type Bookmark struct {
	Id          int64
	Url         *XesamUrl
	Hash        string
	Position    int64
	Length      int64
	Mtime       int64
	Finished    int
	Inode       string
	Created     int64
	Updated     int64
	needsCreate bool
}

type FileError struct {
	err string
}

func (e *FileError) Error() string {
	return e.err
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

func getFileSchemeBookmark(db *sql.DB, url *XesamUrl) (*Bookmark, error) {
	log.Printf("[DEBUG] getting bookmark from file scheme path")
	// Identified by the hash with filesystem heuristics to avoid reading the
	// whole file when not necessary
	var stat syscall.Stat_t
	err := syscall.Stat(url.UnescapedPath(), &stat)
	if err != nil {
		// TODO: relax the requirement that the file must exist
		return nil, &FileError{err: "File does not exist"}
	}
	if stat.Mode&syscall.S_IFREG == 0 {
		return nil, &FileError{err: "Not a regular file"}
	}

	bm := Bookmark{
		Url:   url,
		Inode: fmt.Sprintf("%d", stat.Ino),
		Mtime: stat.Mtim.Nano(),
	}

	// First try: inode and mtime should approximately identify a file on the
	// file system without reading the file
	stmt, err := db.Prepare(`
    select id, position, hash, length, finished, updated, created
    from bookmarks
    where inode = ? and mtime = ?
    limit 1;
    `)
	if err != nil {
		return nil, err
	}
	row := stmt.QueryRow(bm.Inode, bm.Mtime)
	err = row.Scan(&bm.Id, &bm.Position, &bm.Hash, &bm.Length, &bm.Finished,
		&bm.Updated, &bm.Created)
	if err == nil {
		log.Printf("[DEBUG] got bookmark from inode/mtime")
		return &bm, nil
	} else if err == sql.ErrNoRows {
		// Second try: read the file and try to find it by the hash
		f, err := os.Open(url.UnescapedPath())
		if err != nil {
			return nil, err
		}
		defer f.Close()

		hash, err := sha256sum(f)
		if err != nil {
			return nil, err
		}
		bm.Hash = hash

		stmt, err = db.Prepare(`
        select id, position, length, finished, updated, created
        from bookmarks where hash = ?
        limit 1;
        `)
		if err != nil {
			return nil, err
		}

		row = stmt.QueryRow(hash)
		err = row.Scan(&bm.Id, &bm.Position, &bm.Length, &bm.Finished,
			&bm.Updated, &bm.Created)
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

func getOtherSchemeBookmark(db *sql.DB, url *XesamUrl) (*Bookmark, error) {
	bookmark := Bookmark{Url: url}

	stmt, err := db.Prepare(`
    select id, position, length, finished, updated, created
    from bookmarks
    where url = ?
    limit 1;
    `)
	if err != nil {
		return nil, err
	}
	err = stmt.QueryRow(url.String()).Scan(&bookmark.Id, &bookmark.Position,
		&bookmark.Length, &bookmark.Finished, &bookmark.Updated, &bookmark.Created)

	if err != nil {
		if err == sql.ErrNoRows {
			bookmark.needsCreate = true
			return &bookmark, nil
		}
		return nil, err
	}

	return &bookmark, nil
}

func ListBookmarks(db *sql.DB) ([]Bookmark, error) {
	var bookmarks []Bookmark
	rows, err := db.Query(`
    select id, url, position, hash, inode, mtime, length, finished, updated,
        created
    from bookmarks
    order by updated desc
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		bm := Bookmark{}
		var url string
		err = rows.Scan(&bm.Id, &url, &bm.Position, &bm.Hash, &bm.Inode,
			&bm.Mtime, &bm.Length, &bm.Finished, &bm.Updated, &bm.Created)
		if err != nil {
			return nil, err
		}
		parsedUrl, err := ParseXesamUrl(url)
		if err != nil {
			panic(err)
		}
		bm.Url = parsedUrl
		bookmarks = append(bookmarks, bm)
	}

	return bookmarks, nil
}

func GetBookmark(db *sql.DB, url *XesamUrl) (*Bookmark, error) {
	if url.Scheme() == "file" {
		return getFileSchemeBookmark(db, url)
	} else {
		return getOtherSchemeBookmark(db, url)
	}
}

func GetMostRecentBookmark(db *sql.DB) (*Bookmark, error) {
	stmt, err := db.Prepare(`
    select id, url, position, hash, inode, mtime, length, finished, updated,
        created
    from bookmarks
    where finished == 0
    order by updated desc
    limit 1;
    `)
	if err != nil {
		return nil, err
	}
	var url string
	bookmark := Bookmark{}
	row := stmt.QueryRow()
	err = row.Scan(&bookmark.Id, &url, &bookmark.Position, &bookmark.Hash,
		&bookmark.Inode, &bookmark.Mtime, &bookmark.Length, &bookmark.Finished,
		&bookmark.Updated, &bookmark.Created)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	parsedUrl, err := ParseXesamUrl(url)
	if err != nil {
		panic(err)
	}
	bookmark.Url = parsedUrl

	return &bookmark, nil
}

func createBookmark(bm *Bookmark, db *sql.DB) error {
	now := time.Now().Unix()
	stmt, err := db.Prepare(`
    insert into bookmarks (url, position, hash, inode, mtime, length, finished,
        created, updated)
    values(?, ?, ?, ?, ?, ?, ?, ?, ?);
    `)
	if err != nil {
		return err
	}
	result, err := stmt.Exec(bm.Url.String(), bm.Position, bm.Hash, bm.Inode, bm.Mtime,
		bm.Length, bm.Finished, now, now)
	if err != nil {
		return err
	}
	bmId, err := result.LastInsertId()
	if err != nil {
		return err
	}
	bm.Created = now
	bm.Updated = now
	bm.Id = bmId
	bm.needsCreate = false
	return nil
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func updateBookmark(bm *Bookmark, db *sql.DB) error {
	now := time.Now().Unix()
	stmt, err := db.Prepare(`
    update bookmarks
    set url = ?, position = ?, hash = ?, inode = ?, mtime = ?, length = ?,
        finished = ?, updated = ?
    where id = ?;
    `)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(bm.Url.String(), bm.Position, bm.Hash, bm.Inode, bm.Mtime,
		bm.Length, bm.Finished, now, bm.Id)
	if err != nil {
		return err
	}
	bm.Updated = now
	return nil
}

func (bm *Bookmark) Save(db *sql.DB) error {
	if bm.Length > 0 {
		if abs(bm.Length-bm.Position) < finishedThreshold || bm.Position > bm.Length {
			bm.Finished = 1
			bm.Position = 0
		} else {
			bm.Finished = 0
		}
	}

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
