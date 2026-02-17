package agent

import (
	"context"
	"github.com/aydndglr/rick-agent/pkg/logger"
)

type Orchestrator struct {
	Engineer *Agent // Kod yazan Rick
	Reviewer *Agent // Kodu kontrol eden/test eden yardımcı
}

func (o *Orchestrator) SolveIssue(ctx context.Context, issue string) {
	logger.Info("🎭 Çoklu Ajan Döngüsü Başlatıldı")

	// 1. Rick kodu yazar
	// o.Engineer.Run(ctx, issue)

	// 2. Morty testleri çalıştırır
	// testResult := o.Reviewer.Run(ctx, "Yeni yazılan kodları test et ve raporla")

	// 3. Eğer hata varsa Rick'e geri gönder (Self-Correction)
	logger.Debug("🔄 Ajanlar arası geri bildirim döngüsü çalışıyor...")
}