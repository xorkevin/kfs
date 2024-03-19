package kfstest

import (
	"io"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"xorkevin.dev/kfs"
)

func Test_MapFS(t *testing.T) {
	t.Parallel()

	assert := require.New(t)

	now := time.Now()
	var filemode fs.FileMode = 0o644

	testFiles := []TestFSFile{
		{
			Name: "foo.txt",
			Data: []byte("hello, world"),
		},
		{
			Name: "bar/foobar.txt",
			Data: []byte("foo bar"),
		},
	}

	fsys := &MapFS{
		Fsys: fstest.MapFS{},
	}

	{
		var _ kfs.FS = fsys
	}

	for _, i := range testFiles {
		fsys.Fsys[i.Name] = &fstest.MapFile{
			Data:    i.Data,
			Mode:    filemode,
			ModTime: now,
		}
	}

	fileNames := make([]string, 0, len(testFiles))
	for _, i := range testFiles {
		fileNames = append(fileNames, i.Name)
	}
	assert.NoError(fstest.TestFS(fsys, fileNames...))

	assert.NoError(TestFS(fsys, testFiles...))

	assert.NoError(TestFileWrite(fsys, "other/other.txt", []byte("other")))
	subFsys, err := fs.Sub(fsys, "other")
	assert.NoError(err)
	assert.NoError(TestFileWrite(subFsys, "subother/subother.txt", []byte("subother")))
	subsubFsys, err := fs.Sub(subFsys, "subother")
	assert.NoError(TestFileAppend(subsubFsys, "subother.txt", []byte("more")))

	fsys.Fsys["other/link.txt"] = &fstest.MapFile{
		Data:    []byte("subother/subother.txt"),
		Mode:    0o777 | fs.ModeSymlink,
		ModTime: now,
	}

	{
		// test lstat
		info, err := kfs.Lstat(subFsys, "link.txt")
		assert.NoError(err)
		assert.True(info.Mode().Type()&fs.ModeSymlink != 0)
		target, err := kfs.ReadLink(subFsys, "link.txt")
		assert.NoError(err)
		assert.Equal("subother/subother.txt", target)
	}

	{
		// test chtimes
		info, err := fs.Stat(subFsys, "subother/subother.txt")
		assert.NoError(err)
		targetModTime := info.ModTime().Add(time.Second)
		assert.NoError(kfs.Chtimes(subFsys, "subother/subother.txt", time.Time{}, targetModTime))
		info, err = fs.Stat(subFsys, "subother/subother.txt")
		assert.NoError(err)
		assert.True(info.ModTime().Equal(targetModTime))
	}

	{
		// test remove
		assert.NoError(TestFileWrite(subFsys, "subother/another.txt", []byte("another")))
		assert.NoError(TestFileWrite(subFsys, "yetanother.txt", []byte("yetanother")))
		assert.ErrorIs(kfs.Remove(subFsys, "subother/dne.txt"), fs.ErrNotExist)
		assert.NoError(kfs.Remove(subFsys, "subother/subother.txt"))
		assert.NoError(TestFileOpen(subFsys, "subother/another.txt", []byte("another")))
		assert.NoError(kfs.RemoveAll(subFsys, "subother"))
		_, err = fs.Stat(subFsys, "subother/another.txt")
		assert.ErrorIs(err, fs.ErrNotExist)
		_, err := fs.Stat(subFsys, "subother")
		assert.ErrorIs(err, fs.ErrNotExist)
		assert.NoError(TestFileOpen(subFsys, "yetanother.txt", []byte("yetanother")))
	}

	{
		f, err := fsys.OpenFile("foo.txt", os.O_RDONLY, 0o644)
		assert.NoError(err)
		_, ok := f.(io.Seeker)
		assert.True(ok)
		_, ok = f.(io.ReaderAt)
	}
}
