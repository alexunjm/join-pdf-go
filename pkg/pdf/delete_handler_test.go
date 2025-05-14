package pdf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Object Mother para crear objetos de prueba comunes
type DeleteTestMother struct{}

func (m *DeleteTestMother) CreateValidRequest(method, folder string, files []string) *http.Request {
	body := DeleteFilesRequest{
		Folder: folder,
		Files:  files,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(method, "/delete", strings.NewReader(string(jsonBody)))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(context.WithValue(req.Context(), userCodeKey, "testUser"))
}

func (m *DeleteTestMother) CreateValidResponse() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

// Test Data Builder para las solicitudes de eliminación
type DeleteRequestBuilder struct {
	folder string
	files  []string
	method string
}

func NewDeleteRequestBuilder() *DeleteRequestBuilder {
	return &DeleteRequestBuilder{
		folder: "test-folder",
		files:  []string{},
		method: http.MethodDelete,
	}
}

func (b *DeleteRequestBuilder) WithFolder(folder string) *DeleteRequestBuilder {
	b.folder = folder
	return b
}

func (b *DeleteRequestBuilder) WithFiles(files []string) *DeleteRequestBuilder {
	b.files = files
	return b
}

func (b *DeleteRequestBuilder) WithMethod(method string) *DeleteRequestBuilder {
	b.method = method
	return b
}

func (b *DeleteRequestBuilder) Build(t *testing.T) (*http.Request, *httptest.ResponseRecorder) {
	mother := &DeleteTestMother{}
	return mother.CreateValidRequest(b.method, b.folder, b.files), mother.CreateValidResponse()
}

func TestDeleteFilesHandler(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func(t *testing.T) (*http.Request, *httptest.ResponseRecorder)
		setupFiles     func(t *testing.T, userPath string) []string
		expectedStatus int
		expectedFiles  []string
	}{
		{
			name: "Eliminar archivos específicos exitosamente",
			setupRequest: func(t *testing.T) (*http.Request, *httptest.ResponseRecorder) {
				return NewDeleteRequestBuilder().
					WithFolder("test-folder").
					WithFiles([]string{"1-document.pdf", "2-document.pdf"}).
					Build(t)
			},
			setupFiles: func(t *testing.T, userPath string) []string {
				files := []string{"1-document.pdf", "2-document.pdf", "3-document.pdf"}
				folderPath := filepath.Join(userPath, "test-folder")
				os.MkdirAll(folderPath, os.ModePerm)
				for _, f := range files {
					os.Create(filepath.Join(folderPath, f))
				}
				return files
			},
			expectedStatus: http.StatusOK,
			expectedFiles:  []string{"3-document.pdf"},
		},
		{
			name: "Eliminar todos los archivos cuando no se especifica lista",
			setupRequest: func(t *testing.T) (*http.Request, *httptest.ResponseRecorder) {
				return NewDeleteRequestBuilder().
					WithFolder("test-folder").
					WithFiles([]string{}).
					Build(t)
			},
			setupFiles: func(t *testing.T, userPath string) []string {
				files := []string{"1-document.pdf", "2-document.pdf"}
				folderPath := filepath.Join(userPath, "test-folder")
				os.MkdirAll(folderPath, os.ModePerm)
				for _, f := range files {
					os.Create(filepath.Join(folderPath, f))
				}
				return files
			},
			expectedStatus: http.StatusOK,
			expectedFiles:  []string{},
		},
		{
			name: "Error al intentar eliminar archivo que no existe",
			setupRequest: func(t *testing.T) (*http.Request, *httptest.ResponseRecorder) {
				return NewDeleteRequestBuilder().
					WithFolder("test-folder").
					WithFiles([]string{"nonexistent.pdf"}).
					Build(t)
			},
			setupFiles: func(t *testing.T, userPath string) []string {
				files := []string{"1-document.pdf"}
				folderPath := filepath.Join(userPath, "test-folder")
				os.MkdirAll(folderPath, os.ModePerm)
				for _, f := range files {
					os.Create(filepath.Join(folderPath, f))
				}
				return files
			},
			expectedStatus: http.StatusBadRequest,
			expectedFiles:  []string{"1-document.pdf"},
		},
		{
			name: "Error cuando el método no es DELETE",
			setupRequest: func(t *testing.T) (*http.Request, *httptest.ResponseRecorder) {
				return NewDeleteRequestBuilder().
					WithMethod(http.MethodPost).
					WithFolder("test-folder").
					WithFiles([]string{"1-document.pdf"}).
					Build(t)
			},
			setupFiles:     func(t *testing.T, userPath string) []string { return []string{} },
			expectedStatus: http.StatusMethodNotAllowed,
			expectedFiles:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tempDir := t.TempDir()
			userPath := filepath.Join(tempDir, "testUser")
			os.MkdirAll(userPath, os.ModePerm)

			originalGetUserStoragePath := getUserStoragePathFn
			defer func() { getUserStoragePathFn = originalGetUserStoragePath }()

			getUserStoragePathFn = func(r *http.Request) (string, error) {
				return userPath, nil
			}

			tt.setupFiles(t, userPath)
			req, rr := tt.setupRequest(t)

			// Act
			DeleteFilesHandler(rr, req)

			// Assert
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}

			if tt.expectedStatus == http.StatusOK {
				files, _ := ListFilesWithExtension(filepath.Join(userPath, "test-folder"), ".pdf")
				if len(files) != len(tt.expectedFiles) {
					t.Errorf("expected %d files remaining, got %d", len(tt.expectedFiles), len(files))
				}
				for i, expectedFile := range tt.expectedFiles {
					if i < len(files) && files[i] != expectedFile {
						t.Errorf("expected file %s at position %d, got %s", expectedFile, i, files[i])
					}
				}
			}
		})
	}
}
