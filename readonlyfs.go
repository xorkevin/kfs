package kfs

import (
	"io/fs"
	"time"

	"xorkevin.dev/kerrors"
)

// ErrReadOnly is returned when the file is read only
var ErrReadOnly errReadOnly

type (
	errReadOnly struct{}
)

func (e errReadOnly) Error() string {
	return "FS is read-only"
}

type (
	readOnlyFS struct {
		fsys fs.FS
	}
)

func (f *readOnlyFS) Open(name string) (fs.File, error) {
	return f.fsys.Open(name)
}

func (f *readOnlyFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(f.fsys, name)
}

func (f *readOnlyFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(f.fsys, name)
}

func (f *readOnlyFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(f.fsys, name)
}

func (f *readOnlyFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(f.fsys, pattern)
}

func (f *readOnlyFS) Sub(dir string) (fs.FS, error) {
	fsys, err := fs.Sub(f.fsys, dir)
	if err != nil {
		return nil, err
	}
	return NewReadOnlyFS(fsys), nil
}

func (f *readOnlyFS) FullFilePath(name string) (string, error) {
	return FullFilePath(f.fsys, name)
}

func (f *readOnlyFS) Lstat(name string) (fs.FileInfo, error) {
	return Lstat(f.fsys, name)
}

func (f *readOnlyFS) ReadLink(name string) (string, error) {
	return ReadLink(f.fsys, name)
}

func (f *readOnlyFS) OpenFile(name string, flag int, mode fs.FileMode) (File, error) {
	return nil, &fs.PathError{
		Op:   "openfile",
		Path: name,
		Err:  kerrors.WithKind(fs.ErrInvalid, ErrReadOnly, "Read-only fs does not support writing"),
	}
}

func (f *readOnlyFS) Remove(name string) error {
	return &fs.PathError{
		Op:   "remove",
		Path: name,
		Err:  kerrors.WithKind(fs.ErrInvalid, ErrReadOnly, "Read-only fs does not support writing"),
	}
}

func (f *readOnlyFS) RemoveAll(name string) error {
	return &fs.PathError{
		Op:   "removeall",
		Path: name,
		Err:  kerrors.WithKind(fs.ErrInvalid, ErrReadOnly, "Read-only fs does not support writing"),
	}
}

func (f *readOnlyFS) Chtimes(name string, atime, mtime time.Time) error {
	return &fs.PathError{
		Op:   "atime",
		Path: name,
		Err:  kerrors.WithKind(fs.ErrInvalid, ErrReadOnly, "Read-only fs does not support writing"),
	}
}

// NewReadOnlyFS creates a new [FS] that is read-only
func NewReadOnlyFS(fsys fs.FS) FS {
	return &readOnlyFS{
		fsys: fsys,
	}
}
