package engine

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestFS_NewFS(t *testing.T) {
	mfs := NewFS()
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

func TestFS_Mount(t *testing.T) {
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
			mfs := NewFS()
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

func TestFS_Mount_Conflicts(t *testing.T) {
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
		{"exact duplicate", "/foo", "/foo", true},
		{"parent-child", "/foo", "/foo/bar", true},
		{"child-parent", "/foo/bar", "/foo", true},
		{"siblings", "/foo", "/bar", false},
		{"different nested", "/foo/bar", "/foo/baz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := NewFS()

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

func TestFS_Unmount(t *testing.T) {
	mfs := NewFS()
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	// Mount a filesystem
	if err := mfs.Mount("/test", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Verify it's mounted
	if len(mfs.Mounts()) != 1 {
		t.Errorf("Expected 1 mount, got %d", len(mfs.Mounts()))
	}

	// Unmount it
	if err := mfs.Unmount("/test"); err != nil {
		t.Errorf("Unmount failed: %v", err)
	}

	// Verify it's unmounted
	if len(mfs.Mounts()) != 0 {
		t.Errorf("Expected 0 mounts after unmount, got %d", len(mfs.Mounts()))
	}

	// Try to unmount non-existent mount
	err := mfs.Unmount("/nonexistent")
	if err != fs.ErrNotExist {
		t.Errorf("Expected ErrNotExist, got %v", err)
	}
}

func TestFS_Mounts(t *testing.T) {
	mfs := NewFS()
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	// Empty mounts
	mounts := mfs.Mounts()
	if len(mounts) != 0 {
		t.Errorf("Expected empty mounts, got %v", mounts)
	}

	// Add some mounts
	mountPoints := []string{"/foo", "/bar", "/baz/qux"}
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
	expected := []string{"/bar", "/baz/qux", "/foo"}
	for i, exp := range expected {
		if mounts[i] != exp {
			t.Errorf("Mount %d: expected %s, got %s", i, exp, mounts[i])
		}
	}
}

func TestFS_Open(t *testing.T) {
	fs0 := fstest.MapFS{
		"rootfile.txt": &fstest.MapFile{Data: []byte("rootcontent")},
	}
	fs1 := fstest.MapFS{
		"file1.txt":     &fstest.MapFile{Data: []byte("content1")},
		"dir/file2.txt": &fstest.MapFile{Data: []byte("content2")},
	}
	fs2 := fstest.MapFS{
		"file3.txt": &fstest.MapFile{Data: []byte("content3")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/", fs0); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	if err := mfs.Mount("/mount1", fs1); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	if err := mfs.Mount("/mount2", fs2); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		wantContent string
		wantErr     bool
	}{
		{"root file", "/rootfile.txt", "rootcontent", false},
		{"file in mount1", "/mount1/file1.txt", "content1", false},
		{"nested file in mount1", "/mount1/dir/file2.txt", "content2", false},
		{"file in mount2", "/mount2/file3.txt", "content3", false},
		{"non-existent mount", "/mount3/file.txt", "", true},
		{"non-existent file", "/mount1/nonexistent.txt", "", true},
		{"invalid path", "../outside.txt", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := mfs.Open(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Open(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
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

func TestFS_Open_LongestMatch(t *testing.T) {
	// Test that the longest matching mount point is used
	fs1 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("short")},
	}
	fs2 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("long")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/foo", fs1); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	if err := mfs.Mount("/foo/bar", fs2); err != nil {
		// This should fail due to conflict prevention
		if err != fs.ErrExist {
			t.Fatalf("Expected ErrExist, got %v", err)
		}
		return
	}

	// If we reach here, the mount succeeded (shouldn't happen with new logic)
	f, err := mfs.Open("/foo/bar/file.txt")
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

func TestFS_ReadDir(t *testing.T) {
	testFS := fstest.MapFS{
		"dir/file1.txt":        &fstest.MapFile{Data: []byte("content1")},
		"dir/file2.txt":        &fstest.MapFile{Data: []byte("content2")},
		"dir/subdir/file3.txt": &fstest.MapFile{Data: []byte("content3")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/mount", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		wantCount int // Now includes . and ..
		wantErr   bool
	}{
		{"read dir", "/mount/dir", 5, false},           // file1.txt, file2.txt, subdir, ., ..
		{"read subdir", "/mount/dir/subdir", 3, false}, // file3.txt, ., ..
		{"non-existent dir", "/mount/nonexistent", 0, true},
		{"non-existent mount", "/nomount/dir", 0, true},
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

func TestFS_ReadDir_NoReadDirFS(t *testing.T) {
	// Create a filesystem that doesn't implement ReadDirFS
	type basicFS struct {
		fstest.MapFS
	}

	testFS := basicFS{
		MapFS: fstest.MapFS{
			"file.txt": &fstest.MapFile{Data: []byte("content")},
		},
	}

	mfs := NewFS()
	if err := mfs.Mount("/mount", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// ReadDir should still work via fs.ReadDir fallback
	entries, err := mfs.ReadDir("/mount")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) == 0 {
		t.Error("Expected entries, got none")
	}
}

func TestFS_Interface(t *testing.T) {
	mfs := NewFS()

	// Verify it implements fs.FS
	var _ fs.FS = mfs

	// Verify it implements fs.ReadDirFS
	var _ fs.ReadDirFS = mfs
}

func TestFS_ReadDir_DotEntries(t *testing.T) {
	testFS := fstest.MapFS{
		"file1.txt": &fstest.MapFile{Data: []byte("content1")},
		"file2.txt": &fstest.MapFile{Data: []byte("content2")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/test", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	entries, err := mfs.ReadDir("/test")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	// Should have: file1.txt, file2.txt, ., ..
	if len(entries) < 4 {
		t.Errorf("Expected at least 4 entries (including . and ..), got %d", len(entries))
	}

	// Check for . and .. entries
	hasDot := false
	hasDotDot := false
	for _, entry := range entries {
		if entry.Name() == "." {
			hasDot = true
			if !entry.IsDir() {
				t.Error(". entry should be a directory")
			}
		}
		if entry.Name() == ".." {
			hasDotDot = true
			if !entry.IsDir() {
				t.Error(".. entry should be a directory")
			}
		}
	}

	if !hasDot {
		t.Error("Missing . entry in directory listing")
	}
	if !hasDotDot {
		t.Error("Missing .. entry in directory listing")
	}
}

func TestFS_ReadDir_MountPoints(t *testing.T) {
	rootFS := fstest.MapFS{
		"rootfile.txt": &fstest.MapFile{Data: []byte("root")},
	}
	binFS := fstest.MapFS{
		"ls": &fstest.MapFile{Data: []byte("binary")},
	}
	sbinFS := fstest.MapFS{
		"init": &fstest.MapFile{Data: []byte("init")},
	}
	usrFS := fstest.MapFS{
		"readme.txt": &fstest.MapFile{Data: []byte("usr")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/", rootFS); err != nil {
		t.Fatalf("Mount / failed: %v", err)
	}
	if err := mfs.Mount("/bin", binFS); err != nil {
		t.Fatalf("Mount /bin failed: %v", err)
	}
	if err := mfs.Mount("/sbin", sbinFS); err != nil {
		t.Fatalf("Mount /sbin failed: %v", err)
	}
	if err := mfs.Mount("/usr", usrFS); err != nil {
		t.Fatalf("Mount /usr failed: %v", err)
	}

	// Read root directory
	entries, err := mfs.ReadDir("/")
	if err != nil {
		t.Fatalf("ReadDir(/) failed: %v", err)
	}

	// Should have: rootfile.txt, ., .., bin, sbin, usr
	expectedNames := map[string]bool{
		"rootfile.txt": false,
		".":            false,
		"..":           false,
		"bin":          false,
		"sbin":         false,
		"usr":          false,
	}

	for _, entry := range entries {
		if _, exists := expectedNames[entry.Name()]; exists {
			expectedNames[entry.Name()] = true
			// Mounted directories should appear as directories
			if entry.Name() == "bin" || entry.Name() == "sbin" || entry.Name() == "usr" {
				if !entry.IsDir() {
					t.Errorf("%s should be a directory", entry.Name())
				}
			}
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("Expected to find %s in root directory listing", name)
		}
	}
}

func TestFS_ReadDir_NestedMountPoints(t *testing.T) {
	rootFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("root")},
	}
	usrFS := fstest.MapFS{
		"readme.txt": &fstest.MapFile{Data: []byte("usr")},
	}
	usrLocalFS := fstest.MapFS{
		"local.txt": &fstest.MapFile{Data: []byte("local")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/", rootFS); err != nil {
		t.Fatalf("Mount / failed: %v", err)
	}
	if err := mfs.Mount("/usr", usrFS); err != nil {
		t.Fatalf("Mount /usr failed: %v", err)
	}

	// This should fail due to conflict (parent-child relationship)
	err := mfs.Mount("/usr/local", usrLocalFS)
	if err == nil {
		t.Fatal("Expected mount to fail due to parent-child conflict")
	}
}

func TestFS_ReadDir_NoDuplicateMountPoints(t *testing.T) {
	rootFS := fstest.MapFS{
		"bin/file.txt": &fstest.MapFile{Data: []byte("original")},
	}
	binFS := fstest.MapFS{
		"ls": &fstest.MapFile{Data: []byte("binary")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/", rootFS); err != nil {
		t.Fatalf("Mount / failed: %v", err)
	}

	// This should fail because /bin would conflict with existing bin in rootFS
	// Actually, this will succeed but let's test that we don't get duplicate "bin" entries
	err := mfs.Mount("/bin", binFS)
	if err != nil {
		// If it fails, that's fine - conflict detection
		t.Logf("Mount /bin failed as expected: %v", err)
		return
	}

	// Read root directory
	entries, err := mfs.ReadDir("/")
	if err != nil {
		t.Fatalf("ReadDir(/) failed: %v", err)
	}

	// Count "bin" entries - should only have one
	binCount := 0
	for _, entry := range entries {
		if entry.Name() == "bin" {
			binCount++
		}
	}

	if binCount != 1 {
		t.Errorf("Expected exactly 1 'bin' entry, got %d", binCount)
	}
}

func TestFS_ReadDir_DotEntriesInfo(t *testing.T) {
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/test", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	entries, err := mfs.ReadDir("/test")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() == "." || entry.Name() == ".." {
			// Test Info() method
			info, err := entry.Info()
			if err != nil {
				t.Errorf("Info() failed for %s: %v", entry.Name(), err)
				continue
			}

			if info.Name() != entry.Name() {
				t.Errorf("Info().Name() = %s, want %s", info.Name(), entry.Name())
			}

			if !info.IsDir() {
				t.Errorf("%s should be a directory", entry.Name())
			}

			if info.Mode()&fs.ModeDir == 0 {
				t.Errorf("%s Mode() should have ModeDir bit set", entry.Name())
			}

			// Test Type() method
			if entry.Type()&fs.ModeDir == 0 {
				t.Errorf("%s Type() should have ModeDir bit set", entry.Name())
			}
		}
	}
}

func TestFS_ReadDir_DotEntriesRealInfo(t *testing.T) {
	// Create a filesystem with known ModTime
	modTime := time.Date(2025, 12, 18, 10, 30, 0, 0, time.UTC)
	testFS := fstest.MapFS{
		"dir/file.txt": &fstest.MapFile{
			Data:    []byte("content"),
			ModTime: modTime,
		},
	}

	mfs := NewFS()
	if err := mfs.Mount("/test", testFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	entries, err := mfs.ReadDir("/test/dir")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	foundDot := false
	foundDotDot := false

	for _, entry := range entries {
		if entry.Name() == "." {
			foundDot = true
			info, err := entry.Info()
			if err != nil {
				t.Fatalf("Info() failed for '.': %v", err)
			}

			// Check that info is accessible
			_ = info.Size()
			_ = info.ModTime()

			// fstest.MapFS may not provide directory metadata,
			// but the code should handle this gracefully
			t.Logf("'.' entry: Size=%d, ModTime=%v", info.Size(), info.ModTime())
		}

		if entry.Name() == ".." {
			foundDotDot = true
			info, err := entry.Info()
			if err != nil {
				t.Fatalf("Info() failed for '..': %v", err)
			}

			// Check that info is accessible
			_ = info.Size()
			_ = info.ModTime()

			t.Logf("'..' entry: Size=%d, ModTime=%v", info.Size(), info.ModTime())
		}
	}

	if !foundDot {
		t.Error("'.' entry not found")
	}
	if !foundDotDot {
		t.Error("'..' entry not found")
	}
}

func TestFS_ReadDir_DotEntriesRealInfo_OS(t *testing.T) {
	// Use OS filesystem to verify real directory info
	testDir := t.TempDir()
	osFS := os.DirFS(testDir)

	mfs := NewFS()
	if err := mfs.Mount("/test", osFS); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	entries, err := mfs.ReadDir("/test")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() == "." {
			info, err := entry.Info()
			if err != nil {
				t.Fatalf("Info() failed for '.': %v", err)
			}

			// With real OS filesystem, ModTime should be set
			if info.ModTime().IsZero() {
				t.Error("'.' entry should have non-zero ModTime from actual OS directory")
			}

			t.Logf("'.' entry (OS): Size=%d, ModTime=%v", info.Size(), info.ModTime())
		}

		if entry.Name() == ".." {
			info, err := entry.Info()
			if err != nil {
				t.Fatalf("Info() failed for '..': %v", err)
			}

			// Parent directory should also have ModTime set with real OS
			if info.ModTime().IsZero() {
				t.Error("'..' entry should have non-zero ModTime from parent OS directory")
			}

			t.Logf("'..' entry (OS): Size=%d, ModTime=%v", info.Size(), info.ModTime())
		}
	}
}

func BenchmarkFS_Open(b *testing.B) {
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/mount", testFS); err != nil {
		b.Fatalf("Mount failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f, err := mfs.Open("/mount/file.txt")
		if err != nil {
			b.Fatal(err)
		}
		f.Close()
	}
}

func BenchmarkFS_ReadDir(b *testing.B) {
	testFS := fstest.MapFS{
		"dir/file1.txt": &fstest.MapFile{Data: []byte("content1")},
		"dir/file2.txt": &fstest.MapFile{Data: []byte("content2")},
		"dir/file3.txt": &fstest.MapFile{Data: []byte("content3")},
	}

	mfs := NewFS()
	if err := mfs.Mount("/mount", testFS); err != nil {
		b.Fatalf("Mount failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mfs.ReadDir("/mount/dir")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestFS_Module(t *testing.T) {
	script := `
		// Example usage of the fs module

		const fs = require('/lib/fs');
		const process = require('/lib/process');

		console.println('=== FS Module Example ===\n');

		// 1. Read a file
		try {
			console.println('1. Reading /lib/fs/index.js (first 100 chars):');
			const content = fs.readFileSync('/lib/fs/index.js', 'utf8');
			console.println(content.substring(0, 100) + '...\n');
		} catch (e) {
			console.println('Error reading file:', e);
			process.exit(1);
		}


		// 2. Create tmp directory
		try {
			console.println('2. Creating directory /work/tmp:');
			fs.mkdirSync('/work/tmp');
			console.println('Directory created');
			
			console.println('Checking if directory exists:', fs.existsSync('/work/tmp'));
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}
			

		// 3. Write and read a file
		try {
			console.println('3. Writing to /work/tmp/test.txt:');
			fs.writeFileSync('/work/tmp/test.txt', 'Hello from fs module!\n', 'utf8');
			console.println('File written successfully');
			
			console.println('Reading back /work/tmp/test.txt:');
			const content = fs.readFileSync('/work/tmp/test.txt', 'utf8');
			console.println(content);
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 4. Append to a file
		try {
			console.println('4. Appending to /work/tmp/test.txt:');
			fs.appendFileSync('/work/tmp/test.txt', 'Appended line!\n', 'utf8');
			const content = fs.readFileSync('/work/tmp/test.txt', 'utf8');
			console.println('File content after append:');
			console.println(content);
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 5. Check if file exists
		try {
			console.println('5. Checking if files exist:');
			console.println('/work/tmp/test.txt exists:', fs.existsSync('/work/tmp/test.txt'));
			console.println('/work/tmp/nonexistent.txt exists:', fs.existsSync('/work/tmp/nonexistent.txt'));
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 6. Get file stats
		try {
			console.println('6. Getting stats for /work/tmp/test.txt:');
			const stats = fs.statSync('/work/tmp/test.txt');
			console.println('Is file:', stats.isFile());
			console.println('Is directory:', stats.isDirectory());
			console.println('Size:', stats.size, 'bytes');
			console.println('Modified:', stats.mtime);
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 7. List directory contents
		try {
			console.println('7. Listing /lib directory:');
			const files = fs.readdirSync('/lib');
			files.forEach(file => console.println('  -', file));
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 8. List directory with file types
		try {
			console.println('8. Listing /lib with file types:');
			const entries = fs.readdirSync('/lib', { withFileTypes: true });
			entries.forEach(entry => {
				const type = entry.isDirectory() ? '[DIR]' : '[FILE]';
				console.println('  ' + type + ' ' + entry.name);
			});
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 9. Create nested directories
		try {
			console.println('9. Creating nested directories /work/tmp/a/b/c:');
			fs.mkdirSync('/work/tmp/a/b/c', { recursive: true });
			console.println('Nested directories created');
			
			console.println('Removing nested directories:');
			fs.rmSync('/work/tmp/a', { recursive: true });
			console.println('Nested directories removed');
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 10. Copy file
		try {
			console.println('10. Copying /work/tmp/test.txt to /work/tmp/test-copy.txt:');
			fs.copyFileSync('/work/tmp/test.txt', '/work/tmp/test-copy.txt');
			console.println('File copied');
			
			const original = fs.readFileSync('/work/tmp/test.txt', 'utf8');
			const copy = fs.readFileSync('/work/tmp/test-copy.txt', 'utf8');
			console.println('Original and copy match:', original === copy);
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 11. Rename file
		try {
			console.println('11. Renaming /work/tmp/test-copy.txt to /work/tmp/test-renamed.txt:');
			fs.renameSync('/work/tmp/test-copy.txt', '/work/tmp/test-renamed.txt');
			console.println('File renamed');
			console.println('Old file exists:', fs.existsSync('/work/tmp/test-copy.txt'));
			console.println('New file exists:', fs.existsSync('/work/tmp/test-renamed.txt'));
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 12. Clean up
		try {
			console.println('12. Cleaning up test files:');
			fs.unlinkSync('/work/tmp/test.txt');
			fs.unlinkSync('/work/tmp/test-renamed.txt');
			console.println('Test files removed');
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		// 13. Remove tmp directory
		try {
			console.println('13. Removing directory:');
			fs.rmdirSync('/work/tmp');
			console.println('Directory removed');
			console.println();
		} catch (e) {
			console.println('Error:', e);
			process.exit(1);
		}

		console.println('\n=== Example Complete ===');
	`

	// Run the test script
	tc := TestCase{
		name:   "module_fs_complete",
		script: script,
		output: []string{}, // Output will be checked during execution, not a simple string match
	}

	t.Run(tc.name, func(t *testing.T) {
		conf := Config{
			Name: tc.name,
			Code: tc.script,
			Dir:  "../test/",
			Env: map[string]any{
				"PATH": "/lib:/work:/sbin",
				"PWD":  "/work",
			},
			Reader:      &bytes.Buffer{},
			Writer:      &bytes.Buffer{},
			ExecBuilder: testExecBuilder,
		}
		jr, err := New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/process", jr.Process)

		if err := jr.Run(); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()

		// Check that key operations completed successfully
		expectedStrings := []string{
			"=== FS Module Example ===",
			"1. Reading /lib/fs/index.js (first 100 chars):",
			"2. Creating directory /work/tmp:",
			"Directory created",
			"Checking if directory exists: true",
			"3. Writing to /work/tmp/test.txt:",
			"File written successfully",
			"Reading back /work/tmp/test.txt:",
			"Hello from fs module!",
			"4. Appending to /work/tmp/test.txt:",
			"File content after append:",
			"Appended line!",
			"5. Checking if files exist:",
			"/work/tmp/test.txt exists: true",
			"/work/tmp/nonexistent.txt exists: false",
			"6. Getting stats for /work/tmp/test.txt:",
			"Is file: true",
			"Is directory: false",
			"7. Listing /lib directory:",
			"8. Listing /lib with file types:",
			"9. Creating nested directories /work/tmp/a/b/c:",
			"Nested directories created",
			"Removing nested directories:",
			"Nested directories removed",
			"10. Copying /work/tmp/test.txt to /work/tmp/test-copy.txt:",
			"File copied",
			"Original and copy match: true",
			"11. Renaming /work/tmp/test-copy.txt to /work/tmp/test-renamed.txt:",
			"File renamed",
			"Old file exists: false",
			"New file exists: true",
			"12. Cleaning up test files:",
			"Test files removed",
			"13. Removing directory:",
			"Directory removed",
			"=== Example Complete ===",
		}

		for _, expected := range expectedStrings {
			if !strings.Contains(gotOutput, expected) {
				t.Errorf("Expected output to contain %q, but it didn't.\nFull output:\n%s", expected, gotOutput)
			}
		}

		t.Logf("Full output:\n%s", gotOutput)
	})
}
