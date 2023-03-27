package symlinkfs_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"xorkevin.dev/kfs/kfstest"
	"xorkevin.dev/kfs/symlinkfs"
)

func Test_SymlinkFS(t *testing.T) {
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
	for _, i := range testFiles {
		fullPath := filepath.Join(tempDir, filepath.FromSlash(i.Name))
		assert.NoError(os.MkdirAll(filepath.Dir(fullPath), 0o777))
		assert.NoError(os.WriteFile(fullPath, i.Data, 0o644))
	}

	fsys := symlinkfs.New(os.DirFS(tempDir), tempDir)

	fileNames := make([]string, 0, len(testFiles))
	for _, i := range testFiles {
		fileNames = append(fileNames, i.Name)
	}
	assert.NoError(fstest.TestFS(fsys, fileNames...))

	assert.NoError(kfstest.TestFS(fsys, testFiles...))
}
