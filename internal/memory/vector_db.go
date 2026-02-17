package memory

import (
	"context"
	"encoding/json"

	"math"
	"os"
	"sort"
	"sync"

	"github.com/aydndglr/rick-agent/pkg/logger"
	"github.com/google/uuid"
)

// Document: Bellekte tutulan bir bilgi parçası
type Document struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	Embedding []float32              `json:"embedding"`
}

// ScoredDocument: Arama sonucu ve benzerlik puanı
type ScoredDocument struct {
	Document
	Score float64
}

// VectorStore: Rick'in uzun süreli belleğinin arayüzü
type VectorStore interface {
	Add(ctx context.Context, doc Document) error
	Search(ctx context.Context, queryEmbedding []float32, topK int) ([]Document, error)
	Load() error
}

// LocalVectorDB: Disk tabanlı (JSON persistence) ve Cosine Similarity destekli vektör deposu.
// Harici veritabanı (Qdrant/Milvus) gerektirmez, küçük/orta ölçekli projeler için mükemmeldir.
type LocalVectorDB struct {
	filePath string
	docs     []Document
	mu       sync.RWMutex // Eşzamanlı erişim koruması
}

func NewLocalVectorDB(storagePath string) *LocalVectorDB {
	db := &LocalVectorDB{
		filePath: storagePath,
		docs:     []Document{},
	}
	// Başlangıçta varsa eski verileri yükle
	if err := db.Load(); err != nil {
		logger.Warn("⚠️ Bellek dosyası yüklenemedi veya boş: %v", err)
	}
	return db
}

func (db *LocalVectorDB) Add(ctx context.Context, doc Document) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}
	
	db.docs = append(db.docs, doc)
	
	// Her eklemede diske kaydet (Crash durumunda veri kaybını önler)
	return db.saveToDisk()
}

// Search: Gerçek Kosinüs Benzerliği (Cosine Similarity) kullanarak arama yapar.
func (db *LocalVectorDB) Search(ctx context.Context, query []float32, topK int) ([]Document, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if len(db.docs) == 0 {
		return []Document{}, nil
	}

	logger.Info("🔍 Anlamsal bellek taranıyor (%d döküman)...", len(db.docs))

	var results []ScoredDocument

	// Tüm dökümanlarla karşılaştır (Brute-force vector search)
	// Not: Binlerce döküman için bu yöntem hızlıdır, milyonlar için HNSW (Qdrant) gerekir.
	for _, doc := range db.docs {
		score := cosineSimilarity(query, doc.Embedding)
		if score > 0.4 { // Eşik değer (Çok alakasızları ele)
			results = append(results, ScoredDocument{Document: doc, Score: score})
		}
	}

	// Puana göre sırala (En yüksek puan en üstte)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// TopK kadarını al
	if len(results) > topK {
		results = results[:topK]
	}

	// Sadece Document kısmını döndür
	finalDocs := make([]Document, len(results))
	for i, r := range results {
		finalDocs[i] = r.Document
		logger.Debug("   🔹 Bulunan: %.4f puan - %s...", r.Score, r.Content[:min(30, len(r.Content))])
	}

	return finalDocs, nil
}

// Persistence: Verileri diske kaydet
func (db *LocalVectorDB) saveToDisk() error {
	data, err := json.MarshalIndent(db.docs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(db.filePath, data, 0644)
}

// Persistence: Verileri diskten oku
func (db *LocalVectorDB) Load() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, err := os.Stat(db.filePath); os.IsNotExist(err) {
		return nil // Dosya yoksa sorun yok, sıfırdan başlarız
	}

	data, err := os.ReadFile(db.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &db.docs)
}

// --- Matematiksel Yardımcı Fonksiyonlar ---

// cosineSimilarity: İki vektör arasındaki açıyı hesaplar (1.0 = Aynı, -1.0 = Zıt)
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}