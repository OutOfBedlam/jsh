package jsh

import (
	"io"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestMountFS_NewMountFS(t *testing.T) {
	mfs := NewMountFS()
	if mfs == nil {
		t.Fatal("NewMountFS returned nil")
	}
	if mfs.mounts == nil {
		t.Fatal("mounts map is nil")
	}
	if len(mfs.mounts) != 0 {
		t.Errorf("Expected empty mounts, got %d", len(mfs.mounts))
	}
}

func TestMountFS_Mount(t *testing.T) {
	tests := []struct {
		name       string
		mountPoint string
		wantErr    bool
	}{
		{"simple path", "foo", false},
		{"nested path", "foo/bar", false},
		{"with leading slash", "/foo", false},
		{"with trailing slash", "foo/", false},
		{"root", "/", false},
		{"dot", ".", false},
		{"nil filesystem", "test", true}, // Special case
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := NewMountFS()
			testFS := fstest.MapFS{
				"file.txt": &fstest.MapFile{Data: []byte("content")},
			}

			var err error
			if tt.name == "nil filesystem" {
				err = mfs.Mount(tt.mountPoint, nil)
			} else {
				err = mfs.Mount(tt.mountPoint, testFS)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("Mount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMountFS_Mount_Conflicts(t *testing.T) {
	testFS1 := fstest.MapFS{
		"file1.txt": &fstest.MapFile{Data: []byte("content1")},
	}
	testFS2 := fstest.MapFS{
		"file2.txt": &fstest.MapFile{Data: []byte("content2")},
	}

	tests := []struct {
		name        string
		firstMount  string
		secondMount string
		wantErr     bool
	}{
		{"exact duplicate", "foo", "foo", true},
		{"parent-child", "foo", "foo/bar", true},
		{"child-parent", "foo/bar", "foo", true},
		{"siblings", "foo", "bar", false},
		{"different nested", "foo/bar", "foo/baz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := NewMountFS()

			if err := mfs.Mount(tt.firstMount, testFS1); err != nil {
				t.Fatalf("First mount failed: %v", err)
			}

			err := mfs.Mount(tt.secondMount, testFS2)
			if (err != nil) != tt.wantErr {
				t.Errorf("Second mount error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMountFS_Unmount(t *testing.T) {
	mfs := NewMountFS()
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	// Mount a filesystem
	if err := mfs.Mount("test", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Verify it's mounted
	if len(mfs.Mounts()) != 1 {
		t.Errorf("Expected 1 mount, got %d", len(mfs.Mounts()))
	}

	// Unmount it
	if err := mfs.Unmount("test"); err != nil {
		t.Errorf("Unmount failed: %v", err)
	}

	// Verify it's unmounted
	if len(mfs.Mounts()) != 0 {
		t.Errorf("Expected 0 mounts after unmount, got %d", len(mfs.Mounts()))
	}

	// Try to unmount non-existent mount
	err := mfs.Unmount("nonexistent")
	if err != fs.ErrNotExist {
		t.Errorf("Expected ErrNotExist, got %v", err)
	}
}

func TestMountFS_Mounts(t *testing.T) {
	mfs := NewMountFS()
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	// Empty mounts
	mounts := mfs.Mounts()
	if len(mounts) != 0 {
		t.Errorf("Expected empty mounts, got %v", mounts)
	}

	// Add some mounts
	mountPoints := []string{"foo", "bar", "baz/qux"}
	for _, mp := range mountPoints {
		if err := mfs.Mount(mp, testFS); err != nil {
			t.Fatalf("Mount %s failed: %v", mp, err)
		}
	}

	// Verify all are returned and sorted
	mounts = mfs.Mounts()
	if len(mounts) != len(mountPoints) {
		t.Errorf("Expected %d mounts, got %d", len(mountPoints), len(mounts))
	}

	// Check they are sorted
	expected := []string{"bar", "baz/qux", "foo"}
	for i, exp := range expected {
		if mounts[i] != exp {
			t.Errorf("Mount %d: expected %s, got %s", i, exp, mounts[i])
		}
	}
}

func TestMountFS_Open(t *testing.T) {
	fs1 := fstest.MapFS{
		"file1.txt":     &fstest.MapFile{Data: []byte("content1")},
		"dir/file2.txt": &fstest.MapFile{Data: []byte("content2")},
	}
	fs2 := fstest.MapFS{
		"file3.txt": &fstest.MapFile{Data: []byte("content3")},
	}

	mfs := NewMountFS()
	if err := mfs.Mount("mount1", fs1); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	if err := mfs.Mount("mount2", fs2); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		wantContent string
		wantErr     bool
	}{
		{"file in mount1", "mount1/file1.txt", "content1", false},
		{"nested file in mount1", "mount1/dir/file2.txt", "content2", false},
		{"file in mount2", "mount2/file3.txt", "content3", false},
		{"non-existent mount", "mount3/file.txt", "", true},
		{"non-existent file", "mount1/nonexistent.txt", "", true},
		{"invalid path", "../outside.txt", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := mfs.Open(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Open() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			defer f.Close()

			data, err := io.ReadAll(f)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}

			if string(data) != tt.wantContent {
				t.Errorf("Expected content %q, got %q", tt.wantContent, string(data))
			}
		})
	}
}

func TestMountFS_Open_LongestMatch(t *testing.T) {
	// Test that the longest matching mount point is used
	fs1 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("short")},
	}
	fs2 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("long")},
	}

	mfs := NewMountFS()
	if err := mfs.Mount("foo", fs1); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	if err := mfs.Mount("foo/bar", fs2); err != nil {
		// This should fail due to conflict prevention
		if err != fs.ErrExist {
			t.Fatalf("Expected ErrExist, got %v", err)
		}
		return
	}

	// If we reach here, the mount succeeded (shouldn't happen with new logic)
	f, err := mfs.Open("foo/bar/file.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	// Should get content from the longer mount
	if string(data) != "long" {
		t.Errorf("Expected 'long', got %q", string(data))
	}
}

func TestMountFS_ReadDir(t *testing.T) {
	testFS := fstest.MapFS{
		"dir/file1.txt":        &fstest.MapFile{Data: []byte("content1")},
		"dir/file2.txt":        &fstest.MapFile{Data: []byte("content2")},
		"dir/subdir/file3.txt": &fstest.MapFile{Data: []byte("content3")},
	}

	mfs := NewMountFS()
	if err := mfs.Mount("mount", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		wantCount int
		wantErr   bool
	}{
		{"read dir", "mount/dir", 3, false},
		{"read subdir", "mount/dir/subdir", 1, false},
		{"non-existent dir", "mount/nonexistent", 0, true},
		{"non-existent mount", "nomount/dir", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := mfs.ReadDir(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(entries) != tt.wantCount {
				t.Errorf("Expected %d entries, got %d", tt.wantCount, len(entries))
			}
		})
	}
}

func TestMountFS_ReadDir_NoReadDirFS(t *testing.T) {
	// Create a filesystem that doesn't implement ReadDirFS
	type basicFS struct {
		fstest.MapFS
	}

	testFS := basicFS{
		MapFS: fstest.MapFS{
			"file.txt": &fstest.MapFile{Data: []byte("content")},
		},
	}

	mfs := NewMountFS()
	if err := mfs.Mount("mount", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// ReadDir should still work via fs.ReadDir fallback
	entries, err := mfs.ReadDir("mount")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) == 0 {
		t.Error("Expected entries, got none")
	}
}

func TestCleanPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"foo", "foo"},
		{"/foo", "foo"},
		{"foo/", "foo"},
		{"/foo/", "foo"},
		{"foo/bar", "foo/bar"},
		{"/foo/bar/", "foo/bar"},
		{"foo//bar", "foo/bar"},
		{"foo/./bar", "foo/bar"},
		{"foo/../bar", "bar"},
		{"/", "."},
		{".", "."},
		{"", "."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanPath(tt.input)
			if result != tt.expected {
				t.Errorf("cleanPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMountFS_Interface(t *testing.T) {
	mfs := NewMountFS()

	// Verify it implements fs.FS
	var _ fs.FS = mfs

	// Verify it implements fs.ReadDirFS
	var _ fs.ReadDirFS = mfs
}

func BenchmarkMountFS_Open(b *testing.B) {
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	mfs := NewMountFS()
	if err := mfs.Mount("mount", testFS); err != nil {
		b.Fatalf("Mount failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f, err := mfs.Open("mount/file.txt")
		if err != nil {
			b.Fatal(err)
		}
		f.Close()
	}
}

func BenchmarkMountFS_ReadDir(b *testing.B) {
	testFS := fstest.MapFS{
		"dir/file1.txt": &fstest.MapFile{Data: []byte("content1")},
		"dir/file2.txt": &fstest.MapFile{Data: []byte("content2")},
		"dir/file3.txt": &fstest.MapFile{Data: []byte("content3")},
	}

	mfs := NewMountFS()
	if err := mfs.Mount("mount", testFS); err != nil {
		b.Fatalf("Mount failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mfs.ReadDir("mount/dir")
		if err != nil {
			b.Fatal(err)
		}
	}
}
