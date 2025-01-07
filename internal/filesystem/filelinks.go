package filesystem

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"

	"govdupes/ui"
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
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("failed to get raw file stats, path: %q", path)
	}

	fileID := FileIdentity{NumHardLinks: stat.Nlink, Inode: stat.Ino, Device: stat.Dev}

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
		slog.Info("Symbolic link detected", slog.String("path", path))
		return true, nil
	}
	return false, nil
}

// Checks if a file is a hard link or a symbolic link.
func (ft *FileTracker) FindFileLinks(path string, c ui.Config) (*FileIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	fileID, err := ft.CheckHardLink(path, info)
	if err != nil {
		slog.Error("Error checking hard link", slog.Any("error", err))
		return fileID, err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		slog.Info("Symbolic link detected", slog.String("path", path))
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
