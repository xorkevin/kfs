package kfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"xorkevin.dev/kerrors"
)

var (
	// ErrNotImplemented is returned when the file system does not implement an operation
	ErrNotImplemented errNotImplemented
	// ErrTargetOutsideFS is returned when the symlink target is outside the file system
	ErrTargetOutsideFS errTargetOutsideFS
)

type (
	errNotImplemented  struct{}
	errTargetOutsideFS struct{}
)

func (e errNotImplemented) Error() string {
	return "Not implemented"
}

func (e errTargetOutsideFS) Error() string {
	return "Target outside fs"
}

type (
	// LstatFS is a file system that can run lstat
	LstatFS interface {
		fs.FS
		// Lstat returns the FileInfo of the named file without following symbolic
		// links
		Lstat(name string) (fs.FileInfo, error)
	}
)

// Lstat returns the FileInfo of the named file without following symbolic
// links
//
// If fsys does not implement LstatFS, then Lstat returns an error.
func Lstat(fsys fs.FS, name string) (fs.FileInfo, error) {
	f, ok := fsys.(LstatFS)
	if !ok {
		return nil, &fs.PathError{
			Op:   "lstat",
			Path: name,
			Err:  kerrors.WithMsg(ErrNotImplemented, "Failed to lstat file"),
		}
	}
	return f.Lstat(name)
}

type (
	// ReadLinkFS is a file system that can read links
	ReadLinkFS interface {
		fs.FS
		// ReadLink returns the destination of the named symbolic link. Link
		// destinations will always be slash-separated paths relative to the link's
		// directory. The link destination is guaranteed to be a path inside FS.
		ReadLink(name string) (string, error)
	}
)

// ReadLink returns the destination of the named symbolic link.
//
// If fsys does not implement ReadLinkFS, then ReadLink returns an error.
func ReadLink(fsys fs.FS, name string) (string, error) {
	f, ok := fsys.(ReadLinkFS)
	if !ok {
		return "", &fs.PathError{
			Op:   "readlink",
			Path: name,
			Err:  kerrors.WithMsg(ErrNotImplemented, "Failed to read link"),
		}
	}
	return f.ReadLink(name)
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
)

// OpenFile opens a file
//
// If fsys does not implement WriteFS, then OpenFile returns an error.
func OpenFile(fsys fs.FS, name string, flag int, mode fs.FileMode) (File, error) {
	f, ok := fsys.(WriteFS)
	if !ok {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: kerrors.WithMsg(ErrNotImplemented, "Failed to open file")}
	}
	return f.OpenFile(name, flag, mode)
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

type (
	// RemoveFS is a file system that may remove files
	RemoveFS interface {
		fs.FS
		// Remove removes a file
		Remove(name string) error
	}

	// RemoveAllFS is a file system that may remove files and their children
	RemoveAllFS interface {
		fs.FS
		// RemoveAll removes a file and all children
		RemoveAll(name string) error
	}
)

// Remove removes a file
func Remove(fsys fs.FS, name string) error {
	f, ok := fsys.(RemoveFS)
	if !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: kerrors.WithMsg(ErrNotImplemented, "Failed to remove file")}
	}
	return f.Remove(name)
}

// RemoveAll removes a file and all children
func RemoveAll(fsys fs.FS, name string) error {
	f, ok := fsys.(RemoveAllFS)
	if !ok {
		return &fs.PathError{Op: "removeall", Path: name, Err: kerrors.WithMsg(ErrNotImplemented, "Failed to remove file")}
	}
	return f.RemoveAll(name)
}

type (
	osFS struct {
		fsys fs.FS
		dir  string
	}
)

func (f *osFS) Open(name string) (fs.File, error) {
	return f.fsys.Open(name)
}

func (f *osFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(f.fsys, name)
}

func (f *osFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(f.fsys, name)
}

func (f *osFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(f.fsys, name)
}

func (f *osFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(f.fsys, pattern)
}

func (f *osFS) Sub(dir string) (fs.FS, error) {
	fsys, err := fs.Sub(f.fsys, dir)
	if err != nil {
		return nil, err
	}
	return New(fsys, path.Join(f.dir, dir)), nil
}

func (f *osFS) fullFilePath(name string) string {
	return filepath.Join(filepath.FromSlash(f.dir), filepath.FromSlash(name))
}

func (f *osFS) Lstat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "lstat",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}
	info, err := os.Lstat(f.fullFilePath(name))
	if err != nil {
		return nil, &fs.PathError{
			Op:   "lstat",
			Path: name,
			Err:  kerrors.WithMsg(err, "Failed to lstat file"),
		}
	}
	return info, nil
}

func (f *osFS) ReadLink(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{
			Op:   "readlink",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}
	target, err := os.Readlink(f.fullFilePath(name))
	if err != nil {
		return "", &fs.PathError{
			Op:   "readlink",
			Path: name,
			Err:  kerrors.WithMsg(err, "Failed to read link"),
		}
	}
	target = filepath.ToSlash(target)
	if path.IsAbs(target) {
		return "", &fs.PathError{
			Op:   "readlink",
			Path: name,
			Err:  kerrors.WithMsg(ErrTargetOutsideFS, fmt.Sprintf("Target %s is absolute", target)),
		}
	}
	if !fs.ValidPath(path.Join(path.Dir(name), target)) {
		return "", &fs.PathError{
			Op:   "readlink",
			Path: name,
			Err:  kerrors.WithMsg(ErrTargetOutsideFS, fmt.Sprintf("Target %s is outside the FS", target)),
		}
	}
	return target, nil
}

// OpenFile implements [WriteFS]
//
// When O_CREATE is set, it will create any directories in the path of the file
// with 0o777 (before umask)
func (f *osFS) OpenFile(name string, flag int, mode fs.FileMode) (File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: kerrors.WithMsg(fs.ErrInvalid, "Invalid path")}
	}
	fullPath := f.fullFilePath(name)
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

// Remove implements [RemoveFS]
func (f *osFS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: kerrors.WithMsg(fs.ErrInvalid, "Invalid path")}
	}
	if err := os.Remove(f.fullFilePath(name)); err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: kerrors.WithMsg(err, "Failed to remove file")}
	}
	return nil
}

// RemoveAll implements [RemoveFS]
func (f *osFS) RemoveAll(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "removeall", Path: name, Err: kerrors.WithMsg(fs.ErrInvalid, "Invalid path")}
	}
	if err := os.RemoveAll(f.fullFilePath(name)); err != nil {
		return &fs.PathError{Op: "removeall", Path: name, Err: kerrors.WithMsg(err, "Failed to remove file")}
	}
	return nil
}

type (
	// FS implements all the file system operations
	FS interface {
		fs.FS
		fs.StatFS
		fs.ReadDirFS
		fs.ReadFileFS
		fs.GlobFS
		fs.SubFS
		LstatFS
		ReadLinkFS
		WriteFS
		RemoveFS
		RemoveAllFS
	}
)

// New creates a new [FS]
func New(fsys fs.FS, dir string) FS {
	return &osFS{
		fsys: fsys,
		dir:  dir,
	}
}

// DirFS returns an [os.DirFS] wrapped by [FS]
func DirFS(dir string) FS {
	return New(os.DirFS(filepath.FromSlash(dir)), dir)
}
