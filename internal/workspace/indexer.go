package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aydndglr/rick-agent/pkg/logger"
)

// ProjectMap: Projenin hiyerarşik yapısını ve dosya özetlerini tutar
type ProjectMap struct {
	RootPath string
	Tree     string
	Files    []string
}

// Indexer: Proje dizinini analiz eden motor
type Indexer struct {
	RootPath   string
	IgnoreList []string
}

func NewIndexer(root string) *Indexer {
	return &Indexer{
		RootPath: root,
		IgnoreList: []string{
			".git", "node_modules", "dist", "bin", "obj", ".exe", ".log", "store.db",
		},
	}
}

// GenerateFullMap: Tüm projeyi tarar ve modelin anlayacağı bir metin özeti döner
func (idx *Indexer) GenerateFullMap() (*ProjectMap, error) {
	logger.Debug("📂 Proje haritası çıkarılıyor: %s", idx.RootPath)
	
	var tree strings.Builder
	var files []string

	err := filepath.Walk(idx.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignore list kontrolü
		for _, ignore := range idx.IgnoreList {
			if strings.Contains(path, ignore) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Göreli yolu hesapla
		relPath, _ := filepath.Rel(idx.RootPath, path)
		if relPath == "." {
			return nil
		}

		// Görsel ağaç yapısı oluştur
		indent := strings.Repeat("  ", strings.Count(relPath, string(os.PathSeparator)))
		icon := "📄"
		if info.IsDir() {
			icon = "📁"
		} else {
			files = append(files, relPath)
		}

		tree.WriteString(fmt.Sprintf("%s%s %s\n", indent, icon, info.Name()))
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &ProjectMap{
		RootPath: idx.RootPath,
		Tree:     tree.String(),
		Files:    files,
	}, nil
}