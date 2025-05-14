package pdf

// DeleteFilesRequest estructura para la solicitud de eliminación de archivos
type DeleteFilesRequest struct {
	Folder string   `json:"folder"`
	Files  []string `json:"files"`
}
