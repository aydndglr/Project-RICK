package memory

import (
	"context"
	"github.com/aydndglr/rick-agent/internal/brain"
	"github.com/aydndglr/rick-agent/pkg/logger"
)

type RAGEngine struct {
	DB    VectorStore
	Brain brain.LLMProvider // Embedding üretmek için kullanılacak
}

func NewRAGEngine(db VectorStore, b brain.LLMProvider) *RAGEngine {
	return &RAGEngine{DB: db, Brain: b}
}

// RememberRelevantContext: Görevle ilgili geçmiş bilgileri getirir
func (r *RAGEngine) RememberRelevantContext(ctx context.Context, task string) string {
	logger.Debug("🧠 Görevle ilgili geçmiş tecrübeler hatırlanıyor...")
	
	// 1. Task'i vektöre çevir (Embedding)
	// embedding, _ := r.Brain.GenerateEmbedding(ctx, task)
	
	// 2. Benzer dökümanları ara
	// docs, _ := r.DB.Search(ctx, embedding, 3)
	
	// 3. Bilgileri birleştirip context olarak dön
	return "Geçmiş Bilgi: Bu projenin 'Omni' modülleri (OmniPR, OmniMF) Go diliyle yazılmıştır." // Örnek context
}