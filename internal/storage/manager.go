package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Manager handles the persistent storage of downloaded files
type Manager struct {
	baseDir string
}

// NewManager creates a storage manager with a base directory (usually "FILES")
func NewManager(baseDir string) (*Manager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base storage directory: %w", err)
	}
	return &Manager{baseDir: baseDir}, nil
}

// SaveFile organizes and writes the file to disk based on date, sender, and classification
func (m *Manager) SaveFile(
	fileData []byte,
	mimeType string,
	originalName string,
	senderName string,
	docDate string, // Expects "DD-MM-YYYY"
	description string,
) (string, string, error) {
	// 1. Sanitize the sender name to prevent path traversal or invalid directory names
	sanitizedSender := sanitizeName(senderName)
	if sanitizedSender == "" {
		sanitizedSender = "desconhecido"
	}

	// 2. Parse date into year, month, and day-month parts
	year, month, dayMonth, err := parseDate(docDate)
	if err != nil {
		// Fallback to "outro" structure if date parsing fails
		year = "0000"
		month = "00"
		dayMonth = "00-00"
	}

	// 3. Determine the correct file extension
	ext := getExtension(originalName, mimeType)

	// 4. Sanitize description
	sanitizedDesc := sanitizeDescription(description)
	if sanitizedDesc == "" {
		sanitizedDesc = "arquivo"
	}

	// 5. Construct the new filename: DD-MM-description.ext
	newFilename := fmt.Sprintf("%s-%s%s", dayMonth, sanitizedDesc, ext)

	// 6. Build the destination path: FILES/<YEAR>/<MONTH>/<SENDER_NAME>/<NEW_FILENAME>
	targetDir := filepath.Join(m.baseDir, year, month, sanitizedSender)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	destPath := filepath.Join(targetDir, newFilename)

	// 7. Handle file name collision: if file exists, append a counter (e.g. -1, -2) before the extension
	destPath, newFilename = resolveCollision(targetDir, newFilename, ext)

	// 8. Write file to disk
	if err := os.WriteFile(destPath, fileData, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write file: %w", err)
	}

	return newFilename, destPath, nil
}

// sanitizeName keeps only alphanumeric characters, spaces, dots, dashes, and underscores
func sanitizeName(name string) string {
	// Remove accents / diacritics where possible
	name = removeAccents(name)
	// Match anything that is not alphanumeric, spaces, dots, dashes, or underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9\s\.\-_]`)
	cleaned := reg.ReplaceAllString(name, "")
	// Replace spaces with underscores and trim
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.ReplaceAll(cleaned, " ", "_")
	return cleaned
}

// sanitizeDescription cleans the description to be safe for filenames (lowercase, snake_case)
func sanitizeDescription(desc string) string {
	desc = removeAccents(desc)
	desc = strings.ToLower(desc)
	// Replace spaces and special chars with hyphens
	reg := regexp.MustCompile(`[^a-z0-9\-_]`)
	cleaned := reg.ReplaceAllString(desc, "-")
	// Clean up duplicate hyphens
	for strings.Contains(cleaned, "--") {
		cleaned = strings.ReplaceAll(cleaned, "--", "-")
	}
	return strings.Trim(cleaned, "-")
}

// removeAccents replaces Portuguese accented characters with their ASCII equivalents
func removeAccents(s string) string {
	replacements := map[rune]string{
		'á': "a", 'à': "a", 'â': "a", 'ã': "a", 'ä': "a",
		'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
		'í': "i", 'ì': "i", 'î': "i", 'ï': "i",
		'ó': "o", 'ò': "o", 'ô': "o", 'õ': "o", 'ö': "o",
		'ú': "u", 'ù': "u", 'û': "u", 'ü': "u",
		'ç': "c",
		'Á': "A", 'À': "A", 'Â': "A", 'Ã': "A", 'Ä': "A",
		'É': "E", 'È': "E", 'Ê': "E", 'Ë': "E",
		'Í': "I", 'Ì': "I", 'Î': "I", 'Ï': "I",
		'Ó': "O", 'Ò': "O", 'Ô': "O", 'Õ': "O", 'Ö': "O",
		'Ú': "U", 'Ù': "U", 'Û': "U", 'Ü': "U",
		'Ç': "C",
	}

	var sb strings.Builder
	for _, r := range s {
		if val, ok := replacements[r]; ok {
			sb.WriteString(val)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// parseDate parses a "DD-MM-YYYY" string and returns (Year, Month, DD-MM, error)
func parseDate(dateStr string) (string, string, string, error) {
	parts := strings.Split(dateStr, "-")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid date format: %s", dateStr)
	}

	day := parts[0]
	month := parts[1]
	year := parts[2]

	if len(day) != 2 || len(month) != 2 || len(year) != 4 {
		return "", "", "", fmt.Errorf("invalid date parts length: %s", dateStr)
	}

	return year, month, fmt.Sprintf("%s-%s", day, month), nil
}

// getExtension returns the file extension, fallback to mime map if missing
func getExtension(originalName, mimeType string) string {
	ext := filepath.Ext(originalName)
	if ext != "" {
		return strings.ToLower(ext)
	}

	mimeType = strings.ToLower(strings.Split(mimeType, ";")[0])
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	case "application/msword":
		return ".doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.ms-excel":
		return ".xls"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	default:
		return ".bin" // Default fallback binary
	}
}

// resolveCollision renames a file by appending -1, -2 if a collision occurs
func resolveCollision(dir, filename, ext string) (string, string) {
	destPath := filepath.Join(dir, filename)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return destPath, filename
	}

	baseName := strings.TrimSuffix(filename, ext)
	counter := 1
	for {
		candidateFilename := fmt.Sprintf("%s-%d%s", baseName, counter, ext)
		candidatePath := filepath.Join(dir, candidateFilename)
		if _, err := os.Stat(candidatePath); os.IsNotExist(err) {
			return candidatePath, candidateFilename
		}
		counter++
	}
}

// SaveErrorFile saves a file that failed classification inside the ERROS/ directory
func (m *Manager) SaveErrorFile(
	fileData []byte,
	mimeType string,
	originalName string,
	senderName string,
) (string, string, error) {
	// 1. Sanitize the sender name
	sanitizedSender := sanitizeName(senderName)
	if sanitizedSender == "" {
		sanitizedSender = "desconhecido"
	}

	// 2. Resolve originalName and extension
	if originalName == "" {
		originalName = "arquivo_desconhecido"
	}
	ext := getExtension(originalName, mimeType)
	
	// Ensure the originalName has the correct extension if it doesn't already
	if !strings.HasSuffix(strings.ToLower(originalName), ext) {
		originalName = originalName + ext
	}

	// 3. Build the destination path: FILES/ERROS/<SENDER_NAME>/<ORIGINAL_NAME>
	targetDir := filepath.Join(m.baseDir, "ERROS", sanitizedSender)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create error directory %s: %w", targetDir, err)
	}

	destPath := filepath.Join(targetDir, originalName)

	// 4. Resolve name collision
	destPath, finalName := resolveCollision(targetDir, originalName, ext)

	// 5. Write file to disk
	if err := os.WriteFile(destPath, fileData, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write error file: %w", err)
	}

	return finalName, destPath, nil
}

// SaveIrrelevantFile organizes and writes an irrelevant file to disk inside the monthly IRRELEVANTES/ directory
func (m *Manager) SaveIrrelevantFile(
	fileData []byte,
	mimeType string,
	originalName string,
	senderName string,
	docDate string, // Expects "DD-MM-YYYY"
	description string,
) (string, string, error) {
	// 1. Sanitize the sender name
	sanitizedSender := sanitizeName(senderName)
	if sanitizedSender == "" {
		sanitizedSender = "desconhecido"
	}

	// 2. Parse date into year, month, and day-month parts
	year, month, dayMonth, err := parseDate(docDate)
	if err != nil {
		// Fallback if date parsing fails
		year = "0000"
		month = "00"
		dayMonth = "00-00"
	}

	// 3. Determine the correct file extension
	ext := getExtension(originalName, mimeType)

	// 4. Sanitize description
	sanitizedDesc := sanitizeDescription(description)
	if sanitizedDesc == "" {
		sanitizedDesc = "irrelevante"
	}

	// 5. Construct the new filename: DD-MM-description.ext
	newFilename := fmt.Sprintf("%s-%s%s", dayMonth, sanitizedDesc, ext)

	// 6. Build the destination path: FILES/<YEAR>/<MONTH>/IRRELEVANTES/<SENDER_NAME>/<NEW_FILENAME>
	targetDir := filepath.Join(m.baseDir, year, month, "IRRELEVANTES", sanitizedSender)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create irrelevant directory %s: %w", targetDir, err)
	}

	destPath := filepath.Join(targetDir, newFilename)

	// 7. Handle file name collision
	destPath, finalName := resolveCollision(targetDir, newFilename, ext)

	// 8. Write file to disk
	if err := os.WriteFile(destPath, fileData, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write irrelevant file: %w", err)
	}

	return finalName, destPath, nil
}
