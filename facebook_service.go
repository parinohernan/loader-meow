package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// FacebookService maneja la conexión y obtención de publicaciones de Facebook
type FacebookService struct {
	accessToken   string
	messageStore *MessageStore
	httpClient   *http.Client
	groups       map[string]*FacebookGroup // Mapa de groupID -> FacebookGroup
}

// FacebookGroup representa la configuración de un grupo de Facebook
type FacebookGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	AccessToken string    `json:"access_token"` // Token específico del grupo (opcional)
	Enabled     bool      `json:"enabled"`
	LastFetch   time.Time `json:"last_fetch"`
	CreatedAt   time.Time `json:"created_at"`
}

// FacebookPost representa una publicación de Facebook
type FacebookPost struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	CreatedTime string `json:"created_time"`
	From        struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"from"`
}

// FacebookResponse representa la respuesta de la Graph API
type FacebookResponse struct {
	Data   []FacebookPost `json:"data"`
	Paging struct {
		Next string `json:"next"`
	} `json:"paging"`
	Error *FacebookError `json:"error,omitempty"`
}

// FacebookError representa un error de la API de Facebook
type FacebookError struct {
	Message   string `json:"message"`
	Type      string `json:"type"`
	Code      int    `json:"code"`
	SubCode   int    `json:"error_subcode"`
}

// NewFacebookService crea una nueva instancia del servicio de Facebook
func NewFacebookService(accessToken string, messageStore *MessageStore) *FacebookService {
	service := &FacebookService{
		accessToken:  accessToken,
		messageStore: messageStore,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		groups:       make(map[string]*FacebookGroup),
	}
	
	// Cargar grupos guardados
	service.loadGroups()
	
	return service
}

// AddGroup agrega un nuevo grupo de Facebook a la lista
func (s *FacebookService) AddGroup(groupID, groupName, customAccessToken string) error {
	// Usar token personalizado si se proporciona, sino usar el token por defecto
	token := customAccessToken
	if token == "" {
		token = s.accessToken
	}
	
	group := &FacebookGroup{
		ID:          groupID,
		Name:        groupName,
		AccessToken: token,
		Enabled:     true,
		CreatedAt:   time.Now(),
	}
	
	s.groups[groupID] = group
	
	// Guardar en base de datos
	return s.saveGroup(group)
}

// RemoveGroup elimina un grupo de Facebook
func (s *FacebookService) RemoveGroup(groupID string) error {
	delete(s.groups, groupID)
	return s.deleteGroup(groupID)
}

// GetGroups obtiene todos los grupos configurados
func (s *FacebookService) GetGroups() []FacebookGroup {
	groups := make([]FacebookGroup, 0, len(s.groups))
	for _, group := range s.groups {
		groups = append(groups, *group)
	}
	return groups
}

// GetGroup obtiene un grupo específico
func (s *FacebookService) GetGroup(groupID string) (*FacebookGroup, error) {
	group, exists := s.groups[groupID]
	if !exists {
		return nil, fmt.Errorf("group not found: %s", groupID)
	}
	return group, nil
}

// ToggleGroup habilita/deshabilita un grupo
func (s *FacebookService) ToggleGroup(groupID string, enabled bool) error {
	group, exists := s.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}
	
	group.Enabled = enabled
	return s.updateGroup(group)
}

// FetchGroupPosts obtiene publicaciones de un grupo de Facebook
func (s *FacebookService) FetchGroupPosts(groupID string, limit int) ([]FacebookPost, error) {
	group, exists := s.groups[groupID]
	if !exists {
		return nil, fmt.Errorf("group not found: %s", groupID)
	}
	
	if !group.Enabled {
		return nil, fmt.Errorf("group is disabled: %s", groupID)
	}
	
	// Construir URL de la Graph API
	// Usar el token del grupo si está disponible, sino el token por defecto
	token := group.AccessToken
	if token == "" {
		token = s.accessToken
	}
	
	url := fmt.Sprintf(
		"https://graph.facebook.com/v18.0/%s/feed?fields=id,message,created_time,from&limit=%d&access_token=%s",
		groupID,
		limit,
		token,
	)
	
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching posts: %v", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		var fbError FacebookResponse
		json.Unmarshal(body, &fbError)
		if fbError.Error != nil {
			return nil, fmt.Errorf("Facebook API error [%d]: %s", fbError.Error.Code, fbError.Error.Message)
		}
		return nil, fmt.Errorf("Facebook API error: %s - %s", resp.Status, string(body))
	}
	
	var fbResponse FacebookResponse
	if err := json.Unmarshal(body, &fbResponse); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}
	
	// Actualizar última fecha de obtención
	group.LastFetch = time.Now()
	s.updateGroup(group)
	
	return fbResponse.Data, nil
}

// StorePostsAsMessages almacena las publicaciones como mensajes en la base de datos
func (s *FacebookService) StorePostsAsMessages(groupID string, posts []FacebookPost) error {
	group, exists := s.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}
	
	chatJID := fmt.Sprintf("facebook_group_%s", groupID)
	
	for _, post := range posts {
		// Solo procesar si tiene mensaje
		if post.Message == "" {
			continue
		}
		
		// Parsear fecha de creación
		createdTime, err := time.Parse(time.RFC3339, post.CreatedTime)
		if err != nil {
			// Intentar otro formato si falla
			createdTime, err = time.Parse("2006-01-02T15:04:05-0700", post.CreatedTime)
			if err != nil {
				// Intentar formato sin zona horaria
				createdTime, err = time.Parse("2006-01-02T15:04:05", post.CreatedTime)
				if err != nil {
					createdTime = time.Now()
				}
			}
		}
		
		// Crear mensaje
		messageID := post.ID
		senderPhone := post.From.ID
		senderName := post.From.Name
		content := post.Message
		
		// Almacenar en MessageStore
		err = s.messageStore.StoreMessage(
			messageID,
			chatJID,
			senderPhone,
			senderName,
			content,
			createdTime,
			false, // isFromMe
			"",    // mediaType
			"",    // filename
			"",    // url
			nil,   // mediaKey
			nil,   // fileSHA256
			nil,   // fileEncSHA256
			0,     // fileLength
		)
		
		if err != nil {
			return fmt.Errorf("error storing post %s: %v", post.ID, err)
		}
		
		// Actualizar chat
		chatName := fmt.Sprintf("Facebook: %s", group.Name)
		err = s.messageStore.StoreChat(chatJID, chatName, createdTime)
		if err != nil {
			// No crítico, continuar
		}
	}
	
	return nil
}

// FetchAndStorePosts obtiene y almacena publicaciones de un grupo
func (s *FacebookService) FetchAndStorePosts(groupID string, limit int) error {
	posts, err := s.FetchGroupPosts(groupID, limit)
	if err != nil {
		return err
	}
	
	return s.StorePostsAsMessages(groupID, posts)
}

// FetchAllGroupsPosts obtiene publicaciones de todos los grupos habilitados
func (s *FacebookService) FetchAllGroupsPosts(limitPerGroup int) map[string]error {
	errors := make(map[string]error)
	
	for groupID, group := range s.groups {
		if !group.Enabled {
			continue
		}
		
		err := s.FetchAndStorePosts(groupID, limitPerGroup)
		if err != nil {
			errors[groupID] = err
		}
	}
	
	return errors
}

// ===== PERSISTENCIA DE GRUPOS =====

// saveGroup guarda un grupo en la base de datos
func (s *FacebookService) saveGroup(group *FacebookGroup) error {
	query := `
		INSERT INTO facebook_groups (group_id, name, access_token, enabled, last_fetch, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			access_token = VALUES(access_token),
			enabled = VALUES(enabled),
			last_fetch = VALUES(last_fetch)
	`
	
	_, err := s.messageStore.db.Exec(
		query,
		group.ID,
		group.Name,
		group.AccessToken,
		group.Enabled,
		group.LastFetch,
		group.CreatedAt,
	)
	
	return err
}

// updateGroup actualiza un grupo en la base de datos
func (s *FacebookService) updateGroup(group *FacebookGroup) error {
	query := `
		UPDATE facebook_groups
		SET name = ?, access_token = ?, enabled = ?, last_fetch = ?
		WHERE group_id = ?
	`
	
	_, err := s.messageStore.db.Exec(
		query,
		group.Name,
		group.AccessToken,
		group.Enabled,
		group.LastFetch,
		group.ID,
	)
	
	return err
}

// deleteGroup elimina un grupo de la base de datos
func (s *FacebookService) deleteGroup(groupID string) error {
	query := `DELETE FROM facebook_groups WHERE group_id = ?`
	_, err := s.messageStore.db.Exec(query, groupID)
	return err
}

// loadGroups carga los grupos desde la base de datos
func (s *FacebookService) loadGroups() error {
	// Crear tabla si no existe
	if err := s.createGroupsTable(); err != nil {
		return err
	}
	
	query := `SELECT group_id, name, access_token, enabled, last_fetch, created_at FROM facebook_groups`
	rows, err := s.messageStore.db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	for rows.Next() {
		var group FacebookGroup
		var lastFetch, createdAt sql.NullTime
		
		err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.AccessToken,
			&group.Enabled,
			&lastFetch,
			&createdAt,
		)
		if err != nil {
			continue
		}
		
		if lastFetch.Valid {
			group.LastFetch = lastFetch.Time
		}
		if createdAt.Valid {
			group.CreatedAt = createdAt.Time
		} else {
			group.CreatedAt = time.Now()
		}
		
		s.groups[group.ID] = &group
	}
	
	return nil
}

// createGroupsTable crea la tabla de grupos si no existe
func (s *FacebookService) createGroupsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS facebook_groups (
			group_id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(500) NOT NULL,
			access_token TEXT,
			enabled BOOLEAN DEFAULT TRUE,
			last_fetch TIMESTAMP NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_enabled (enabled)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`
	
	_, err := s.messageStore.db.Exec(query)
	return err
}

// ===== CONFIGURACIÓN =====

// LoadConfig carga la configuración de Facebook desde variables de entorno o archivo
func LoadFacebookConfig() (string, error) {
	// Intentar desde variable de entorno
	accessToken := os.Getenv("FACEBOOK_ACCESS_TOKEN")
	if accessToken != "" {
		return accessToken, nil
	}
	
	// Intentar desde archivo de configuración
	configFile := "facebook-config.env"
	if _, err := os.Stat(configFile); err == nil {
		content, err := os.ReadFile(configFile)
		if err == nil {
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "FACEBOOK_ACCESS_TOKEN=") {
					token := strings.TrimPrefix(line, "FACEBOOK_ACCESS_TOKEN=")
					token = strings.Trim(token, `"`)
					token = strings.Trim(token, "'")
					return token, nil
				}
			}
		}
	}
	
	return "", fmt.Errorf("Facebook access token not found. Set FACEBOOK_ACCESS_TOKEN environment variable or create facebook-config.env file")
}

