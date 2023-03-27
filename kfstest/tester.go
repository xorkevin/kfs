package kfstest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"xorkevin.dev/kerrors"
)

type (
	TestFSFile struct {
		Name string
		Data []byte
	}
)

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
