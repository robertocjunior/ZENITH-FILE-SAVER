package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// FileLog represents a record of a processed file
type FileLog struct {
	ID           int64     `json:"id"`
	SenderName   string    `json:"sender_name"`
	SenderJID    string    `json:"sender_jid"`
	OriginalName string    `json:"original_name"`
	NewName      string    `json:"new_name"`
	Category     string    `json:"category"`
	StoragePath  string    `json:"storage_path"`
	FileHash     string    `json:"file_hash"`
	Timestamp    time.Time `json:"timestamp"`
	Status       string    `json:"status"` // "success", "failed"
	ErrorMessage string    `json:"error_message,omitempty"`
}

// DB handles database connections and queries
type DB struct {
	conn *sql.DB
}

// NewDB initializes the SQLite database for application logging
func NewDB(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "app.db")
	conn, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_cache=shared&_journal_mode=WAL", dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Set connection limits
	conn.SetMaxOpenConns(1)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.conn.Close()
}

// migrate creates the necessary tables if they do not exist
func (d *DB) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS file_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender_name TEXT NOT NULL,
		sender_jid TEXT NOT NULL,
		original_name TEXT NOT NULL,
		new_name TEXT NOT NULL,
		category TEXT NOT NULL,
		storage_path TEXT NOT NULL,
		file_hash TEXT NOT NULL DEFAULT '',
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		status TEXT NOT NULL,
		error_message TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_file_logs_timestamp ON file_logs(timestamp DESC);
	`
	_, err := d.conn.Exec(query)
	return err
}

// LogFile records a file processing outcome in the database
func (d *DB) LogFile(senderName, senderJID, originalName, newName, category, storagePath, fileHash, status, errMsg string) (*FileLog, error) {
	query := `
	INSERT INTO file_logs (sender_name, sender_jid, original_name, new_name, category, storage_path, file_hash, status, error_message, timestamp)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	res, err := d.conn.Exec(query, senderName, senderJID, originalName, newName, category, storagePath, fileHash, status, errMsg, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert file log: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve last insert id: %w", err)
	}

	return &FileLog{
		ID:           id,
		SenderName:   senderName,
		SenderJID:    senderJID,
		OriginalName: originalName,
		NewName:      newName,
		Category:     category,
		StoragePath:  storagePath,
		FileHash:     fileHash,
		Timestamp:    now,
		Status:       status,
		ErrorMessage: errMsg,
	}, nil
}

// GetLogs retrieves the recent file processing history
func (d *DB) GetLogs(limit int) ([]FileLog, error) {
	query := `
	SELECT id, sender_name, sender_jid, original_name, new_name, category, storage_path, file_hash, timestamp, status, COALESCE(error_message, '')
	FROM file_logs
	ORDER BY timestamp DESC
	LIMIT ?
	`
	rows, err := d.conn.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query file logs: %w", err)
	}
	defer rows.Close()

	var logs []FileLog
	for rows.Next() {
		var log FileLog
		err := rows.Scan(
			&log.ID,
			&log.SenderName,
			&log.SenderJID,
			&log.OriginalName,
			&log.NewName,
			&log.Category,
			&log.StoragePath,
			&log.FileHash,
			&log.Timestamp,
			&log.Status,
			&log.ErrorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// HashExists checks if a successful file log with the given hash already exists
func (d *DB) HashExists(hash string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM file_logs WHERE file_hash = ? AND status = 'success')"
	err := d.conn.QueryRow(query, hash).Scan(&exists)
	return exists, err
}
