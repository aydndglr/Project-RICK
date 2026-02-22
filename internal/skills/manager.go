package skills

import (
	"fmt"
	"sort"
	"sync"

	"github.com/aydndglr/rick-agent-v3/internal/core/kernel"
)

// Manager: Tüm yeteneklerin kayıt defteri.
type Manager struct {
	tools map[string]kernel.Tool
	mu    sync.RWMutex
}

// NewManager: Boş bir yönetici oluşturur.
func NewManager() *Manager {
	return &Manager{
		tools: make(map[string]kernel.Tool),
	}
}

// Register: Sisteme yeni bir araç ekler (Thread-safe).
func (m *Manager) Register(t kernel.Tool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Eğer aynı isimde varsa üzerine yazar (Update mantığı)
	m.tools[t.Name()] = t
}

// GetTool: İsmi verilen aracı bulur.
func (m *Manager) GetTool(name string) (kernel.Tool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if t, exists := m.tools[name]; exists {
		return t, nil
	}
	return nil, fmt.Errorf("araç bulunamadı: %s", name)
}

// ListTools: LLM'e göndermek için araç listesini (Definition) hazırlar.
func (m *Manager) ListTools() []kernel.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []kernel.Tool
	for _, t := range m.tools {
		list = append(list, t)
	}

	// İsim sırasına göre dizelim ki LLM kafası karışmasın
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name() < list[j].Name()
	})

	return list
}

// Count: Kaç tane araç olduğunu döner.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tools)
}