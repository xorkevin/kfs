package kfstest

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"testing/fstest"
	"time"

	"xorkevin.dev/kfs/writefs"
)

type (
	// MapFS is an in memory [writefs.WriteFS]
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

func (m *MapFS) OpenFile(name string, flag int, mode fs.FileMode) (writefs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrInvalid}
	}

	isRead, isWrite := isReadWrite(flag)
	if !isRead && !isWrite {
		// must read or write
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrInvalid}
	}
	if isRead && isWrite {
		// do not support both reading and writing for simplicity
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrInvalid}
	}

	if flag&os.O_CREATE == 0 {
		if !isWrite {
			// disallow create when not writing
			return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrInvalid}
		}
		if flag&os.O_EXCL != 0 {
			// disallow using excl when create not specified
			return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrInvalid}
		}
	}

	f := m.Fsys[name]
	if f == nil {
		if flag&os.O_CREATE == 0 {
			return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrNotExist}
		}

		f = &fstest.MapFile{
			Data:    nil,
			Mode:    mode,
			ModTime: time.Now(),
		}
	} else {
		if flag&os.O_EXCL != 0 {
			return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrExist}
		}
	}

	if flag&os.O_TRUNC != 0 {
		if !isWrite {
			// disallow using trunc when not writing
			return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrInvalid}
		}
		f.Data = nil
	}
	end := false
	if flag&os.O_APPEND != 0 {
		if !isWrite {
			// disallow using append when not writing
			return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrInvalid}
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

func (f *subdirFS) OpenFile(name string, flag int, mode fs.FileMode) (writefs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrInvalid}
	}
	return f.m.OpenFile(path.Join(f.dir, name), flag, mode)
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

func (f *mapFile) Read(p []byte) (int, error) {
	if f.r == nil {
		return 0, &fs.PathError{Op: "read", Path: f.path, Err: fs.ErrInvalid}
	}
	return f.r.Read(p)
}

func (f *mapFile) Write(p []byte) (int, error) {
	if f.b == nil {
		return 0, &fs.PathError{Op: "write", Path: f.path, Err: fs.ErrInvalid}
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

func (i *mapFileInfo) Type() fs.FileMode {
	return i.f.Mode.Type()
}

func (i *mapFileInfo) ModTime() time.Time {
	return i.f.ModTime
}

func (i *mapFileInfo) IsDir() bool {
	return i.f.Mode&fs.ModeDir != 0
}

func (i *mapFileInfo) Sys() any {
	return i.f.Sys
}

func (i *mapFileInfo) Info() (fs.FileInfo, error) {
	return i, nil
}
