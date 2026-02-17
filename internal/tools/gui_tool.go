package tools

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/aydndglr/rick-agent/pkg/logger"
)

type GUITool struct {
	Workspace string
}

func NewGUITool(ws string) *GUITool {
	return &GUITool{Workspace: ws}
}

func (g *GUITool) Name() string {
	return "GUI Controller"
}

func (g *GUITool) Execute(ctx context.Context, action string, args map[string]interface{}) (string, error) {
	logger.Action("🎮 GUI Aksiyonu: %s", action)

	switch action {
	case "screenshot":
		return g.takeScreenshot()
	case "click":
		x, okX := args["x"].(float64)
		y, okY := args["y"].(float64)
		if !okX || !okY {
			return "", fmt.Errorf("geçersiz x veya y koordinatı")
		}
		return g.runPythonGUIScript(fmt.Sprintf("import pyautogui; pyautogui.click(%v, %v)", x, y))
	case "type":
		text, ok := args["text"].(string)
		if !ok {
			return "", fmt.Errorf("geçersiz metin parametresi")
		}
		return g.runPythonGUIScript(fmt.Sprintf("import pyautogui; pyautogui.write('%s', interval=0.1)", text))
	default:
		return "", fmt.Errorf("bilinmeyen GUI aksiyonu: %s", action)
	}
}

func (g *GUITool) takeScreenshot() (string, error) {
	path := filepath.Join(g.Workspace, "current_screen.png")
	script := fmt.Sprintf("import pyautogui; pyautogui.screenshot('%s')", path)
	_, err := g.runPythonGUIScript(script)
	if err != nil {
		return "", err
	}
	return path, nil
}

func (g *GUITool) runPythonGUIScript(script string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "python", "-c", script)
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("python gui hatası: %v", err)
	}
	return "✅ Başarılı", nil
}