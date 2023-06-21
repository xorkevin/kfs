package kfs

import (
	"io/fs"
	"path"
	"time"

	"xorkevin.dev/kerrors"
)

// ErrFileMasked is returned when the file is masked
var ErrFileMasked errFileMasked

type (
	errFileMasked struct{}
)

func (e errFileMasked) Error() string {
	return "File is masked"
}

type (
	// FileFilter filters files by file path and dir entry
	FileFilter = func(p string) (bool, error)

	maskFS struct {
		fsys   fs.FS
		dir    string
		filter FileFilter
	}
)

func (f *maskFS) checkFile(op string, name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{
			Op:   op,
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}
	ok, err := f.filter(path.Join(f.dir, name))
	if err != nil {
		return &fs.PathError{
			Op:   op,
			Path: name,
			Err:  kerrors.WithMsg(err, "Failed filtering file"),
		}
	}
	if !ok {
		return &fs.PathError{
			Op:   op,
			Path: name,
			Err:  kerrors.WithKind(fs.ErrPermission, ErrFileMasked, "File does not exist"),
		}
	}
	return nil
}

func (f *maskFS) Open(name string) (fs.File, error) {
	if err := f.checkFile("open", name); err != nil {
		return nil, err
	}
	return f.fsys.Open(name)
}

func (f *maskFS) Stat(name string) (fs.FileInfo, error) {
	if err := f.checkFile("stat", name); err != nil {
		return nil, err
	}
	return fs.Stat(f.fsys, name)
}

func (f *maskFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := f.checkFile("readdir", name); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(f.fsys, name)
	if err != nil {
		return nil, err
	}
	basePath := path.Join(f.dir, name)
	res := make([]fs.DirEntry, 0, len(entries))
	for _, i := range entries {
		if ok, err := f.filter(path.Join(basePath, i.Name())); err != nil {
			return nil, &fs.PathError{
				Op:   "readdir",
				Path: name,
				Err:  kerrors.WithMsg(err, "Failed filtering dir entry"),
			}
		} else if !ok {
			continue
		}
		res = append(res, i)
	}
	return res, nil
}

func (f *maskFS) ReadFile(name string) ([]byte, error) {
	if err := f.checkFile("readfile", name); err != nil {
		return nil, err
	}
	return fs.ReadFile(f.fsys, name)
}

func (f *maskFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(f.fsys, pattern)
}

func (f *maskFS) Sub(dir string) (fs.FS, error) {
	if err := f.checkFile("sub", dir); err != nil {
		return nil, err
	}
	fsys, err := fs.Sub(f.fsys, dir)
	if err != nil {
		return nil, err
	}
	return &maskFS{
		fsys:   fsys,
		dir:    path.Join(f.dir, dir),
		filter: f.filter,
	}, nil
}

func (f *maskFS) Lstat(name string) (fs.FileInfo, error) {
	if err := f.checkFile("lstat", name); err != nil {
		return nil, err
	}
	return Lstat(f.fsys, name)
}

func (f *maskFS) ReadLink(name string) (string, error) {
	if err := f.checkFile("readlink", name); err != nil {
		return "", err
	}
	return ReadLink(f.fsys, name)
}

func (f *maskFS) OpenFile(name string, flag int, mode fs.FileMode) (File, error) {
	if err := f.checkFile("openfile", name); err != nil {
		return nil, err
	}
	return OpenFile(f.fsys, name, flag, mode)
}

func (f *maskFS) Remove(name string) error {
	if err := f.checkFile("remove", name); err != nil {
		return err
	}
	return Remove(f.fsys, name)
}

func (f *maskFS) RemoveAll(name string) error {
	if err := f.checkFile("removeall", name); err != nil {
		return err
	}
	return RemoveAll(f.fsys, name)
}

func (f *maskFS) Chtimes(name string, atime, mtime time.Time) error {
	if err := f.checkFile("chtimes", name); err != nil {
		return err
	}
	return Chtimes(f.fsys, name, atime, mtime)
}

// NewMaskFS creates a new [FS] that masks an fs based on a filter
func NewMaskFS(fsys fs.FS, filter FileFilter) FS {
	return &maskFS{
		fsys:   fsys,
		dir:    "",
		filter: filter,
	}
}
