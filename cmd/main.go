package main

// El objetivo es:
// 1.  Añadir los nuevos endpoints (`/generate-code` y `/login`) definidos en el paquete `pdf` (del paso anterior).
// 2.  Aplicar el `AuthMiddleware` a los handlers que ahora requieren autenticación (`/upload`, `/list`, `/generate`, `/download`).
// 3.  Mantener el `viewHandler` para servir una página HTML (`view.html`) que contendrá los formularios para generar código y para iniciar sesión.

// Aquí está el código `main.go` modificado y la explicación arquitectónica:

import (
	"fmt"
	"local-pruebas/pkg/pdf" // Asegúrate de que esta ruta de importación sea correcta para tu proyecto
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Estructura y función para cargar la página HTML (sin cambios significativos, solo manejo de errores)
type Page struct {
	Title string
	Body  []byte
}

func loadPage(filename string) (*Page, error) {
	fmt.Println("Attempting to load view:", filename) // Log más específico
	body, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("Error loading file:", err) // Log del error
		return nil, err                         // Devolver el error si el archivo no se puede leer
	}
	return &Page{Title: "PDF Access View", Body: body}, nil // Título más descriptivo
}

// viewHandler: Sirve la página HTML que contiene los formularios de acceso
func viewHandler(w http.ResponseWriter, r *http.Request) {
	path, _ := os.Getwd() // Obtiene el directorio de trabajo actual
	// Nota: Esta ruta asume que main.go se ejecuta desde la raíz del módulo
	// y que pkg/pdf está en esa raíz.
	viewFilePath := filepath.Join(path, "pkg", "pdf", "view.html")

	// Intentar obtener la cookie de autenticación
	_, err := r.Cookie("auth_code")
	if err != nil {
		// Cookie no encontrada o error al leerla
		// Código de acceso en la cookie no válido
		viewFilePath = filepath.Join(path, "pkg", "pdf", "view_home.html")
		// http.Error(w, "No autenticado. Por favor, inicie sesión.", http.StatusUnauthorized)
		// return
	}
	p, _ := loadPage(viewFilePath)
	fmt.Fprintf(w, "%s", p.Body)
}

func main() {
	http.HandleFunc("/view/", viewHandler)
	http.HandleFunc("/generate-code", pdf.GenerateCodeHandler)
	http.HandleFunc("/login", pdf.LoginHandler)
	// --- Handlers de PDF (Ahora protegidos por el Middleware de Autenticación) ---
	// Envolvemos cada handler con el AuthMiddleware.
	// El middleware se ejecutará primero, verificará la cookie, y si es válida,
	// llamará al handler original (UploadHandler, ListHandler, etc.)
	http.HandleFunc("/upload", pdf.AuthMiddleware(pdf.UploadHandler))
	http.HandleFunc("/list", pdf.AuthMiddleware(pdf.ListHandler))
	http.HandleFunc("/generate", pdf.AuthMiddleware(pdf.GenerateHandler))
	http.HandleFunc("/download", pdf.AuthMiddleware(pdf.DownloadHandler))

	fmt.Println("Server starting on :8080") // Mensaje de inicio del servidor
	log.Fatal(http.ListenAndServe(":8080", nil))
}
