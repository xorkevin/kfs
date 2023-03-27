package kfstest

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
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
	subFsys, err := fs.Sub(fsys, "bar")
	assert.NoError(err)
	assert.NoError(TestFileWrite(subFsys, "subother/subother.txt", []byte("subother")))
	subsubFsys, err := fs.Sub(subFsys, "subother")
	assert.NoError(TestFileAppend(subsubFsys, "subother.txt", []byte("more")))
}
