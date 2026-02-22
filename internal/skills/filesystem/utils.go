package filesystem

import (
	"path/filepath"
	"strings"
)

// ResolvePath: Rick'in verdiği yolu temizler ve sistem uyumlu hale getirir.
func ResolvePath(path string) string {
	// Rick bazen Windows'ta / kullanır, Go bunu filepath.Clean ile çözer.
	cleanPath := filepath.Clean(path)
	
	// Eğer Rick başa gereksiz ./ koyarsa temizle
	return strings.TrimPrefix(cleanPath, ".\\")
}