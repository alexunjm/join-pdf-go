package pdf

import (
	"context" // Necesario para pasar el código de usuario en el contexto
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync" // Necesario para proteger el mapa de códigos válidos

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// --- Estado Global (para códigos de acceso válidos) ---
// Usamos un mapa en memoria para almacenar los códigos válidos.
// En un sistema de producción, esto debería ser persistente (DB, caché distribuida).
// Usamos un Mutex para hacer el acceso al mapa seguro en entornos concurrentes.
var (
	validCodes = map[string]bool{"alex": true}
	codesMutex sync.Mutex
)

// --- Clave de Contexto para pasar el código de usuario ---
// Es una buena práctica usar un tipo no exportado para evitar colisiones de claves de contexto.
type contextKey string

const userCodeKey contextKey = "userCode"

// GenerateCodeHandler: Genera un nuevo código de acceso basado en nombre y fecha.
// Este código se almacena en memoria como válido.
func GenerateCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Parsear el formulario para obtener nombre y fecha
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error al parsear el formulario", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	date := r.FormValue("date") // Asumimos que la fecha viene en un formato string

	if name == "" || date == "" {
		http.Error(w, "Bad Request: Nombre y fecha son requeridos", http.StatusBadRequest)
		return
	}

	// 1. Combinar nombre y fecha para crear los datos a codificar
	// Puedes usar un separador si quieres, o simplemente concatenar.
	// Concatenar es suficiente para generar un código único basado en la combinación.
	dataToEncode := name + date

	// 2. Codificar los datos combinados a Base64
	// Convertimos el string a []byte antes de codificar
	code := base64.StdEncoding.EncodeToString([]byte(dataToEncode))

	// 3. Agregar el código generado al mapa de códigos válidos
	// Es crucial usar el mutex para proteger el acceso al mapa
	codesMutex.Lock()       // Bloquear el mutex antes de escribir en el mapa
	validCodes[code] = true // Marcar el código como válido
	codesMutex.Unlock()     // Desbloquear el mutex después de escribir

	// 4. Responder al cliente con el código generado
	w.Header().Set("Content-Type", "text/plain") // Indicar que la respuesta es texto plano
	w.WriteHeader(http.StatusOK)                 // Opcional: indicar explícitamente el status 200 OK
	fmt.Fprintln(w, code)                        // Escribir el código en la respuesta

}

// Si el código es válido, se establece una cookie de autenticación.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Parsear el formulario para obtener el código de acceso
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error al parsear el formulario", http.StatusBadRequest)
		return
	}

	accessCode := r.FormValue("access_code")
	if accessCode == "" {
		http.Error(w, "Falta el código de acceso", http.StatusBadRequest)
		return
	}

	// Verificar si el código de acceso es válido (thread-safe)
	codesMutex.Lock()
	isValid := validCodes[accessCode]
	codesMutex.Unlock()

	if !isValid {
		http.Error(w, "Código de acceso inválido", http.StatusUnauthorized)
		return
	}

	// Si el código es válido, establecer una cookie de autenticación
	cookie := http.Cookie{
		Name:     "auth_code", // Nombre de la cookie
		Value:    accessCode,  // El valor es el código de acceso
		Path:     "/",         // La cookie es válida para todas las rutas
		HttpOnly: true,        // La cookie no es accesible desde JavaScript del cliente
		// Secure:   true,        // Descomentar en producción con HTTPS
		SameSite: http.SameSiteLaxMode, // Protección básica contra CSRF
		// Expires: time.Now().Add(24 * time.Hour), // Opcional: establecer expiración
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/view/pdf", http.StatusSeeOther)
}

// --- Middleware de Autenticación ---
// Esta función envuelve a los handlers que requieren autenticación.
// Verifica la cookie "auth_code" y valida el código.
// Si es válido, agrega el código al contexto de la petición para que los handlers lo usen.
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Intentar obtener la cookie de autenticación
		cookie, err := r.Cookie("auth_code")
		if err != nil {
			// Cookie no encontrada o error al leerla
			http.Error(w, "No autenticado. Por favor, inicie sesión.", http.StatusUnauthorized)
			return
		}

		accessCode := cookie.Value

		// Verificar si el código de acceso de la cookie es válido (thread-safe)
		codesMutex.Lock()
		isValid := validCodes[accessCode]
		codesMutex.Unlock()

		if !isValid {
			// Código de acceso en la cookie no válido
			http.Error(w, "Código de acceso inválido o expirado. Por favor, inicie sesión de nuevo.", http.StatusUnauthorized)
			return
		}

		// Si el código es válido, agregar el código al contexto de la petición
		ctx := context.WithValue(r.Context(), userCodeKey, accessCode)
		reqWithContext := r.WithContext(ctx)

		// Llamar al siguiente handler en la cadena con la petición modificada
		next.ServeHTTP(w, reqWithContext)
	}
}

// --- Handlers Existentes Modificados para Usar el Código de Usuario ---

// Helper para obtener la ruta base de almacenamiento del usuario
func getUserStoragePath(r *http.Request) (string, error) {
	// Obtener el código de usuario del contexto (establecido por el middleware)
	userCode, ok := r.Context().Value(userCodeKey).(string)
	if !ok {
		// Esto no debería pasar si el middleware se aplica correctamente,
		// pero es una verificación defensiva.
		return "", fmt.Errorf("código de usuario no encontrado en el contexto")
	}

	path, _ := os.Getwd() // Obtiene el directorio de trabajo actual
	// Construye la ruta base de almacenamiento incluyendo el código de usuario
	return filepath.Join(path, "archivos", userCode), nil
}

func ListHandler(w http.ResponseWriter, r *http.Request) {

	// Obtener la ruta base de almacenamiento del usuario
	userStoragePath, err := getUserStoragePath(r)
	if err != nil {
		http.Error(w, "Error interno de autenticación", http.StatusInternalServerError)
		return
	}
	folder := r.URL.Query().Get("folder")
	if folder == "" {
		http.Error(w, "Falta el nombre de la carpeta", http.StatusBadRequest)
		return
	}

	folderPath := filepath.Join(userStoragePath, folder)
	fmt.Println(folderPath)

	files, err := ListFilesWithExtension(folderPath, ".pdf")
	if err != nil {
		http.Error(w, "Error al listar archivos", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Obtener la ruta base de almacenamiento del usuario
	userStoragePath, err := getUserStoragePath(r)
	if err != nil {
		http.Error(w, "Error interno de autenticación", http.StatusInternalServerError)
		return
	}

	folder := r.FormValue("folder")
	if folder == "" {
		http.Error(w, "Falta el nombre de la carpeta", http.StatusBadRequest)
		return
	}

	// Construir la ruta completa de la carpeta dentro del espacio del usuario
	folderPath := filepath.Join(userStoragePath, folder)
	fmt.Println("Subiendo a:", folderPath) // Log para depuración

	// Crear la carpeta del usuario y la carpeta específica si no existen
	if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
		http.Error(w, "No se pudo crear la carpeta del usuario/carpeta", http.StatusInternalServerError)
		return
	}

	destFiles, err := ListFilesWithExtension(folderPath, ".pdf")
	if err != nil {
		http.Error(w, "Error leyendo el directorio", http.StatusBadRequest)
		return
	}
	counter := len(destFiles)

	// Parsear archivos
	err = r.ParseMultipartForm(32 << 20) // 32 MB
	if err != nil {
		http.Error(w, "Error al parsear el formulario", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["pdfs"]
	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Error al abrir archivo", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		filename := fileHeader.Filename
		numStr := strings.Split(filename, "-")[0]
		if _, err := strconv.Atoi(numStr); err != nil {
			filename = fmt.Sprintf("%d-%s", i+1+counter, filename)
		}
		dst, err := os.Create(filepath.Join(folderPath, filename))
		if err != nil {
			http.Error(w, "Error al guardar archivo", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, "Error al copiar archivo", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Archivos subidos correctamente"))
}

var osReadDir = os.ReadDir // Alias para facilitar mocking en tests si fuera necesario

// ListFilesWithExtension: Función auxiliar que lista y ordena archivos PDF en un directorio dado.
// No necesita saber del código de usuario, solo opera sobre la ruta que recibe.
func ListFilesWithExtension(dir string, ext string) ([]string, error) {
	fmt.Println("Leyendo directorio:", dir) // Log para depuración
	files, err := osReadDir(dir)
	if err != nil {
		return nil, err // Devuelve el error si el directorio no existe o hay problemas de permisos
	}

	var matchedFiles []string
	for _, file := range files {
		// Ignorar directorios y solo incluir archivos con la extensión especificada
		if !file.IsDir() && strings.HasSuffix(file.Name(), ext) {
			matchedFiles = append(matchedFiles, file.Name())
		}
	}

	// Ordenar los archivos basándose en el primer número encontrado en el nombre
	sort.Slice(matchedFiles, func(i, j int) bool {
		// Expresión regular para encontrar números
		re := regexp.MustCompile(`[0-9] `)

		// Encontrar todos los números en ambos nombres de archivo
		foundN1 := re.FindAllString(matchedFiles[i], -1)
		foundN2 := re.FindAllString(matchedFiles[j], -1)

		// Si alguno no tiene números o hay algún problema, usar orden alfabético (simplificado)
		// Para robustez, se deberían manejar estos casos explícitamente.
		if len(foundN1) == 0 || len(foundN2) == 0 {
			return matchedFiles[i] < matchedFiles[j] // Fallback alfabético
		}

		// Convertir el primer número encontrado a entero
		n1, err1 := strconv.Atoi(foundN1[0])
		n2, err2 := strconv.Atoi(foundN2[0])

		// Si la conversión falla, usar orden alfabético (simplificado)
		if err1 != nil || err2 != nil {
			return matchedFiles[i] < matchedFiles[j] // Fallback alfabético
		}

		// Comparar los números
		return n1 < n2
	})

	return matchedFiles, nil
}

func GenerateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Obtener la ruta base de almacenamiento del usuario
	userStoragePath, err := getUserStoragePath(r)
	if err != nil {
		http.Error(w, "Error interno de autenticación", http.StatusInternalServerError)
		return
	}

	folder := r.FormValue("folder")
	if folder == "" {
		http.Error(w, "Falta el nombre de la carpeta", http.StatusBadRequest)
		return
	}

	// Llamar a la función auxiliar para unir PDFs, pasándole la ruta base del usuario y la carpeta
	err = joinPDFs(userStoragePath, folder) // joinPDFs ahora recibe la ruta base del usuario
	if err != nil {
		http.Error(w, "Error al unir PDFs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("PDF generado correctamente"))
}

func joinPDFs(path, folder string) error {
	folderPath := filepath.Join(path, folder)
	files, err := ListFilesWithExtension(folderPath, ".pdf")
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no se encontraron archivos PDF en la ruta proporcionada")
	}
	outputFilePath := filepath.Join(folderPath, "../", folder+".pdf")
	filesToJoin := make([]string, len(files))
	for i := 0; i < len(files); i++ {
		filesToJoin[i] = filepath.Join(folderPath, files[i])
	}
	err = api.MergeCreateFile(filesToJoin, outputFilePath, false, nil)
	if err != nil {
		return err
	}
	return nil
}

func DownloadHandler(w http.ResponseWriter, r *http.Request) {
	// Obtener la ruta base de almacenamiento del usuario
	userStoragePath, err := getUserStoragePath(r)
	if err != nil {
		http.Error(w, "Error interno de autenticación", http.StatusInternalServerError)
		return
	}
	folder := r.URL.Query().Get("folder")
	if folder == "" {
		http.Error(w, "Falta el nombre de la carpeta", http.StatusBadRequest)
		return
	}
	pdfPath := filepath.Join(userStoragePath, folder+".pdf")
	http.ServeFile(w, r, pdfPath)
}
