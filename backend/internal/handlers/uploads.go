package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const uploadMaxBytes = 5 << 20 // 5 MiB

var uploadAllowedTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func (a *Handlers) ensureUploadDir() error {
	return os.MkdirAll(a.Cfg.UploadDir, 0o755)
}

func (a *Handlers) saveUploadFile(r *http.Request, field string) (publicPath string, err error) {
	if err := r.ParseMultipartForm(uploadMaxBytes + 1024); err != nil {
		return "", err
	}
	return a.saveMultipartField(r, field)
}

// saveMultipartField сохраняет поле multipart после вызова ParseMultipartForm (или из saveUploadFile).
func (a *Handlers) saveMultipartField(r *http.Request, field string) (publicPath string, err error) {
	if err := a.ensureUploadDir(); err != nil {
		return "", err
	}
	f, hdr, err := r.FormFile(field)
	if err != nil {
		return "", err
	}
	defer f.Close()
	ct := hdr.Header.Get("Content-Type")
	ext, ok := uploadAllowedTypes[ct]
	if !ok {
		switch strings.ToLower(strings.TrimPrefix(filepath.Ext(hdr.Filename), ".")) {
		case "jpg", "jpeg":
			ext, ok = ".jpg", true
		case "png":
			ext, ok = ".png", true
		case "webp":
			ext, ok = ".webp", true
		}
	}
	if !ok {
		return "", fmt.Errorf("тип файла: jpeg, png или webp")
	}
	if hdr.Size > uploadMaxBytes {
		return "", fmt.Errorf("файл больше 5 МБ")
	}
	name := uuid.New().String() + ext
	full := filepath.Join(a.Cfg.UploadDir, name)
	out, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, f); err != nil {
		_ = os.Remove(full)
		return "", err
	}
	return "/api/files/" + name, nil
}

func (a *Handlers) handleUploadedFile(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/files/")
	name = filepath.Clean(name)
	if name == "." || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(a.Cfg.UploadDir, name)
	absUpload, _ := filepath.Abs(a.Cfg.UploadDir)
	absFull, _ := filepath.Abs(full)
	if !strings.HasPrefix(absFull, absUpload+string(os.PathSeparator)) && absFull != absUpload {
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(absFull); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, absFull)
}
