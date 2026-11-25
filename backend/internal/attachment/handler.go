package attachment

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
)

const maxUploadBytes = 25 << 20 // 25 MiB

// RegisterRoutes mounts multipart upload, list, and download endpoints on the chi router.
func RegisterRoutes(r chi.Router, repo Repository, uploadDir string) {
	r.Post("/api/v1/attachments", func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, maxUploadBytes)
		if err := req.ParseMultipartForm(maxUploadBytes); err != nil {
			http.Error(w, `{"detail":"file too large or invalid multipart form"}`, http.StatusBadRequest)
			return
		}

		entityType := req.FormValue("entity_type")
		if entityType != "test_case" && entityType != "test_execution" {
			http.Error(w, `{"detail":"entity_type must be test_case or test_execution"}`, http.StatusBadRequest)
			return
		}
		entityID, err := strconv.ParseInt(req.FormValue("entity_id"), 10, 64)
		if err != nil || entityID <= 0 {
			http.Error(w, `{"detail":"invalid entity_id"}`, http.StatusBadRequest)
			return
		}

		file, header, err := req.FormFile("file")
		if err != nil {
			http.Error(w, `{"detail":"missing file field"}`, http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()

		if err := os.MkdirAll(uploadDir, 0o755); err != nil {
			http.Error(w, `{"detail":"failed to prepare upload dir"}`, http.StatusInternalServerError)
			return
		}

		rnd := make([]byte, 8)
		_, _ = rand.Read(rnd)
		safeName := filepath.Base(header.Filename)
		storagePath := filepath.Join(uploadDir, hex.EncodeToString(rnd)+"_"+safeName)

		dst, err := os.Create(storagePath)
		if err != nil {
			http.Error(w, `{"detail":"failed to store file"}`, http.StatusInternalServerError)
			return
		}
		written, err := io.Copy(dst, file)
		_ = dst.Close()
		if err != nil {
			http.Error(w, `{"detail":"failed to write file"}`, http.StatusInternalServerError)
			return
		}

		ct := header.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/octet-stream"
		}

		saved, err := repo.Create(req.Context(), Attachment{
			EntityType:  entityType,
			EntityID:    entityID,
			Filename:    safeName,
			ContentType: ct,
			SizeBytes:   written,
			StoragePath: storagePath,
		})
		if err != nil {
			http.Error(w, `{"detail":"failed to save attachment metadata"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(saved)
	})

	r.Get("/api/v1/attachments", func(w http.ResponseWriter, req *http.Request) {
		entityType := req.URL.Query().Get("entity_type")
		entityID, err := strconv.ParseInt(req.URL.Query().Get("entity_id"), 10, 64)
		if (entityType != "test_case" && entityType != "test_execution") || err != nil || entityID <= 0 {
			http.Error(w, `{"detail":"invalid entity_type or entity_id"}`, http.StatusBadRequest)
			return
		}

		items, err := repo.ListByEntity(req.Context(), entityType, entityID)
		if err != nil {
			http.Error(w, `{"detail":"failed to list attachments"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	})

	r.Get("/api/v1/attachments/{id}/download", func(w http.ResponseWriter, req *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, `{"detail":"invalid attachment id"}`, http.StatusBadRequest)
			return
		}
		a, err := repo.GetByID(req.Context(), id)
		if err != nil {
			http.Error(w, `{"detail":"attachment not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", a.ContentType)
		w.Header().Set("Content-Disposition", `attachment; filename="`+a.Filename+`"`)
		http.ServeFile(w, req, a.StoragePath)
	})
}
