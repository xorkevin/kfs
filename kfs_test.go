package kfs_test

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"xorkevin.dev/kfs"
	"xorkevin.dev/kfs/kfstest"
)

func Test_FS(t *testing.T) {
	t.Parallel()

	assert := require.New(t)

	gitFileRegex, err := regexp.Compile(`^(?:.*/)?\.git(?:/.*)?$`)
	assert.NoError(err)

	testGitFileFilter := func(p string) (bool, error) {
		return !gitFileRegex.MatchString(p), nil
	}

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

	{
		hiddenFilePath := filepath.Join(tempDir, filepath.FromSlash(".git/hidden.txt"))
		assert.NoError(os.MkdirAll(filepath.Dir(hiddenFilePath), 0o777))
		assert.NoError(os.WriteFile(hiddenFilePath, []byte("hidden file data"), 0o644))
	}

	fsys := kfs.NewMaskFS(kfs.DirFS(tempDir), testGitFileFilter)

	assert.NoError(kfs.WriteFile(fsys, "foo.txt", []byte("hello, world"), 0o644))
	assert.NoError(kfstest.TestFileWrite(fsys, "bar/foobar.txt", []byte("foo bar")))
	subFsys, err := fs.Sub(fsys, "other")
	assert.NoError(err)
	assert.NoError(kfstest.TestFileWrite(subFsys, "subother/subother.txt", []byte("subother")))

	assert.NoError(kfstest.TestFS(fsys, testFiles...))

	{
		// test read-only fs
		roFS := kfs.NewReadOnlyFS(fsys)
		assert.NoError(kfstest.TestFS(roFS, testFiles...))
		assert.ErrorIs(
			kfs.WriteFile(roFS, "shouldfailwriting", []byte("should fail writing"), 0o644),
			kfs.ErrReadOnly,
		)

		fullpath, err := kfs.FullFilePath(roFS, "foo.txt")
		assert.NoError(err)
		assert.Equal(path.Join(filepath.ToSlash(tempDir), "foo.txt"), fullpath)
	}

	assert.NoError(kfstest.TestFileAppend(subFsys, "subother/subother.txt", []byte("more")))

	assert.NoError(os.Symlink("subother/subother.txt", path.Join(tempDir, "other/link.txt")))

	{
		// test lstat
		roFS := kfs.NewReadOnlyFS(subFsys)
		info, err := kfs.Lstat(roFS, "link.txt")
		assert.NoError(err)
		assert.True(info.Mode().Type()&fs.ModeSymlink != 0)
		target, err := kfs.ReadLink(roFS, "link.txt")
		assert.NoError(err)
		assert.Equal("subother/subother.txt", target)
		content, err := fs.ReadFile(roFS, "link.txt")
		assert.NoError(err)
		assert.Equal([]byte("subothermore"), content)
	}

	{
		// test mask
		entries, err := fs.ReadDir(fsys, ".")
		assert.NoError(err)
		for _, i := range entries {
			assert.NotEqual(".git", i.Name())
		}
		_, err = fs.ReadFile(fsys, ".git")
		assert.ErrorIs(err, kfs.ErrFileMasked)
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
		assert.NoError(kfstest.TestFileWrite(subFsys, "subother/another.txt", []byte("another")))
		assert.NoError(kfstest.TestFileWrite(subFsys, "yetanother.txt", []byte("yetanother")))
		assert.ErrorIs(kfs.Remove(subFsys, "subother/dne.txt"), fs.ErrNotExist)
		assert.NoError(kfs.Remove(subFsys, "subother/subother.txt"))
		assert.NoError(kfstest.TestFileOpen(subFsys, "subother/another.txt", []byte("another")))
		assert.NoError(kfs.RemoveAll(subFsys, "subother"))
		_, err = fs.Stat(subFsys, "subother/another.txt")
		assert.ErrorIs(err, fs.ErrNotExist)
		_, err := fs.Stat(subFsys, "subother")
		assert.ErrorIs(err, fs.ErrNotExist)
		assert.NoError(kfstest.TestFileOpen(subFsys, "yetanother.txt", []byte("yetanother")))
	}
}
