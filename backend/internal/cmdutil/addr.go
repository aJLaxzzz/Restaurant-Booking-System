package cmdutil

import "os"

// ListenAddr возвращает ADDR из окружения или значение по умолчанию (например ":8081").
func ListenAddr(defaultAddr string) string {
	if a := os.Getenv("ADDR"); a != "" {
		return a
	}
	return defaultAddr
}
