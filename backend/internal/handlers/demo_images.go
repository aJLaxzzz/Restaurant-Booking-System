package handlers

import "fmt"

// Локальные ассеты из frontend/public/demo/ (отдаётся тем же origin, что и SPA).
const (
	demoPhotoTrattoria = "/demo/restaurants/trattoria.svg"
	demoPhotoLaLuna    = "/demo/restaurants/la-luna.svg"
	demoPhotoSakura    = "/demo/restaurants/sakura.svg"
	demoPhotoBella     = "/demo/restaurants/bella-vista.svg"
)

func demoDishImageAt(i int) string {
	n := (i % 14) + 1
	return fmt.Sprintf("/demo/dishes/dish-%02d.svg", n)
}
