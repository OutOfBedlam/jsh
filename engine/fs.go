package engine

import (
	"io"
	"io/fs"
	"sort"
	"strings"
	"time"
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

	mountPoint = CleanPath(mountPoint)

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
	mountPoint = CleanPath(mountPoint)

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
	name = CleanPath(name)
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

func (m *FS) CleanPath(name string) string {
	return CleanPath(name)
}

func (m *FS) Stat(name string) (fs.FileInfo, error) {
	name = CleanPath(name)
	f, err := m.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

func (m *FS) ReadFile(name string) ([]byte, error) {
	name = CleanPath(name)
	f, err := m.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// ReadDir implements fs.ReadDirFS
func (m *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	name = CleanPath(name)
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

	// Read base directory entries
	var entries []fs.DirEntry
	if readDirFS, ok := bestFS.(fs.ReadDirFS); ok {
		var err error
		entries, err = readDirFS.ReadDir(relPath)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		entries, err = fs.ReadDir(bestFS, relPath)
		if err != nil {
			return nil, err
		}
	}

	// Get current directory info for "." entry
	var currentInfo fs.FileInfo
	if f, err := bestFS.Open(relPath); err == nil {
		currentInfo, _ = f.Stat()
		f.Close()
	}

	// Get parent directory info for ".." entry
	var parentInfo fs.FileInfo
	parentRelPath := relPath
	if relPath == "." {
		// Already at the root of this mount, use root info for parent
		if f, err := bestFS.Open("."); err == nil {
			parentInfo, _ = f.Stat()
			f.Close()
		}
	} else {
		// Get parent directory
		lastSlash := strings.LastIndex(relPath, "/")
		if lastSlash > 0 {
			parentRelPath = relPath[:lastSlash]
		} else {
			parentRelPath = "."
		}
		if f, err := bestFS.Open(parentRelPath); err == nil {
			parentInfo, _ = f.Stat()
			f.Close()
		}
	}

	// Add . and .. entries with real directory info
	dotEntries := []fs.DirEntry{
		&dotDirEntry{name: ".", isDir: true, info: currentInfo},
		&dotDirEntry{name: "..", isDir: true, info: parentInfo},
	}
	entries = append(dotEntries, entries...)

	// Add mounted directories as entries
	for mountPoint := range m.mounts {
		// Skip the root mount
		if mountPoint == "/" {
			continue
		}

		// Check if this mount point is a direct child of the current directory
		// For example, if name is "/" and mountPoint is "/bin", add "bin"
		// If name is "/usr" and mountPoint is "/usr/local", add "local"
		if strings.HasPrefix(mountPoint, name+"/") || (name == "/" && mountPoint != "/") {
			relativePath := strings.TrimPrefix(mountPoint, name)
			relativePath = strings.TrimPrefix(relativePath, "/")

			// Only include direct children (not nested)
			if !strings.Contains(relativePath, "/") && relativePath != "" {
				// Check if this entry already exists in the base entries
				exists := false
				for _, entry := range entries {
					if entry.Name() == relativePath {
						exists = true
						break
					}
				}
				if !exists {
					entries = append(entries, &dotDirEntry{name: relativePath, isDir: true})
				}
			}
		}
	}

	return entries, nil
}

// dotDirEntry implements fs.DirEntry for . and .. and mount points
type dotDirEntry struct {
	name  string
	isDir bool
	info  fs.FileInfo // underlying file info, may be nil
}

func (d *dotDirEntry) Name() string {
	return d.name
}

func (d *dotDirEntry) IsDir() bool {
	return d.isDir
}

func (d *dotDirEntry) Type() fs.FileMode {
	if d.isDir {
		return fs.ModeDir
	}
	return 0
}

func (d *dotDirEntry) Info() (fs.FileInfo, error) {
	if d.info != nil {
		return &dotFileInfo{
			name:    d.name,
			isDir:   d.isDir,
			size:    d.info.Size(),
			modTime: d.info.ModTime(),
		}, nil
	}
	return &dotFileInfo{name: d.name, isDir: d.isDir}, nil
}

// dotFileInfo implements fs.FileInfo for . and .. and mount points
type dotFileInfo struct {
	name    string
	isDir   bool
	size    int64
	modTime time.Time
}

func (d *dotFileInfo) Name() string {
	return d.name
}

func (d *dotFileInfo) Size() int64 {
	return d.size
}

func (d *dotFileInfo) Mode() fs.FileMode {
	if d.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}

func (d *dotFileInfo) ModTime() time.Time {
	return d.modTime
}

func (d *dotFileInfo) IsDir() bool {
	return d.isDir
}

func (d *dotFileInfo) Sys() interface{} {
	return nil
}
