package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SupabaseService maneja la integración con Supabase
type SupabaseService struct {
	url    string
	apiKey string
	client *http.Client
}

// CargaData representa los datos de una carga para Supabase
type CargaData struct {
	Material          string `json:"material"`
	Presentacion      string `json:"presentacion"`
	Peso              string `json:"peso"`
	TipoEquipo        string `json:"tipoEquipo"`
	LocalidadCarga    string `json:"localidadCarga"`
	LocalidadDescarga string `json:"localidadDescarga"`
	FechaCarga        string `json:"fechaCarga"`
	FechaDescarga     string `json:"fechaDescarga"`
	Telefono          string `json:"telefono"`
	Correo            string `json:"correo"`
	PuntoReferencia   string `json:"puntoReferencia"`
	Precio            string `json:"precio"`
	FormaDePago       string `json:"formaDePago"`
	Observaciones     string `json:"observaciones"`
}

// SupabaseCarga representa la estructura de carga en Supabase
type SupabaseCarga struct {
	DadorID           string    `json:"dador_id"`
	Peso              string    `json:"peso"`
	UbicacionInicial  string    `json:"ubicacioninicial_id"`
	UbicacionFinal    string    `json:"ubicacionfinal_id"`
	TelefonoDador     string    `json:"telefonodador"`
	PuntoReferencia   string    `json:"puntoreferencia"`
	MaterialID        string    `json:"material_id"`
	PresentacionID    string    `json:"presentacion_id"`
	ValorViaje        string    `json:"valorviaje"`
	PagoPor           string    `json:"pagopor"`
	OtroPagoPor       *string   `json:"otropagopor"`
	FechaCarga        string    `json:"fechacarga"`
	FechaDescarga     string    `json:"fechadescarga"`
	FormaDePagoID     string    `json:"formadepago_id"`
	Email             string    `json:"email"`
	TipoEquipo        string    `json:"tipo_equipo"`
	Observaciones     string    `json:"observaciones"`
}

// Ubicacion representa una ubicación en Supabase
type Ubicacion struct {
	ID        string  `json:"id"`
	Direccion string  `json:"direccion"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
}

// GoogleMapsResponse representa la respuesta de Google Maps Geocoding API
type GoogleMapsResponse struct {
	Results []GoogleMapsResult `json:"results"`
	Status  string             `json:"status"`
}

// GoogleMapsResult representa un resultado de geocoding
type GoogleMapsResult struct {
	FormattedAddress string                 `json:"formatted_address"`
	Geometry         GoogleMapsGeometry     `json:"geometry"`
	AddressComponents []GoogleMapsComponent `json:"address_components"`
}

// GoogleMapsGeometry representa la geometría de una ubicación
type GoogleMapsGeometry struct {
	Location GoogleMapsLocation `json:"location"`
}

// GoogleMapsLocation representa coordenadas lat/lng
type GoogleMapsLocation struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// GoogleMapsComponent representa un componente de dirección
type GoogleMapsComponent struct {
	LongName  string   `json:"long_name"`
	ShortName string   `json:"short_name"`
	Types     []string `json:"types"`
}

// Material representa un material en Supabase
type Material struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

// Presentacion representa una presentación en Supabase
type Presentacion struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

// TipoEquipo representa un tipo de equipo en Supabase
type TipoEquipo struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

// FormaPago representa una forma de pago en Supabase
type FormaPago struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

// NewSupabaseService crea una nueva instancia del servicio de Supabase
func NewSupabaseService() *SupabaseService {
	return &SupabaseService{
		url:    "https://ikiusmdtltakhmmlljsp.supabase.co",
		apiKey: getEnvAI("SUPABASE_KEY", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImlraXVzbWR0bHRha2htbWxsanNwIiwicm9sZSI6ImFub24iLCJpYXQiOjE3MzQ2MjkyMzEsImV4cCI6MjA1MDIwNTIzMX0.q6NMMUK2ONGFs-b10XZySVlQiCXSLsjZbtBZyUTiVjc"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CrearCargasDesdeJSON procesa un JSON de cargas y las crea en Supabase
func (s *SupabaseService) CrearCargasDesdeJSON(jsonData []byte) ([]string, error) {
	// Parsear JSON de IA
	var cargas []CargaData
	if err := json.Unmarshal(jsonData, &cargas); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}
	
	var createdIDs []string
	
	for i, carga := range cargas {
		// Crear carga individual
		cargaID, err := s.crearCarga(carga, i)
		if err != nil {
			return createdIDs, fmt.Errorf("failed to create carga %d: %v", i+1, err)
		}
		
		createdIDs = append(createdIDs, cargaID)
		
		// Pequeña pausa entre cargas
		time.Sleep(100 * time.Millisecond)
	}
	
	return createdIDs, nil
}

// crearCarga crea una carga individual en Supabase
func (s *SupabaseService) crearCarga(carga CargaData, _ int) (string, error) {
	// Obtener/crear ubicaciones
	ubicacionInicialID, err := s.obtenerOCrearUbicacion(carga.LocalidadCarga)
	if err != nil {
		return "", fmt.Errorf("failed to get initial location: %v", err)
	}
	
	ubicacionFinalID, err := s.obtenerOCrearUbicacion(carga.LocalidadDescarga)
	if err != nil {
		return "", fmt.Errorf("failed to get final location: %v", err)
	}
	
	// Mapear materiales/presentaciones/equipos a IDs
	materialID, err := s.obtenerMaterialID(carga.Material)
	if err != nil {
		return "", fmt.Errorf("failed to get material ID: %v", err)
	}
	
	presentacionID, err := s.obtenerPresentacionID(carga.Presentacion)
	if err != nil {
		return "", fmt.Errorf("failed to get presentacion ID: %v", err)
	}
	
	tipoEquipoID, err := s.obtenerTipoEquipoID(carga.TipoEquipo)
	if err != nil {
		return "", fmt.Errorf("failed to get tipo equipo ID: %v", err)
	}
	
	formaPagoID, err := s.obtenerFormaPagoID(carga.FormaDePago)
	if err != nil {
		return "", fmt.Errorf("failed to get forma pago ID: %v", err)
	}
	
	// Crear estructura de carga para Supabase
	supabaseCarga := SupabaseCarga{
		DadorID:          "20d060b6-33b5-4222-a039-a3e603d979be", // Dador configurado
		Peso:             carga.Peso,
		UbicacionInicial: ubicacionInicialID,
		UbicacionFinal:   ubicacionFinalID,
		TelefonoDador:    s.validarTelefono(carga.Telefono),
		PuntoReferencia:  carga.PuntoReferencia,
		MaterialID:       materialID,
		PresentacionID:   presentacionID,
		ValorViaje:       carga.Precio,
		PagoPor:          "Otros",
		OtroPagoPor:      nil,
		FechaCarga:       s.formatearFecha(carga.FechaCarga),
		FechaDescarga:    s.formatearFecha(carga.FechaDescarga),
		FormaDePagoID:    formaPagoID,
		Email:            carga.Correo,
		TipoEquipo:       tipoEquipoID,
		Observaciones:    carga.Observaciones,
	}
	
	// Insertar en Supabase
	return s.insertarCarga(supabaseCarga)
}

// obtenerOCrearUbicacion obtiene o crea una ubicación usando geocoding
func (s *SupabaseService) obtenerOCrearUbicacion(direccion string) (string, error) {
	if direccion == "" {
		return "", fmt.Errorf("direccion is empty")
	}
	
	fmt.Printf("🔍 Buscando ubicación: %s\n", direccion)
	
	// Buscar ubicación existente
	ubicacionID, err := s.buscarUbicacion(direccion)
	if err == nil && ubicacionID != "" {
		fmt.Printf("✅ Ubicación encontrada: %s (ID: %s)\n", direccion, ubicacionID)
		return ubicacionID, nil
	}
	
	fmt.Printf("📍 Ubicación no encontrada, creando nueva...\n")
	
	// Si no existe, crear nueva con geocoding
	newID, err := s.crearUbicacionConGeocoding(direccion)
	if err != nil {
		return "", fmt.Errorf("failed to create ubicacion for '%s': %v", direccion, err)
	}
	
	fmt.Printf("✅ Ubicación creada: %s (ID: %s)\n", direccion, newID)
	return newID, nil
}

// buscarUbicacion busca una ubicación existente en Supabase
func (s *SupabaseService) buscarUbicacion(direccion string) (string, error) {
	url := fmt.Sprintf("%s/rest/v1/ubicaciones?direccion=eq.%s&select=id", s.url, direccion)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("apikey", s.apiKey)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("supabase error: %d", resp.StatusCode)
	}
	
	var ubicaciones []Ubicacion
	if err := json.NewDecoder(resp.Body).Decode(&ubicaciones); err != nil {
		return "", err
	}
	
	if len(ubicaciones) > 0 {
		return ubicaciones[0].ID, nil
	}
	
	return "", fmt.Errorf("ubicacion not found")
}

// crearUbicacionConGeocoding crea una nueva ubicación usando geocoding
func (s *SupabaseService) crearUbicacionConGeocoding(direccion string) (string, error) {
	fmt.Printf("🗺️ Obteniendo coordenadas para: %s\n", direccion)
	
	// Obtener coordenadas usando Google Maps API
	coords, err := s.obtenerCoordenadas(direccion)
	if err != nil {
		return "", fmt.Errorf("failed to get coordinates: %v", err)
	}
	
	fmt.Printf("📍 Coordenadas obtenidas: lat=%f, lng=%f\n", coords.Lat, coords.Lng)
	
	// Crear ubicación en Supabase
	ubicacion := Ubicacion{
		Direccion: direccion,
		Lat:       coords.Lat,
		Lng:       coords.Lng,
	}
	
	fmt.Printf("💾 Insertando ubicación en Supabase...\n")
	id, err := s.insertarUbicacion(ubicacion)
	if err != nil {
		return "", fmt.Errorf("insertarUbicacion failed: %v", err)
	}
	
	fmt.Printf("✅ Ubicación insertada con ID: %s\n", id)
	return id, nil
}

// Coordenadas representa coordenadas y datos de ubicación
type Coordenadas struct {
	Lat float64
	Lng float64
}

// obtenerCoordenadas obtiene coordenadas usando Google Maps API
func (s *SupabaseService) obtenerCoordenadas(direccion string) (*Coordenadas, error) {
	// Usar la misma API key que funciona en el script JS
	apiKey := getEnvAI("GOOGLE_MAPS_API_KEY", "AIzaSyASe9Id-6Dr6lxr5mCb7O3l2HlmNrY-mRU")
	
	// Limpiar y encodear la dirección correctamente
	cleanAddress := strings.TrimSpace(direccion)
	
	// Crear URL base
	baseURL := "https://maps.googleapis.com/maps/api/geocode/json"
	params := fmt.Sprintf("?address=%s&components=country:AR&region=AR&key=%s&language=es",
		strings.ReplaceAll(cleanAddress, " ", "+"),
		apiKey)
	
	url := baseURL + params
	
	fmt.Printf("🗺️ Geocoding: %s\n", cleanAddress)
	
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to call Google Maps API: %v", err)
	}
	defer resp.Body.Close()
	
	// Leer el body completo primero para logging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Google Maps response: %v", err)
	}
	
	// Verificar si la respuesta es HTML (error de API key o quota)
	if strings.HasPrefix(string(bodyBytes), "<") || strings.Contains(string(bodyBytes), "<!DOCTYPE") {
		fmt.Printf("❌ Google Maps devolvió HTML en lugar de JSON (posible problema de API key)\n")
		maxLen := 200
		if len(bodyBytes) < maxLen {
			maxLen = len(bodyBytes)
		}
		fmt.Printf("📄 Primeros caracteres: %s\n", string(bodyBytes[:maxLen]))
		return nil, fmt.Errorf("Google Maps API returned HTML instead of JSON - check API key and quota")
	}
	
	var geocodeResp GoogleMapsResponse
	if err := json.Unmarshal(bodyBytes, &geocodeResp); err != nil {
		fmt.Printf("❌ Error parseando respuesta de Google Maps\n")
		maxLen := 500
		if len(bodyBytes) < maxLen {
			maxLen = len(bodyBytes)
		}
		fmt.Printf("📄 Respuesta recibida: %s\n", string(bodyBytes[:maxLen]))
		return nil, fmt.Errorf("failed to decode Google Maps response: %v", err)
	}
	
	fmt.Printf("🗺️ Google Maps status: %s\n", geocodeResp.Status)
	
	if geocodeResp.Status != "OK" {
		return nil, fmt.Errorf("geocoding failed with status: %s", geocodeResp.Status)
	}
	
	if len(geocodeResp.Results) == 0 {
		return nil, fmt.Errorf("no geocoding results for: %s", direccion)
	}
	
	result := geocodeResp.Results[0]
	coords := &Coordenadas{
		Lat: result.Geometry.Location.Lat,
		Lng: result.Geometry.Location.Lng,
	}
	
	fmt.Printf("✅ Coordenadas obtenidas: lat=%f, lng=%f\n", coords.Lat, coords.Lng)
	fmt.Printf("📍 Dirección detectada por Google: %s\n", result.FormattedAddress)
	
	return coords, nil
}

// insertarUbicacion inserta una nueva ubicación en Supabase
func (s *SupabaseService) insertarUbicacion(ubicacion Ubicacion) (string, error) {
	// Usar select=id para obtener solo el ID en la respuesta (igual que JS)
	url := fmt.Sprintf("%s/rest/v1/ubicaciones?select=id", s.url)
	
	// Preparar datos para insertar (sin ID)
	ubicacionData := map[string]interface{}{
		"direccion": ubicacion.Direccion,
		"lat":       ubicacion.Lat,
		"lng":       ubicacion.Lng,
	}
	
	jsonData, err := json.Marshal(ubicacionData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal ubicacion: %v", err)
	}
	
	fmt.Printf("📤 Enviando a Supabase: %s\n", string(jsonData))
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("apikey", s.apiKey)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
	
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("📥 Respuesta de Supabase (%d): %s\n", resp.StatusCode, string(body))
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%d - %s", resp.StatusCode, string(body))
	}
	
	// Parsear respuesta - puede ser array o objeto único
	// Intentar primero como array
	var ubicaciones []map[string]interface{}
	if err := json.Unmarshal(body, &ubicaciones); err == nil && len(ubicaciones) > 0 {
		// Obtener el ID de la respuesta
		if id, ok := ubicaciones[0]["id"].(string); ok && id != "" {
			fmt.Printf("✅ ID obtenido (array): %s\n", id)
			return id, nil
		}
	}
	
	// Intentar como objeto único (.single() en JS)
	var ubicacionSingle map[string]interface{}
	if err := json.Unmarshal(body, &ubicacionSingle); err == nil {
		if id, ok := ubicacionSingle["id"].(string); ok && id != "" {
			fmt.Printf("✅ ID obtenido (single): %s\n", id)
			return id, nil
		}
	}
	
	return "", fmt.Errorf("could not extract ID from response: %s", string(body))
}

// insertarCarga inserta una nueva carga en Supabase
func (s *SupabaseService) insertarCarga(carga SupabaseCarga) (string, error) {
	url := fmt.Sprintf("%s/rest/v1/cargas", s.url)
	
	jsonData, err := json.Marshal(carga)
	if err != nil {
		return "", err
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	
	req.Header.Set("apikey", s.apiKey)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
	
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to insert carga: %d - %s", resp.StatusCode, string(body))
	}
	
	var cargas []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&cargas); err != nil {
		return "", err
	}
	
	if len(cargas) > 0 {
		if id, ok := cargas[0]["id"].(string); ok {
			return id, nil
		}
	}
	
	return "", fmt.Errorf("no carga ID returned")
}

// Funciones auxiliares para mapear IDs

func (s *SupabaseService) obtenerMaterialID(nombre string) (string, error) {
	materiales := map[string]string{
		"Agroquímicos":             "b93fcec6-b173-47d4-be39-52d272bc8a87",
		"Alimentos y bebidas":      "97ca010a-6375-40d6-880e-051ba3818516",
		"Fertilizante":             "220193b8-bafe-476d-a225-433b567db256",
		"Ganado":                   "b181d3b8-f92c-44fd-9334-c34041ef29df",
		"Girasol":                  "bdb09420-ef80-4de0-a038-03285d48fb92",
		"Maiz":                     "6def5e3b-358d-46e5-9170-8e42a2c97d23",
		"Maquinarias":              "4e9efe3d-8eb6-4600-96dd-eb35cbad8699",
		"Materiales construcción":  "49ebf50f-d37a-446c-927c-f463fda953e0",
		"Otras cargas generales":   "8cd407f6-297e-4730-a1d6-15a2ac485809",
		"Otros cultivos":           "c921caf8-5e2b-4fdb-9190-d7fe624771bf",
		"Refrigerados":             "176bf83f-3109-431d-8a35-1d157ae4d91f",
		"Soja":                     "4edee3cb-7308-4d1b-96e7-a378052004e7",
		"Trigo":                    "04ba66a5-6a87-4243-b8ed-45baf6cfc2e8",
	}
	
	if id, exists := materiales[nombre]; exists {
		return id, nil
	}
	return materiales["Otras cargas generales"], nil // Default
}

func (s *SupabaseService) obtenerPresentacionID(nombre string) (string, error) {
	presentaciones := map[string]string{
		"Big Bag":   "ca7cf082-837c-4c14-b2ad-c85f0821d86c",
		"Bolsa":     "e676ca36-8a96-4338-9a41-2692c18664f5",
		"Granel":    "3923f3da-eb7d-4438-8fcd-74d53891c392",
		"Otros":     "510db5c8-eb5f-4ef1-b23a-96d4e4869f2d",
		"Pallet":    "234a739b-6666-4595-a8df-51e840c09599",
	}
	
	if id, exists := presentaciones[nombre]; exists {
		return id, nil
	}
	return presentaciones["Otros"], nil // Default
}

func (s *SupabaseService) obtenerTipoEquipoID(nombre string) (string, error) {
	tiposEquipo := map[string]string{
		"Batea":            "85bf5951-50a7-4abc-af6e-ea3b9550d97d",
		"Camioneta":        "8fa614ad-af82-4909-b0ff-b1d288ea97a3",
		"CamionJaula":      "1933f25d-eb8e-43cf-b2e8-5224ab6a4ef2",
		"Carreton":         "779ba2a1-f4e3-4121-be59-3e1cdd2c6da8",
		"Chasis y Acoplado": "a16bdd90-df15-4adf-8cc4-7a74ad375ffd",
		"Furgon":           "9eb2b303-5c92-45ae-8120-4cc40dd3fa49",
		"Otros":            "e1c0cc7d-27fb-4206-9fe3-280ffc40d742",
		"Semi":             "be085c4d-f6a5-4f36-b869-9ec606bef794",
		"Tolva":            "5939b8d1-71d7-4e37-851b-db388856945e",
	}
	
	if id, exists := tiposEquipo[nombre]; exists {
		return id, nil
	}
	return tiposEquipo["Otros"], nil // Default
}

func (s *SupabaseService) obtenerFormaPagoID(nombre string) (string, error) {
	formasPago := map[string]string{
		"Cheque":        "48c0c41f-ed88-4b3a-b06d-9a1f03131fe8",
		"E-check":       "692684a5-9103-4257-a3e3-6486f907177a",
		"Efectivo":      "c96c6cd8-8742-4a8c-9df6-18554a7c87af",
		"Otros":         "e0f74bf6-2886-44da-9469-c68ffaf53e4f",
		"Transferencia": "7b998228-2121-465b-9721-679a320e50ae",
	}
	
	if id, exists := formasPago[nombre]; exists {
		return id, nil
	}
	return formasPago["Efectivo"], nil // Default
}

// validarTelefono valida y formatea un teléfono argentino
func (s *SupabaseService) validarTelefono(telefono string) string {
	if telefono == "" {
		return ""
	}
	
	// Remover espacios y caracteres especiales
	limpio := strings.ReplaceAll(telefono, " ", "")
	limpio = strings.ReplaceAll(limpio, "-", "")
	limpio = strings.ReplaceAll(limpio, "(", "")
	limpio = strings.ReplaceAll(limpio, ")", "")
	
	// Si no tiene código de país, agregar +54
	if !strings.HasPrefix(limpio, "+54") && !strings.HasPrefix(limpio, "54") {
		if strings.HasPrefix(limpio, "9") {
			// Agregar código de área si es necesario
			limpio = "+549" + limpio[1:]
		} else {
			limpio = "+54" + limpio
		}
	}
	
	return limpio
}

// formatearFecha convierte fecha a formato ISO
func (s *SupabaseService) formatearFecha(fecha string) string {
	if fecha == "" {
		return time.Now().Format("2006-01-02")
	}
	
	// Intentar parsear diferentes formatos
	formatos := []string{
		"02/01/2006",
		"2006-01-02",
		"02-01-2006",
	}
	
	for _, formato := range formatos {
		if t, err := time.Parse(formato, fecha); err == nil {
			return t.Format("2006-01-02")
		}
	}
	
	// Si no se puede parsear, usar fecha actual
	return time.Now().Format("2006-01-02")
}
