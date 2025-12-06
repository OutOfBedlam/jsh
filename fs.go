package jsh

import (
	"io/fs"
	"sort"
	"strings"

	"github.com/OutOfBedlam/jsh/global"
)

// FS allows mounting multiple fs.FS at different paths
type FS struct {
	mounts map[string]fs.FS
}

var _ fs.FS = (*FS)(nil)
var _ fs.ReadDirFS = (*FS)(nil)

// NewFS creates a new MountFS
func NewFS() *FS {
	return &FS{mounts: make(map[string]fs.FS)}
}

// Mount mounts an fs.FS at a given virtual path
// Returns error if mountPoint is invalid or already exists
func (m *FS) Mount(mountPoint string, filesystem fs.FS) error {
	if filesystem == nil {
		return fs.ErrInvalid
	}

	mountPoint = global.CleanPath(mountPoint)

	// Check for conflicting mounts
	for existing := range m.mounts {
		if mountPoint == existing {
			return fs.ErrExist
		}
		// Check if new mount would shadow existing mount
		if mountPoint != "/" && strings.HasPrefix(existing, mountPoint+"/") {
			return fs.ErrExist
		}
		// Check if existing mount would shadow new mount
		if existing != "/" && strings.HasPrefix(mountPoint, existing+"/") {
			return fs.ErrExist
		}
	}

	m.mounts[mountPoint] = filesystem
	return nil
}

// Unmount removes a mounted filesystem at the given path
func (m *FS) Unmount(mountPoint string) error {
	mountPoint = global.CleanPath(mountPoint)

	if _, ok := m.mounts[mountPoint]; !ok {
		return fs.ErrNotExist
	}

	delete(m.mounts, mountPoint)
	return nil
}

// Mounts returns a list of all mount points
func (m *FS) Mounts() []string {
	mounts := make([]string, 0, len(m.mounts))
	for mountPoint := range m.mounts {
		mounts = append(mounts, mountPoint)
	}
	sort.Strings(mounts)
	return mounts
}

// Open implements fs.FS
func (m *FS) Open(name string) (fs.File, error) {
	name = global.CleanPath(name)
	// Validate path: skip leading / for fs.ValidPath check
	validPath := strings.TrimPrefix(name, "/")
	if validPath == "" {
		validPath = "."
	}
	if !fs.ValidPath(validPath) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	// Find the longest matching mount point
	var bestMatch string
	var bestFS fs.FS

	for mountPoint, filesystem := range m.mounts {
		if mountPoint == "/" {
			// Root mount matches everything
			if bestMatch == "" {
				bestMatch = "/"
				bestFS = filesystem
			}
			continue
		}
		if name == mountPoint || strings.HasPrefix(name, mountPoint+"/") {
			if len(mountPoint) > len(bestMatch) {
				bestMatch = mountPoint
				bestFS = filesystem
			}
		}
	}

	if bestFS == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	relPath := strings.TrimPrefix(name, bestMatch)
	relPath = strings.TrimPrefix(relPath, "/")
	if relPath == "" {
		relPath = "."
	}

	return bestFS.Open(relPath)
}

// ReadDir implements fs.ReadDirFS
func (m *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	name = global.CleanPath(name)
	// Validate path: skip leading / for fs.ValidPath check
	validPath := strings.TrimPrefix(name, "/")
	if validPath == "" {
		validPath = "."
	}
	if !fs.ValidPath(validPath) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}

	// Find the longest matching mount point
	var bestMatch string
	var bestFS fs.FS

	for mountPoint, filesystem := range m.mounts {
		if mountPoint == "/" {
			// Root mount matches everything
			if bestMatch == "" {
				bestMatch = "/"
				bestFS = filesystem
			}
			continue
		}
		if name == mountPoint || strings.HasPrefix(name, mountPoint+"/") {
			if len(mountPoint) > len(bestMatch) {
				bestMatch = mountPoint
				bestFS = filesystem
			}
		}
	}

	if bestFS == nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}

	relPath := strings.TrimPrefix(name, bestMatch)
	relPath = strings.TrimPrefix(relPath, "/")
	if relPath == "" {
		relPath = "."
	}

	// Try to read directory from the matched filesystem
	if readDirFS, ok := bestFS.(fs.ReadDirFS); ok {
		return readDirFS.ReadDir(relPath)
	}

	// Fallback: use fs.ReadDir
	return fs.ReadDir(bestFS, relPath)
}
