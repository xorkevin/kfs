package kfs

import (
	"errors"
	"io/fs"
	"path"

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
	FileFilter = func(p string, entry fs.DirEntry) (bool, error)

	maskFS struct {
		fsys   fs.FS
		dir    string
		filter FileFilter
	}
)

func (f *maskFS) checkFileStat(op string, name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   op,
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}
	info, err := fs.Stat(f.fsys, name)
	if err != nil {
		return nil, err
	}
	ok, err := f.filter(path.Join(f.dir, name), fs.FileInfoToDirEntry(info))
	if err != nil {
		return nil, &fs.PathError{
			Op:   op,
			Path: name,
			Err:  kerrors.WithMsg(err, "Failed filtering file"),
		}
	}
	if !ok {
		return nil, &fs.PathError{
			Op:   op,
			Path: name,
			Err:  kerrors.WithKind(fs.ErrPermission, ErrFileMasked, "File does not exist"),
		}
	}
	return info, nil
}

func (f *maskFS) checkFileLstat(op string, name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   op,
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}
	info, err := Lstat(f.fsys, name)
	if err != nil {
		return nil, err
	}
	ok, err := f.filter(path.Join(f.dir, name), fs.FileInfoToDirEntry(info))
	if err != nil {
		return nil, &fs.PathError{
			Op:   op,
			Path: name,
			Err:  kerrors.WithMsg(err, "Failed filtering file"),
		}
	}
	if !ok {
		return nil, &fs.PathError{
			Op:   op,
			Path: name,
			Err:  kerrors.WithKind(fs.ErrPermission, ErrFileMasked, "File does not exist"),
		}
	}
	return info, nil
}

func (f *maskFS) Open(name string) (fs.File, error) {
	if _, err := f.checkFileStat("open", name); err != nil {
		return nil, err
	}
	return f.fsys.Open(name)
}

func (f *maskFS) Stat(name string) (fs.FileInfo, error) {
	return f.checkFileStat("stat", name)
}

func (f *maskFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if _, err := f.checkFileStat("readdir", name); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(f.fsys, name)
	if err != nil {
		return nil, err
	}
	res := make([]fs.DirEntry, 0, len(entries))
	for _, i := range entries {
		entryPath := path.Join(f.dir, name, i.Name())
		if ok, err := f.filter(entryPath, i); err != nil {
			return nil, &fs.PathError{
				Op:   "readdir",
				Path: entryPath,
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
	if _, err := f.checkFileStat("readfile", name); err != nil {
		return nil, err
	}
	return fs.ReadFile(f.fsys, name)
}

func (f *maskFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(f.fsys, pattern)
}

func (f *maskFS) Sub(dir string) (fs.FS, error) {
	if _, err := f.checkFileStat("sub", dir); err != nil {
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
	return f.checkFileLstat("lstat", name)
}

func (f *maskFS) ReadLink(name string) (string, error) {
	if _, err := f.checkFileLstat("readlink", name); err != nil {
		return "", err
	}
	return ReadLink(f.fsys, name)
}

func (f *maskFS) OpenFile(name string, flag int, mode fs.FileMode) (File, error) {
	if _, err := f.checkFileStat("openfile", name); err != nil {
		if errors.Is(err, ErrFileMasked) || !errors.Is(err, fs.ErrNotExist) {
			// do not open files if masked and return if error is something other
			// than not-exist
			return nil, err
		}
	}
	return OpenFile(f.fsys, name, flag, mode)
}

// NewMaskFS creates a new [FS] that masks an fs based on a filter
func NewMaskFS(fsys fs.FS, filter FileFilter) FS {
	return &maskFS{
		fsys:   fsys,
		dir:    "",
		filter: filter,
	}
}
