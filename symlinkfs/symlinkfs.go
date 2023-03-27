package symlinkfs

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"xorkevin.dev/kerrors"
)

var (
	// ErrNotImplemented is returned when the file system does not implement [LstatFS] or [ReadLinkFS]
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
	rl, ok := fsys.(LstatFS)
	if !ok {
		return nil, &fs.PathError{
			Op:   "lstat",
			Path: name,
			Err:  kerrors.WithMsg(ErrNotImplemented, "Failed to lstat file"),
		}
	}
	return rl.Lstat(name)
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
	rl, ok := fsys.(ReadLinkFS)
	if !ok {
		return "", &fs.PathError{
			Op:   "readlink",
			Path: name,
			Err:  kerrors.WithMsg(ErrNotImplemented, "Failed to read link"),
		}
	}
	return rl.ReadLink(name)
}

type (
	symlinkFS struct {
		fsys fs.FS
		dir  string
	}
)

func (f *symlinkFS) Open(name string) (fs.File, error) {
	return f.fsys.Open(name)
}

func (f *symlinkFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(f.fsys, name)
}

func (f *symlinkFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(f.fsys, name)
}

func (f *symlinkFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(f.fsys, name)
}

func (f *symlinkFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(f.fsys, pattern)
}

func (f *symlinkFS) Sub(dir string) (fs.FS, error) {
	fsys, err := fs.Sub(f.fsys, dir)
	if err != nil {
		return nil, err
	}
	return New(fsys, path.Join(f.dir, dir)), nil
}

func (f *symlinkFS) fullFilePath(name string) string {
	return filepath.Join(filepath.FromSlash(f.dir), filepath.FromSlash(name))
}

func (f *symlinkFS) Lstat(name string) (fs.FileInfo, error) {
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

func (f *symlinkFS) ReadLink(name string) (string, error) {
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

type (
	// SymlinkFS is an [LstatFS] and [ReadLinkFS]
	SymlinkFS interface {
		LstatFS
		ReadLinkFS
	}
)

// New creates a new [SymlinkFS]
func New(fsys fs.FS, dir string) SymlinkFS {
	return &symlinkFS{
		fsys: fsys,
		dir:  dir,
	}
}
