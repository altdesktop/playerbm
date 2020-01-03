package model

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
	"time"
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

	db, err := InitDb(":memory:")
	defer db.Close()
	require.NoError(t, err)

	url, err := ParseXesamUrl("file://" + f.Name())
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
	bm.Length = int64(1e+10)
	bm.Finished = 1
	bm.Save(db)
	url, err = ParseXesamUrl("file://" + f.Name())
	bm, err = GetBookmark(db, url)
	require.NoError(t, err)
	require.Equal(t, bm.Position, int64(1000),
		"The position should be saved correctly for the bookmark")
	require.Equal(t, bm.Length, int64(1e+10),
		"The length should be saved correctly for the bookmark")
	require.Equal(t, bm.Finished, 1,
		"The finished flag should be saved correctly for the bookmark")
	// Reset the finish flag for the next tests
	bm.Finished = 0
	require.NoError(t, bm.Save(db))

	// Make sure listing the bookmarks works
	bookmarks, err := ListBookmarks(db)
	require.NoError(t, err)
	require.Equal(t, len(bookmarks), 1)
	require.Equal(t, &bookmarks[0], bm)

	// A more recent bookmark
	time.Sleep(1 * time.Second)
	f = createTmpFile(t)
	defer f.Close()
	url, err = ParseXesamUrl("file://" + f.Name())
	require.NoError(t, err)
	recentBm, err := GetBookmark(db, url)
	require.NoError(t, err)
	err = recentBm.Save(db)
	require.NoError(t, err)
	recentUrl, err := GetMostRecentUrl(db)
	require.NoError(t, err)
	require.NotNil(t, recentUrl)
	require.Equal(t, url, recentUrl,
		"GetMostRecentUrl() should return the most recently saved bookmark url")
	// If it's finished, it shouldn't show up
	recentBm.Finished = 1
	err = recentBm.Save(db)
	require.NoError(t, err)
	recentUrl, err = GetMostRecentUrl(db)
	require.NoError(t, err)
	require.NotNil(t, recentUrl)
	require.Equal(t, bm.Url, recentUrl)

	// Delete the bookmark
	err = bm.Delete(db)
	require.NoError(t, err)
	bookmarks, err = ListBookmarks(db)
	require.NoError(t, err)
	require.Equal(t, len(bookmarks), 1)
}
