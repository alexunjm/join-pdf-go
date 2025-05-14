package pdf

import (
	"io/fs"
	"testing"
	"time"
)

// TestDataBuilder para construir datos de prueba de archivos
type MockFileBuilder struct {
	name    string
	isDir   bool
	size    int64
	modTime time.Time
}

// MockFile implementa la interfaz fs.DirEntry
type MockFile struct {
	name    string
	isDir   bool
	size    int64
	modTime time.Time
}

func (m MockFile) Name() string               { return m.name }
func (m MockFile) IsDir() bool                { return m.isDir }
func (m MockFile) Type() fs.FileMode          { return fs.ModePerm }
func (m MockFile) Info() (fs.FileInfo, error) { return nil, nil }

// Métodos del Builder
func NewMockFileBuilder() *MockFileBuilder {
	return &MockFileBuilder{
		name:    "default.pdf",
		isDir:   false,
		size:    1024,
		modTime: time.Now(),
	}
}

func (b *MockFileBuilder) WithName(name string) *MockFileBuilder {
	b.name = name
	return b
}

func (b *MockFileBuilder) AsDirectory() *MockFileBuilder {
	b.isDir = true
	return b
}

func (b *MockFileBuilder) WithSize(size int64) *MockFileBuilder {
	b.size = size
	return b
}

func (b *MockFileBuilder) WithModTime(modTime time.Time) *MockFileBuilder {
	b.modTime = modTime
	return b
}

func (b *MockFileBuilder) Build() MockFile {
	return MockFile{
		name:    b.name,
		isDir:   b.isDir,
		size:    b.size,
		modTime: b.modTime,
	}
}

// Mock de la función os.ReadDir
type MockReadDirFunc func(string) ([]fs.DirEntry, error)

// Tests
func TestListFilesWithExtension(t *testing.T) {
	// Tabla de casos de prueba
	tests := []struct {
		name          string
		directory     string
		extension     string
		mockFiles     []MockFile
		expectedFiles []string
		expectedError error
	}{
		{
			name:      "Archivos PDF ordenados por número",
			directory: "/test/dir",
			extension: ".pdf",
			mockFiles: []MockFile{
				NewMockFileBuilder().WithName("2-document.pdf").Build(),
				NewMockFileBuilder().WithName("12-document.pdf").Build(),
				NewMockFileBuilder().WithName("1-document.pdf").Build(),
				NewMockFileBuilder().WithName("16-document.pdf").Build(),
				NewMockFileBuilder().WithName("31-document.pdf").Build(),
				NewMockFileBuilder().WithName("30-document.pdf").Build(),
				NewMockFileBuilder().WithName("3-document.pdf").Build(),
			},
			expectedFiles: []string{
				"1-document.pdf",
				"2-document.pdf",
				"3-document.pdf",
				"12-document.pdf",
				"16-document.pdf",
				"30-document.pdf",
				"31-document.pdf",
			},
			expectedError: nil,
		},
		{
			name:      "Ignorar archivos no PDF",
			directory: "/test/dir",
			extension: ".pdf",
			mockFiles: []MockFile{
				NewMockFileBuilder().WithName("1-document.pdf").Build(),
				NewMockFileBuilder().WithName("document.txt").Build(),
				NewMockFileBuilder().WithName("2-document.pdf").Build(),
			},
			expectedFiles: []string{
				"1-document.pdf",
				"2-document.pdf",
			},
			expectedError: nil,
		},
		{
			name:      "Ignorar directorios",
			directory: "/test/dir",
			extension: ".pdf",
			mockFiles: []MockFile{
				NewMockFileBuilder().WithName("1-document.pdf").Build(),
				NewMockFileBuilder().WithName("folder").AsDirectory().Build(),
				NewMockFileBuilder().WithName("2-document.pdf").Build(),
			},
			expectedFiles: []string{
				"1-document.pdf",
				"2-document.pdf",
			},
			expectedError: nil,
		},
		{
			name:      "Ordenamiento alfabético para archivos sin números",
			directory: "/test/dir",
			extension: ".pdf",
			mockFiles: []MockFile{
				NewMockFileBuilder().WithName("c-document.pdf").Build(),
				NewMockFileBuilder().WithName("a-document.pdf").Build(),
				NewMockFileBuilder().WithName("b-document.pdf").Build(),
			},
			expectedFiles: []string{
				"a-document.pdf",
				"b-document.pdf",
				"c-document.pdf",
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			originalOsReadDir := osReadDir
			defer func() { osReadDir = originalOsReadDir }()

			osReadDir = func(dir string) ([]fs.DirEntry, error) {
				if dir != tt.directory {
					t.Errorf("Expected directory %s, got %s", tt.directory, dir)
				}
				var entries []fs.DirEntry
				for _, mockFile := range tt.mockFiles {
					entries = append(entries, mockFile)
				}
				return entries, tt.expectedError
			}

			// Act
			result, err := ListFilesWithExtension(tt.directory, tt.extension)

			// Assert
			if err != tt.expectedError {
				t.Errorf("Expected error %v, got %v", tt.expectedError, err)
			}

			if len(result) != len(tt.expectedFiles) {
				t.Errorf("Expected %d files, got %d", len(tt.expectedFiles), len(result))
			}

			for i, expectedFile := range tt.expectedFiles {
				if result[i] != expectedFile {
					t.Errorf("Expected file %s at position %d, got %s", expectedFile, i, result[i])
				}
			}
		})
	}
}
