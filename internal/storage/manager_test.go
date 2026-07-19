package storage

import (
	"os"
	"testing"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Robert", "Robert"},
		{"João Águia", "Joao_Aguia"},
		{"Dad & Mom", "Dad__Mom"},
		{"J. J. Smith", "J._J._Smith"},
		{"User/Admin", "UserAdmin"},
	}

	for _, test := range tests {
		result := sanitizeName(test.input)
		if result != test.expected {
			t.Errorf("sanitizeName(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}

func TestSanitizeDescription(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Energia Enel", "energia-enel"},
		{"transferência pix", "transferencia-pix"},
		{"Comprovante de Luz!!!", "comprovante-de-luz"},
		{"Supermercado--Carrefour", "supermercado-carrefour"},
	}

	for _, test := range tests {
		result := sanitizeDescription(test.input)
		if result != test.expected {
			t.Errorf("sanitizeDescription(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}

func TestParseDate(t *testing.T) {
	year, month, dayMonth, err := parseDate("12-07-2026")
	if err != nil {
		t.Fatalf("unexpected error parsing valid date: %v", err)
	}

	if year != "2026" {
		t.Errorf("expected year 2026, got %s", year)
	}
	if month != "07" {
		t.Errorf("expected month 07, got %s", month)
	}
	if dayMonth != "12-07" {
		t.Errorf("expected dayMonth 12-07, got %s", dayMonth)
	}

	_, _, _, err = parseDate("invalid-date")
	if err == nil {
		t.Error("expected error for invalid date format, got nil")
	}
}

func TestSaveFileAndCollision(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	data := []byte("dummy data")
	filename1, path1, err := mgr.SaveFile(data, "application/pdf", "", "João", "15-08-2026", "Luz Enel")
	if err != nil {
		t.Fatalf("failed to save first file: %v", err)
	}

	expectedName := "15-08-luz-enel.pdf"
	if filename1 != expectedName {
		t.Errorf("expected name %s, got %s", expectedName, filename1)
	}

	// Verify file was written
	if _, err := os.Stat(path1); os.IsNotExist(err) {
		t.Errorf("saved file does not exist at: %s", path1)
	}

	// Save another file with the exact same details to trigger collision
	filename2, path2, err := mgr.SaveFile(data, "application/pdf", "", "João", "15-08-2026", "Luz Enel")
	if err != nil {
		t.Fatalf("failed to save duplicate file: %v", err)
	}

	expectedNameCol := "15-08-luz-enel-1.pdf"
	if filename2 != expectedNameCol {
		t.Errorf("expected collision name %s, got %s", expectedNameCol, filename2)
	}

	if path1 == path2 {
		t.Errorf("duplicate files should have different paths; both were: %s", path1)
	}

	// Verify second file exists
	if _, err := os.Stat(path2); os.IsNotExist(err) {
		t.Errorf("collision resolved file does not exist at: %s", path2)
	}
}
