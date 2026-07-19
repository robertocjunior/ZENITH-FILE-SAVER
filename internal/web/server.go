package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"google.golang.org/genai"
	"zenith-file-saver/internal/config"
	"zenith-file-saver/internal/db"
	"zenith-file-saver/internal/whatsapp"
)

//go:embed static
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Simple for local and docker setup
	},
}

// WSMessage defines the JSON format for WebSocket messages
type WSMessage struct {
	Type    string      `json:"type"`    // "state", "qr", "log", "file_processed"
	Payload interface{} `json:"payload"` // raw data or string
}

// Server handles HTTP APIs and WebSockets
type Server struct {
	configMgr *config.Manager
	waMgr     *whatsapp.ClientManager
	database  *db.DB

	clients   map[*websocket.Conn]bool
	clientsMu sync.Mutex
	broadcast chan WSMessage
}

// NewServer creates and starts the web server routing system
func NewServer(cfg *config.Manager, wa *whatsapp.ClientManager, database *db.DB) *Server {
	s := &Server{
		configMgr: cfg,
		waMgr:     wa,
		database:  database,
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan WSMessage, 100),
	}

	go s.runBroadcastLoop()
	return s
}

// RegisterRoutes sets up the routing map
func (s *Server) RegisterRoutes() (http.Handler, error) {
	mux := http.NewServeMux()

	// REST APIs
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/models", s.handleModels)
	mux.HandleFunc("/api/groups", s.handleGroups)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/logs", s.handleLogs)

	// WebSocket Endpoint
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Serve Static Assets from Embedded FS
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded static filesystem: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	return mux, nil
}

// BroadcastLog sends a formatted log message to all WebSocket clients
func (s *Server) BroadcastLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	s.broadcast <- WSMessage{
		Type:    "log",
		Payload: msg,
	}
}

// BroadcastState sends the current WhatsApp connection state
func (s *Server) BroadcastState(state string) {
	s.broadcast <- WSMessage{
		Type:    "state",
		Payload: state,
	}
}

// BroadcastQR sends the base64 pairing QR Code
func (s *Server) BroadcastQR(qrBase64 string) {
	s.broadcast <- WSMessage{
		Type:    "qr",
		Payload: qrBase64,
	}
}

// BroadcastFileProcessed sends a newly processed file log
func (s *Server) BroadcastFileProcessed(fileLog db.FileLog) {
	s.broadcast <- WSMessage{
		Type:    "file_processed",
		Payload: fileLog,
	}
}

// runBroadcastLoop coordinates sending events to active WS connections
func (s *Server) runBroadcastLoop() {
	for msg := range s.broadcast {
		s.clientsMu.Lock()
		for client := range s.clients {
			err := client.WriteJSON(msg)
			if err != nil {
				client.Close()
				delete(s.clients, client)
			}
		}
		s.clientsMu.Unlock()
	}
}

// handleWebSocket upgrades incoming requests to WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	// Feed initial connection status and QR code if available
	state, qrCode := s.waMgr.GetState()
	conn.WriteJSON(WSMessage{Type: "state", Payload: state})
	if qrCode != "" {
		conn.WriteJSON(WSMessage{Type: "qr", Payload: qrCode})
	}

	// Keep the socket connection open to read (we discard incoming WS input)
	go func() {
		defer func() {
			s.clientsMu.Lock()
			delete(s.clients, conn)
			s.clientsMu.Unlock()
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

// GET /api/config (retrieve configurations)
// POST /api/config (save configurations)
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		cfg := s.configMgr.Get()
		// Redact the API key slightly for safety when reading it
		// but since the UI needs to load it to edit, we can send it or return it directly.
		// For usability, we return it as is since it's only accessible locally.
		json.NewEncoder(w).Encode(cfg)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			GeminiAPIKey       string `json:"gemini_api_key"`
			GeminiModel        string `json:"gemini_model"`
			MonitoredGroupJID  string `json:"monitored_group_jid"`
			MonitoredGroupName string `json:"monitored_group_name"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Update config file
		if err := s.configMgr.Update(req.GeminiAPIKey, req.GeminiModel, req.MonitoredGroupJID, req.MonitoredGroupName); err != nil {
			http.Error(w, fmt.Sprintf("failed to save config: %v", err), http.StatusInternalServerError)
			return
		}

		// Update WhatsApp client monitored group
		s.waMgr.SetMonitoredGroup(req.MonitoredGroupJID)

		s.BroadcastLog("[Config] Configurações atualizadas: Grupo monitorado setado para '%s'", req.MonitoredGroupName)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// GET /api/groups (list joined WhatsApp groups)
func (s *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	groups, err := s.waMgr.GetGroups()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type GroupItem struct {
		JID  string `json:"jid"`
		Name string `json:"name"`
	}

	var list []GroupItem
	for _, g := range groups {
		list = append(list, GroupItem{
			JID:  g.JID.String(),
			Name: g.Name,
		})
	}

	json.NewEncoder(w).Encode(list)
}

// POST /api/logout (force logout of WhatsApp session)
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.waMgr.Logout(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// GET /api/logs (retrieve files processing log history)
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	logs, err := s.database.GetLogs(50) // Return last 50 logs
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(logs)
}

// GET /api/models (list available Gemini models)
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	apiKey := s.configMgr.Get().GeminiAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		apiKey = r.URL.Query().Get("key")
	}

	if apiKey == "" {
		http.Error(w, "Gemini API key is not configured", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create client: %v", err), http.StatusInternalServerError)
		return
	}

	response, err := client.Models.List(ctx, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to list models: %v", err), http.StatusInternalServerError)
		return
	}

	type ModelItem struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
	}

	var list []ModelItem
	for _, m := range response.Items {
		lowerName := strings.ToLower(m.Name)
		// Only list models that have "gemini" and support classification (not text-embeddings)
		if strings.Contains(lowerName, "gemini") && !strings.Contains(lowerName, "embedding") {
			cleanName := m.Name
			if strings.HasPrefix(cleanName, "models/") {
				cleanName = strings.TrimPrefix(cleanName, "models/")
			}
			list = append(list, ModelItem{
				Name:        cleanName,
				DisplayName: m.DisplayName,
			})
		}
	}

	json.NewEncoder(w).Encode(list)
}
