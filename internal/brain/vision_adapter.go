package brain

import (
	"encoding/base64"
	"os"
)

// EncodeImageToBase64: Ekran görüntüsünü modelin anlayacağı formata çevirir
func EncodeImageToBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// TODO: Predict metodunu, içinde görüntü verisi (image_url) taşıyabilecek şekilde 
// Multi-modal hale getireceğiz.