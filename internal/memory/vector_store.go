package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/aydndglr/rick-agent-v3/internal/core/kernel"
	"github.com/aydndglr/rick-agent-v3/internal/core/logger"
	"github.com/google/uuid"
)

// Document: Hafızadaki tekil bilgi birimi
type Document struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	Embedding []float32              `json:"embedding"`
	CreatedAt time.Time              `json:"created_at"`
}

// VectorStore: Basit, yerel vektör veritabanı
type VectorStore struct {
	FilePath string
	Brain    kernel.Brain // Embedding üretmek için
	docs     []Document
	mu       sync.RWMutex
}

func NewVectorStore(path string, brain kernel.Brain) *VectorStore {
	store := &VectorStore{
		FilePath: path,
		Brain:    brain,
		docs:     []Document{},
	}
	store.load() // Başlarken yükle
	return store
}

// Add: Hafızaya yeni bilgi ekler
func (vs *VectorStore) Add(ctx context.Context, content string, metadata map[string]interface{}) error {
	// 1. Embedding üret
	vector, err := vs.Brain.Embed(ctx, content)
	if err != nil {
		return fmt.Errorf("embedding hatası: %v", err)
	}

	doc := Document{
		ID:        uuid.New().String(),
		Content:   content,
		Metadata:  metadata,
		Embedding: vector,
		CreatedAt: time.Now(),
	}

	vs.mu.Lock()
	vs.docs = append(vs.docs, doc)
	vs.mu.Unlock()

	// Diske kaydet
	return vs.save()
}

// Search: Anlamsal arama yapar
func (vs *VectorStore) Search(ctx context.Context, query string, limit int) ([]string, error) {
	// 1. Sorgunun vektörünü al
	queryVector, err := vs.Brain.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	vs.mu.RLock()
	defer vs.mu.RUnlock()

	type result struct {
		doc   Document
		score float64
	}

	var results []result

	// 2. Benzerlik hesapla (Cosine Similarity)
	for _, doc := range vs.docs {
		score := cosineSimilarity(queryVector, doc.Embedding)
		if score > 0.4 { // Eşik değer (Çok alakasızları ele)
			results = append(results, result{doc, score})
		}
	}

	// 3. Sırala (En yüksek puan en üstte)
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// 4. Sonuçları hazırla
	var contents []string
	for i := 0; i < limit && i < len(results); i++ {
		// Metadata'yı da ekle ki bağlam kopmasın
		metaStr := ""
		if val, ok := results[i].doc.Metadata["source"]; ok {
			metaStr = fmt.Sprintf("[%v] ", val)
		}
		contents = append(contents, fmt.Sprintf("%s%s", metaStr, results[i].doc.Content))
	}

	return contents, nil
}

// -- Persistence (Disk İşlemleri) --

func (vs *VectorStore) save() error {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	data, err := json.MarshalIndent(vs.docs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(vs.FilePath, data, 0644)
}

func (vs *VectorStore) load() {
	if _, err := os.Stat(vs.FilePath); os.IsNotExist(err) {
		return
	}

	data, err := os.ReadFile(vs.FilePath)
	if err != nil {
		logger.Warn("Hafıza dosyası okunamadı: %v", err)
		return
	}

	json.Unmarshal(data, &vs.docs)
	logger.Info("🧠 Hafıza yüklendi: %d kayıt", len(vs.docs))
}

// -- Math Helpers --

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}
	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}
	if normA == 0 || normB == 0 {
		return 0.0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}