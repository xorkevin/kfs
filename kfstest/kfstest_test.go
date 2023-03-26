package kfstest

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"xorkevin.dev/kfs/writefs"
)

func Test_MapFS(t *testing.T) {
	t.Parallel()

	assert := require.New(t)

	now := time.Now()
	var filemode fs.FileMode = 0o644

	fsys := &MapFS{
		Fsys: fstest.MapFS{
			"foo.txt": &fstest.MapFile{
				Data:    []byte(`hello, world`),
				Mode:    filemode,
				ModTime: now,
			},
			"bar/foobar.txt": &fstest.MapFile{
				Data:    []byte(`foobar`),
				Mode:    filemode,
				ModTime: now,
			},
		},
	}
	assert.NoError(fstest.TestFS(fsys, "foo.txt", "bar/foobar.txt"))

	{
		info, err := fs.Stat(fsys, "foo.txt")
		assert.NoError(err)
		assert.NotNil(info)
		assert.Equal("foo.txt", info.Name())
	}

	{
		info, err := fs.Stat(fsys, "bar/foobar.txt")
		assert.NoError(err)
		assert.NotNil(info)
		assert.Equal("foobar.txt", info.Name())
	}

	{
		entries, err := fs.ReadDir(fsys, ".")
		assert.NoError(err)
		assert.Len(entries, 2)
		assert.Equal("bar", entries[0].Name())
		assert.Equal("foo.txt", entries[1].Name())
	}

	{
		entries, err := fs.ReadDir(fsys, "bar")
		assert.NoError(err)
		assert.Len(entries, 1)
		assert.Equal("foobar.txt", entries[0].Name())
	}

	{
		b, err := fs.ReadFile(fsys, "foo.txt")
		assert.NoError(err)
		assert.Equal([]byte("hello, world"), b)
	}

	{
		b, err := fs.ReadFile(fsys, "bar/foobar.txt")
		assert.NoError(err)
		assert.Equal([]byte("foobar"), b)
	}

	{
		paths, err := fs.Glob(fsys, "*.txt")
		assert.NoError(err)
		assert.Equal([]string{"foo.txt"}, paths)
	}

	{
		assert.NoError(writefs.WriteFile(fsys, "other/other.txt", []byte("other"), 0x644))
		b, err := fs.ReadFile(fsys, "other/other.txt")
		assert.NoError(err)
		assert.Equal([]byte("other"), b)
	}

	subFsys, err := fs.Sub(fsys, "bar")
	assert.NoError(err)

	{
		info, err := fs.Stat(subFsys, "foobar.txt")
		assert.NoError(err)
		assert.NotNil(info)
		assert.Equal("foobar.txt", info.Name())
	}

	{
		entries, err := fs.ReadDir(subFsys, ".")
		assert.NoError(err)
		assert.Len(entries, 1)
		assert.Equal("foobar.txt", entries[0].Name())
	}

	{
		b, err := fs.ReadFile(subFsys, "foobar.txt")
		assert.NoError(err)
		assert.Equal([]byte("foobar"), b)
	}

	{
		paths, err := fs.Glob(subFsys, "*.txt")
		assert.NoError(err)
		assert.Equal([]string{"foobar.txt"}, paths)
	}

	{
		assert.NoError(writefs.WriteFile(subFsys, "subother/subother.txt", []byte("subother"), 0x644))
		subsubFsys, err := fs.Sub(subFsys, "subother")
		assert.NoError(err)
		b, err := fs.ReadFile(subsubFsys, "subother.txt")
		assert.NoError(err)
		assert.Equal([]byte("subother"), b)
	}
}
