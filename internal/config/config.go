package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Config holds the application configuration settings
type Config struct {
	GeminiAPIKey       string `json:"gemini_api_key"`
	GeminiModel        string `json:"gemini_model"`
	MonitoredGroupJID  string `json:"monitored_group_jid"`
	MonitoredGroupName string `json:"monitored_group_name"`
	Port               string `json:"port"`
}

// Manager handles concurrent access and persistence of the Config
type Manager struct {
	filePath string
	mu       sync.RWMutex
	config   Config
}

// NewManager initializes the configuration manager. It ensures the data directory exists and loads or creates the config file.
func NewManager(dataDir string) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(dataDir, "config.json")
	mgr := &Manager{
		filePath: configPath,
		config: Config{
			GeminiModel: "gemini-3.5-flash", // Default active model
			Port:        "8080",             // Default port
		},
	}

	if err := mgr.load(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Save default configuration
			if err := mgr.Save(); err != nil {
				return nil, fmt.Errorf("failed to initialize config file: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	return mgr, nil
}

// load reads the config file from disk. RWMutex should be locked by caller if calling concurrently, but load is usually called during initialization.
func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// Apply default port if empty
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	// Apply default model if empty
	if cfg.GeminiModel == "" {
		cfg.GeminiModel = "gemini-3.5-flash"
	}

	m.config = cfg
	return nil
}

// Get returns a copy of the current configuration (thread-safe)
func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Update updates the configuration fields and persists the changes to disk
func (m *Manager) Update(apiKey, model, groupJID, groupName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.GeminiAPIKey = apiKey
	m.config.GeminiModel = model
	m.config.MonitoredGroupJID = groupJID
	m.config.MonitoredGroupName = groupName

	return m.saveUnlocked()
}

// Save persists the current configuration to disk (thread-safe)
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveUnlocked()
}

// saveUnlocked writes the configuration to disk. Caller must hold the write lock.
func (m *Manager) saveUnlocked() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
