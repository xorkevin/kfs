package kfstest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"xorkevin.dev/kerrors"
	"xorkevin.dev/kfs"
)

// TestFileOpen tests reading a file using Open
func TestFileOpen(fsys fs.FS, name string, data []byte) (retErr error) {
	f, err := fsys.Open(name)
	if err != nil {
		return kerrors.WithMsg(err, fmt.Sprintf("Failed to open file %s", name))
	}
	defer func() {
		if err := f.Close(); err != nil {
			retErr = errors.Join(retErr, kerrors.WithMsg(err, fmt.Sprintf("Failed closing file %s", name)))
		}
	}()
	content, err := io.ReadAll(f)
	if err != nil {
		return kerrors.WithMsg(err, fmt.Sprintf("Failed to read file %s", name))
	}
	if !bytes.Equal(data, content) {
		return kerrors.WithMsg(nil, fmt.Sprintf("Data for %s does not match", name))
	}
	info, err := f.Stat()
	if err != nil {
		return kerrors.WithMsg(err, fmt.Sprintf("Failed to stat file %s", name))
	}
	if info.Name() != path.Base(name) {
		return kerrors.WithMsg(nil, fmt.Sprintf("Fileinfo name for %s does not match %s", name, info.Name()))
	}
	return nil
}

type (
	// TestFSFile specifies files to test by [TestFS]
	TestFSFile struct {
		Name string
		Data []byte
	}
)

// TestFS tests fs operations:
//
//   - Open
//   - Stat
//   - ReadFile
//   - ReadDir
//   - Glob
//   - Sub
func TestFS(fsys fs.FS, files ...TestFSFile) error {
	filesByDir := map[string][]TestFSFile{}
	readDirRes := map[string][]fs.DirEntry{}
	globbedAncestors := map[string]struct{}{}

	for _, i := range files {
		// check file open
		if err := TestFileOpen(fsys, i.Name, i.Data); err != nil {
			return err
		}

		// check stat
		info, err := fs.Stat(fsys, i.Name)
		if err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to stat %s", i.Name))
		}
		if info.Name() != path.Base(i.Name) {
			return kerrors.WithMsg(nil, fmt.Sprintf("Fileinfo name for %s does not match %s", i.Name, info.Name()))
		}

		// check content of read file
		content, err := fs.ReadFile(fsys, i.Name)
		if err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to readfile %s", i.Name))
		}
		if !bytes.Equal(i.Data, content) {
			return kerrors.WithMsg(nil, fmt.Sprintf("Data for %s does not match", i.Name))
		}

		// get directory, directory child, and rest if there exists one
		dir, rest, hasDir := strings.Cut(i.Name, "/")
		var child string
		if hasDir {
			child, _, _ = strings.Cut(rest, "/")
			// for files with a directory, group them by directory for subdir testing
			filesByDir[dir] = append(filesByDir[dir], TestFSFile{
				Name: rest,
				Data: i.Data,
			})
		} else {
			dir = "."
			child = i.Name
		}

		// get readdir if not already obtained
		entries, isCached := readDirRes[dir]
		if !isCached {
			var err error
			entries, err = fs.ReadDir(fsys, dir)
			if err != nil {
				return kerrors.WithMsg(err, fmt.Sprintf("Failed to readdir %s for %s", dir, i.Name))
			}
			readDirRes[dir] = entries
		}

		// check readdir output for directory child
		hasEntry := false
		for _, j := range entries {
			if j.Name() == child {
				hasEntry = true
				break
			}
		}
		if !hasEntry {
			return kerrors.WithMsg(nil, fmt.Sprintf("Missing dir entry %s in %s for %s", child, dir, i.Name))
		}

		// check glob pattern
		ancestors, base := path.Split(i.Name)
		if _, ok := globbedAncestors[ancestors]; !ok {
			globbedAncestors[ancestors] = struct{}{}
			ext := path.Ext(base)
			if ext != "" {
				pattern := path.Join(ancestors, "*"+ext)
				entries, err := fs.Glob(fsys, pattern)
				if err != nil {
					return kerrors.WithMsg(err, fmt.Sprintf("Failed to glob %s for %s", pattern, i.Name))
				}
				hasEntry := false
				for _, j := range entries {
					if j == i.Name {
						hasEntry = true
						break
					}
				}
				if !hasEntry {
					return kerrors.WithMsg(nil, fmt.Sprintf("Missing glob entry %s in %s", i.Name, pattern))
				}
			}
		}
	}

	// test subdir
	dirs := make([]string, 0, len(filesByDir))
	for i := range filesByDir {
		dirs = append(dirs, i)
	}
	sort.Strings(dirs)
	for _, i := range dirs {
		subfsys, err := fs.Sub(fsys, i)
		if err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed subdir %s", i))
		}
		if err := TestFS(subfsys, filesByDir[i]...); err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed TestFS in subdir %s", i))
		}
	}
	return nil
}

// TestFileWrite tests writing a file with [kfs.OpenFile]
func TestFileWrite(fsys fs.FS, name string, data []byte) error {
	if err := func() (retErr error) {
		f, err := kfs.OpenFile(fsys, name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to open file %s for writing", name))
		}
		defer func() {
			if err := f.Close(); err != nil {
				retErr = errors.Join(retErr, err)
			}
		}()
		info, err := f.Stat()
		if err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to stat file %s", name))
		}
		if info.Name() != path.Base(name) {
			return kerrors.WithMsg(err, fmt.Sprintf("Fileinfo name %s does not match for %s", info.Name(), name))
		}
		if _, err := f.Write(data); err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to write file %s", name))
		}
		return nil
	}(); err != nil {
		return err
	}
	if err := func() (retErr error) {
		f, err := kfs.OpenFile(fsys, name, os.O_RDONLY, 0)
		if err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to open file %s for writing", name))
		}
		defer func() {
			if err := f.Close(); err != nil {
				retErr = errors.Join(retErr, err)
			}
		}()
		info, err := f.Stat()
		if err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to stat file %s", name))
		}
		if info.Name() != path.Base(name) {
			return kerrors.WithMsg(err, fmt.Sprintf("Fileinfo name %s does not match for %s", info.Name(), name))
		}
		if !info.Mode().IsRegular() {
			return kerrors.WithMsg(err, fmt.Sprintf("Fileinfo mode is not a regular file for %s", name))
		}
		if info.IsDir() {
			return kerrors.WithMsg(err, fmt.Sprintf("Fileinfo mode is not a regular file for %s", name))
		}
		if info.Size() != int64(len(data)) {
			return kerrors.WithMsg(err, fmt.Sprintf("Fileinfo size does not match data for %s", name))
		}
		if info.ModTime().IsZero() {
			return kerrors.WithMsg(err, fmt.Sprintf("Fileinfo modtime is unset for %s", name))
		}
		info.Sys() // does not panic
		content, err := io.ReadAll(f)
		if err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to read file %s", name))
		}
		if !bytes.Equal(data, content) {
			return kerrors.WithMsg(err, fmt.Sprintf("File data does not match for %s", name))
		}
		return nil
	}(); err != nil {
		return err
	}
	return nil
}

// TestFileAppend tests appending to a file with [kfs.OpenFile]
func TestFileAppend(fsys fs.FS, name string, data []byte) error {
	orig, err := fs.ReadFile(fsys, name)
	if err != nil {
		return kerrors.WithMsg(err, fmt.Sprintf("Failed to read file %s", name))
	}
	if err := func() (retErr error) {
		f, err := kfs.OpenFile(fsys, name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to open file %s for writing", name))
		}
		defer func() {
			if err := f.Close(); err != nil {
				retErr = errors.Join(retErr, err)
			}
		}()
		if _, err := f.Write(data); err != nil {
			return kerrors.WithMsg(err, fmt.Sprintf("Failed to write file %s", name))
		}
		return nil
	}(); err != nil {
		return err
	}
	expected := append(orig, data...)
	content, err := fs.ReadFile(fsys, name)
	if !bytes.Equal(expected, content) {
		return kerrors.WithMsg(err, fmt.Sprintf("File data does not match for %s", name))
	}
	return nil
}
