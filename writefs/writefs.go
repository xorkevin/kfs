package writefs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"xorkevin.dev/kerrors"
)

var ErrNotImplemented errNotImplemented

type (
	errNotImplemented struct{}
)

func (e errNotImplemented) Error() string {
	return "Not implemented"
}

type (
	// File is an [fs.File] that allows writing
	File interface {
		fs.File
		io.Writer
	}

	// WriteFS is a file system that may be read from and written to
	WriteFS interface {
		fs.FS
		// OpenFile returns an open file
		OpenFile(name string, flag int, mode fs.FileMode) (File, error)
	}

	writeFS struct {
		fsys fs.FS
		dir  string
	}
)

// OpenFile opens a file
//
// If fsys does not implement WriteFS, then OpenFile returns an error.
func OpenFile(fsys fs.FS, name string, flag int, mode fs.FileMode) (File, error) {
	rl, ok := fsys.(WriteFS)
	if !ok {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: kerrors.WithMsg(ErrNotImplemented, "Failed to open file")}
	}
	return rl.OpenFile(name, flag, mode)
}

// WriteFile writes a file
//
// If fsys does not implement WriteFS, then OpenFile returns an error.
func WriteFile(fsys fs.FS, name string, data []byte, perm fs.FileMode) (retErr error) {
	f, err := OpenFile(fsys, name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return kerrors.WithMsg(err, "Failed opening file")
	}
	defer func() {
		if err := f.Close(); err != nil {
			retErr = errors.Join(retErr, kerrors.WithMsg(err, "Failed closing file"))
		}
	}()
	_, err = f.Write(data)
	if err != nil {
		return &fs.PathError{Op: "openfile", Path: name, Err: kerrors.WithMsg(err, "Failed writing to file")}
	}
	return nil
}

func (f *writeFS) Open(name string) (fs.File, error) {
	return f.fsys.Open(name)
}

func (f *writeFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(f.fsys, name)
}

func (f *writeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(f.fsys, name)
}

func (f *writeFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(f.fsys, name)
}

func (f *writeFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(f.fsys, pattern)
}

func (f *writeFS) Sub(dir string) (fs.FS, error) {
	fsys, err := fs.Sub(f.fsys, dir)
	if err != nil {
		return nil, err
	}
	return New(fsys, path.Join(f.dir, dir)), nil
}

// OpenFile implements [WriteFS]
//
// When O_CREATE is set, it will create any directories in the path of the file
// with 0o777 (before umask)
func (f *writeFS) OpenFile(name string, flag int, mode fs.FileMode) (File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrInvalid}
	}
	fullPath := filepath.Join(filepath.FromSlash(f.dir), filepath.FromSlash(name))
	if flag&os.O_CREATE != 0 {
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o777); err != nil {
			return nil, &fs.PathError{Op: "openfile", Path: name, Err: kerrors.WithMsg(err, "Failed to mkdir")}
		}
	}
	fi, err := os.OpenFile(fullPath, flag, mode)
	if err != nil {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: kerrors.WithMsg(err, "Failed to open file")}
	}
	return fi, nil
}

// New creates a new [WriteFS]
func New(fsys fs.FS, dir string) WriteFS {
	return &writeFS{
		fsys: fsys,
		dir:  dir,
	}
}
