package filesystem

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"govdupes/internal/config"
)

type FileIdentity struct {
	NumHardLinks   uint64
	SymbolicLink   string
	IsSymbolicLink bool
	IsHardLink     bool
	Inode          uint64
	Device         uint64
}

type FileTracker struct {
	seen map[FileIdentity]struct{}
}

func NewFileTracker() *FileTracker {
	return &FileTracker{
		seen: make(map[FileIdentity]struct{}),
	}
}

func (ft *FileTracker) CheckHardLink(path string, info fs.FileInfo) (*FileIdentity, error) {
	// syscall get the file's info
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("failed to get raw file stats, path: %q", path)
	}

	fileID := FileIdentity{NumHardLinks: stat.Nlink, Inode: stat.Ino, Device: stat.Dev}

	// If we've seen the deviceID & Inode before, then it's a hard link.
	if _, exists := ft.seen[fileID]; exists {
		fileID.IsHardLink = true
		return &fileID, nil
	}

	ft.seen[fileID] = struct{}{}
	fileID.IsHardLink = false
	return &fileID, nil
}

func IsSymbolicLink(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		log.Printf("Symbolic link detected: %q", path)
		return true, nil
	}
	return false, nil
}

// Checks if a file is a hard link or a symbolic link.
func (ft *FileTracker) FindFileLinks(path string, c config.Config) (*FileIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	fileID, err := ft.CheckHardLink(path, info)
	if err != nil {
		log.Println(err)
		return fileID, err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		log.Printf("Symbolic link detected: %q", path)
		fileID.IsSymbolicLink = true

		if c.FollowSymbolicLinks {
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return fileID, fmt.Errorf("failed to resolve symlink: %w", err)
			}
			fileID.SymbolicLink = realPath
		}
	}

	return fileID, nil
}
