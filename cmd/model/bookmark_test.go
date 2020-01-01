package model

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
)

func createTmpFile(t *testing.T) *os.File {
	f, err := ioutil.TempFile("", "pbm-track")
	require.NoError(t, err)
	_, err = f.WriteString(uuid.New().String())
	require.NoError(t, err)
	return f
}

func TestBookmarkSave(t *testing.T) {
	f := createTmpFile(t)
	defer os.Remove(f.Name())

	url := "file://" + f.Name()

	db, err := InitDb(":memory:")
	defer db.Close()
	require.NoError(t, err)

	bm, err := GetBookmark(db, url)
	require.NoError(t, err)
	require.NotNil(t, bm)
	t.Log(bm)
	require.NotNil(t, bm.Hash)
	require.Equal(t, len(bm.Hash), 64, "The bookmark hash should be a valid sha256 hex digest")
	require.False(t, bm.Exists(), "The bookmark should not already exist")
	err = bm.Save(db)
	require.NoError(t, err)
	require.NotEqual(t, bm.Id, 0, "After save, the bookmark should have a database id")
	require.True(t, bm.Exists(), "After save, the bookmark should be marked as exists")
	require.Greater(t, bm.Created, int64(0), "The bookmark should have a create timestamp")
	require.Greater(t, bm.Updated, int64(0), "The bookmark should have an updated timestamp")

	// Update the position and length, save the bookmark, and make sure it propagates to
	// the database
	require.Equal(t, bm.Position, int64(0))
	bm.Position = int64(1000)
	bm.Length = int64(1001)
	bm.Finished = 1
	bm.Save(db)
	bm, err = GetBookmark(db, url)
	require.NoError(t, err)
	require.Equal(t, bm.Position, int64(1000))
	require.Equal(t, bm.Length, int64(1001))
	require.Equal(t, bm.Finished, 1)

	// Make sure listing the bookmarks works
	bookmarks, err := ListBookmarks(db)
	require.NoError(t, err)
	require.Equal(t, len(bookmarks), 1)
	require.Equal(t, &bookmarks[0], bm)

	// Delete the bookmark
	err = bm.Delete(db)
	require.NoError(t, err)
	bookmarks, err = ListBookmarks(db)
	require.NoError(t, err)
	require.Equal(t, len(bookmarks), 0)
}
