package kfs_test

import (
	"io/fs"
	"os"
	"path"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"xorkevin.dev/kfs"
	"xorkevin.dev/kfs/kfstest"
)

func Test_FS(t *testing.T) {
	t.Parallel()

	assert := require.New(t)

	tempDir := t.TempDir()

	testFiles := []kfstest.TestFSFile{
		{
			Name: "foo.txt",
			Data: []byte("hello, world"),
		},
		{
			Name: "bar/foobar.txt",
			Data: []byte("foo bar"),
		},
		{
			Name: "other/subother/subother.txt",
			Data: []byte("subother"),
		},
	}

	fsys := kfs.New(os.DirFS(tempDir), tempDir)

	assert.NoError(kfs.WriteFile(fsys, "foo.txt", []byte("hello, world"), 0o644))
	assert.NoError(kfstest.TestFileWrite(fsys, "bar/foobar.txt", []byte("foo bar")))
	subFsys, err := fs.Sub(fsys, "other")
	assert.NoError(err)
	assert.NoError(kfstest.TestFileWrite(subFsys, "subother/subother.txt", []byte("subother")))

	fileNames := make([]string, 0, len(testFiles))
	for _, i := range testFiles {
		fileNames = append(fileNames, i.Name)
	}
	assert.NoError(fstest.TestFS(fsys, fileNames...))

	assert.NoError(kfstest.TestFS(fsys, testFiles...))

	assert.NoError(kfstest.TestFileAppend(subFsys, "subother/subother.txt", []byte("more")))

	assert.NoError(os.Symlink("subother/subother.txt", path.Join(tempDir, "other/link.txt")))

	{
		info, err := kfs.Lstat(subFsys, "link.txt")
		assert.NoError(err)
		assert.True(info.Mode().Type()&fs.ModeSymlink != 0)
		target, err := kfs.ReadLink(subFsys, "link.txt")
		assert.NoError(err)
		assert.Equal("subother/subother.txt", target)
		content, err := fs.ReadFile(subFsys, "link.txt")
		assert.NoError(err)
		assert.Equal([]byte("subothermore"), content)
	}
}
