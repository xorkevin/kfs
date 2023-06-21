package kfstest

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing/fstest"
	"time"

	"xorkevin.dev/kerrors"
	"xorkevin.dev/kfs"
)

type (
	// MapFS is an in-memory [kfs.FS]
	MapFS struct {
		Fsys fstest.MapFS
	}
)

const (
	rwFlagMask = os.O_RDONLY | os.O_WRONLY | os.O_RDWR
)

func isReadWrite(flag int) (bool, bool) {
	switch flag & rwFlagMask {
	case os.O_RDONLY:
		return true, false
	case os.O_WRONLY:
		return false, true
	case os.O_RDWR:
		return true, true
	default:
		return false, false
	}
}

func (m *MapFS) Open(name string) (fs.File, error) {
	return m.Fsys.Open(name)
}

func (m *MapFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(m.Fsys, name)
}

func (m *MapFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(m.Fsys, name)
}

func (m *MapFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(m.Fsys, name)
}

func (m *MapFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(m.Fsys, pattern)
}

func (m *MapFS) Sub(dir string) (fs.FS, error) {
	fsys, err := fs.Sub(m.Fsys, dir)
	if err != nil {
		return nil, err
	}
	return &subdirFS{
		m:    m,
		fsys: fsys,
		dir:  dir,
	}, nil
}

func (m *MapFS) OpenFile(name string, flag int, mode fs.FileMode) (kfs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "openfile",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}

	isRead, isWrite := isReadWrite(flag)
	if !isRead && !isWrite {
		return nil, &fs.PathError{
			Op:   "openfile",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Must read or write"),
		}
	}
	if isRead && isWrite {
		// do not support both reading and writing for simplicity
		return nil, &fs.PathError{
			Op:   "openfile",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Unimplemented"),
		}
	}

	if flag&os.O_CREATE != 0 {
		if !isWrite {
			return nil, &fs.PathError{
				Op:   "openfile",
				Path: name,
				Err:  kerrors.WithMsg(fs.ErrInvalid, "May not create when not writing"),
			}
		}
	} else {
		if flag&os.O_EXCL != 0 {
			return nil, &fs.PathError{
				Op:   "openfile",
				Path: name,
				Err:  kerrors.WithMsg(fs.ErrInvalid, "May only use excl when creating"),
			}
		}
	}

	f := m.Fsys[name]
	if f == nil {
		if flag&os.O_CREATE == 0 {
			return nil, &fs.PathError{
				Op:   "openfile",
				Path: name,
				Err:  kerrors.WithMsg(fs.ErrNotExist, "File does not exist"),
			}
		}

		f = &fstest.MapFile{
			Data:    nil,
			Mode:    mode,
			ModTime: time.Now(),
		}
	} else {
		if flag&os.O_EXCL != 0 {
			return nil, &fs.PathError{
				Op:   "openfile",
				Path: name,
				Err:  kerrors.WithMsg(fs.ErrExist, "File already exists"),
			}
		}
	}

	if flag&os.O_TRUNC != 0 {
		if !isWrite {
			return nil, &fs.PathError{
				Op:   "openfile",
				Path: name,
				Err:  kerrors.WithMsg(fs.ErrInvalid, "May not truncate when not writing"),
			}
		}
		f.Data = nil
	}
	end := false
	if flag&os.O_APPEND != 0 {
		if !isWrite {
			return nil, &fs.PathError{
				Op:   "openfile",
				Path: name,
				Err:  kerrors.WithMsg(fs.ErrInvalid, "May not append when not writing"),
			}
		}
		end = true
	}

	var r *bytes.Reader
	if isRead {
		r = bytes.NewReader(f.Data)
	}
	var b *bytes.Buffer
	if isWrite {
		b = &bytes.Buffer{}
		if end {
			b.Write(f.Data)
		}
	}

	return &mapFile{
		info: mapFileInfo{
			name: path.Base(name),
			f:    f,
		},
		path: name,
		r:    r,
		b:    b,
		fsys: m,
	}, nil
}

func (m *MapFS) Lstat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "lstat",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}

	// fs.MapFS does not follow symlinks
	return fs.Stat(m.Fsys, name)
}

func (m *MapFS) ReadLink(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{
			Op:   "readlink",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}

	if f, ok := m.Fsys[name]; ok {
		if f.Mode.Type()&fs.ModeSymlink != 0 {
			target := string(f.Data)
			if path.IsAbs(target) {
				return "", &fs.PathError{
					Op:   "readlink",
					Path: name,
					Err:  kerrors.WithMsg(kfs.ErrTargetOutsideFS, fmt.Sprintf("Target %s is absolute", target)),
				}
			}
			if !fs.ValidPath(path.Join(path.Dir(name), target)) {
				return "", &fs.PathError{
					Op:   "readlink",
					Path: name,
					Err:  kerrors.WithMsg(kfs.ErrTargetOutsideFS, fmt.Sprintf("Target %s is outside the FS", target)),
				}
			}
			return target, nil
		}
	}

	return "", &fs.PathError{
		Op:   "readlink",
		Path: name,
		Err:  kerrors.WithMsg(fs.ErrInvalid, "File is not a link"),
	}
}

func (m *MapFS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{
			Op:   "remove",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}

	if _, ok := m.Fsys[name]; !ok {
		return &fs.PathError{
			Op:   "remove",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrNotExist, "File does not exist"),
		}
	}
	delete(m.Fsys, name)
	return nil
}

func (m *MapFS) RemoveAll(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{
			Op:   "removeall",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}

	var names []string
	for k := range m.Fsys {
		if k == name || strings.HasPrefix(k, name+"/") {
			names = append(names, k)
		}
	}
	for _, i := range names {
		delete(m.Fsys, i)
	}
	return nil
}

func (m *MapFS) Chtimes(name string, atime, mtime time.Time) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{
			Op:   "chtimes",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}

	f := m.Fsys[name]
	if f == nil {
		return &fs.PathError{
			Op:   "chtimes",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrNotExist, "File does not exist"),
		}
	}
	if mtime != (time.Time{}) {
		f.ModTime = mtime
	}
	return nil
}

type (
	subdirFS struct {
		m    *MapFS
		fsys fs.FS
		dir  string
	}
)

func (f *subdirFS) Open(name string) (fs.File, error) {
	return f.fsys.Open(name)
}

func (f *subdirFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(f.fsys, name)
}

func (f *subdirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(f.fsys, name)
}

func (f *subdirFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(f.fsys, name)
}

func (f *subdirFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(f.fsys, pattern)
}

func (f *subdirFS) Sub(dir string) (fs.FS, error) {
	fsys, err := fs.Sub(f.fsys, dir)
	if err != nil {
		return nil, err
	}
	return &subdirFS{
		m:    f.m,
		fsys: fsys,
		dir:  path.Join(f.dir, dir),
	}, nil
}

func (f *subdirFS) OpenFile(name string, flag int, mode fs.FileMode) (kfs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "openfile",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}
	return f.m.OpenFile(path.Join(f.dir, name), flag, mode)
}

func (f *subdirFS) Lstat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "lstat",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}
	return f.m.Lstat(path.Join(f.dir, name))
}

func (f *subdirFS) ReadLink(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{
			Op:   "readlink",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}
	return f.m.ReadLink(path.Join(f.dir, name))
}

func (f *subdirFS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{
			Op:   "remove",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}
	return f.m.Remove(path.Join(f.dir, name))
}

func (f *subdirFS) RemoveAll(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{
			Op:   "remove",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}
	return f.m.RemoveAll(path.Join(f.dir, name))
}

func (f *subdirFS) Chtimes(name string, atime, mtime time.Time) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{
			Op:   "chtimes",
			Path: name,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "Invalid path"),
		}
	}
	return f.m.Chtimes(path.Join(f.dir, name), atime, mtime)
}

type (
	mapFile struct {
		info mapFileInfo
		path string
		r    *bytes.Reader
		b    *bytes.Buffer
		fsys *MapFS
	}

	mapFileInfo struct {
		name string
		f    *fstest.MapFile
	}
)

func (f *mapFile) Stat() (fs.FileInfo, error) {
	return &f.info, nil
}

func (f *mapFile) assertReader() error {
	if f.r == nil {
		return &fs.PathError{
			Op:   "read",
			Path: f.path,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "File not open for reading"),
		}
	}
	return nil
}

func (f *mapFile) Read(p []byte) (int, error) {
	if err := f.assertReader(); err != nil {
		return 0, nil
	}
	return f.r.Read(p)
}

func (f *mapFile) Seek(offset int64, whence int) (int64, error) {
	if err := f.assertReader(); err != nil {
		return 0, nil
	}
	return f.r.Seek(offset, whence)
}

func (f *mapFile) ReadAt(b []byte, offset int64) (int, error) {
	if err := f.assertReader(); err != nil {
		return 0, nil
	}
	return f.r.ReadAt(b, offset)
}

func (f *mapFile) Write(p []byte) (int, error) {
	if f.b == nil {
		return 0, &fs.PathError{
			Op:   "write",
			Path: f.path,
			Err:  kerrors.WithMsg(fs.ErrInvalid, "File not open for writing"),
		}
	}
	return f.b.Write(p)
}

func (f *mapFile) Close() error {
	if f.b != nil {
		f.fsys.Fsys[f.path] = &fstest.MapFile{
			Data:    f.b.Bytes(),
			Mode:    f.info.f.Mode,
			ModTime: time.Now(),
		}
		f.b = nil
	}
	return nil
}

func (i *mapFileInfo) Name() string {
	return i.name
}

func (i *mapFileInfo) Size() int64 {
	return int64(len(i.f.Data))
}

func (i *mapFileInfo) Mode() fs.FileMode {
	return i.f.Mode
}

func (i *mapFileInfo) ModTime() time.Time {
	return i.f.ModTime
}

func (i *mapFileInfo) IsDir() bool {
	return i.f.Mode.IsDir()
}

func (i *mapFileInfo) Sys() any {
	return i.f.Sys
}
