package attachment

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/storage"
)

const maxUploadBytes = 25 << 20 // 25 MiB

// RegisterRoutes mounts multipart upload, list, download, and delete endpoints on the chi router.
func RegisterRoutes(r chi.Router, repo Repository, store storage.Storage, guard *auth.AccessGuard) {
	r.With(auth.RequireRole("tester")).Post("/api/v1/attachments", func(w http.ResponseWriter, req *http.Request) {
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
		if auth.WriteAccessError(w, guard.EnforceEntity(req.Context(), entityType, entityID, true)) {
			return
		}
		exists, err := repo.EntityExists(req.Context(), entityType, entityID)
		if err != nil {
			http.Error(w, `{"detail":"failed to verify target entity"}`, http.StatusInternalServerError)
			return
		}
		if !exists {
			http.Error(w, `{"detail":"target entity not found"}`, http.StatusNotFound)
			return
		}

		file, header, err := req.FormFile("file")
		if err != nil {
			http.Error(w, `{"detail":"missing file field"}`, http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()

		rnd := make([]byte, 8)
		_, _ = rand.Read(rnd)
		safeName := filepath.Base(header.Filename)
		if safeName == "." || safeName == "/" || safeName == "" {
			safeName = "attachment"
		}
		key := hex.EncodeToString(rnd) + "_" + safeName

		// Peek at first bytes to detect content type before storing.
		peek := make([]byte, 512)
		n, _ := file.Read(peek)
		ct := "application/octet-stream"
		if n > 0 {
			ct = http.DetectContentType(peek[:n])
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			http.Error(w, `{"detail":"failed to read file"}`, http.StatusInternalServerError)
			return
		}

		storagePath, err := store.Put(req.Context(), key, file, header.Size)
		if err != nil {
			http.Error(w, `{"detail":"failed to store file"}`, http.StatusInternalServerError)
			return
		}

		saved, err := repo.Create(req.Context(), Attachment{
			EntityType:  entityType,
			EntityID:    entityID,
			Filename:    safeName,
			ContentType: ct,
			SizeBytes:   header.Size,
			StoragePath: storagePath,
		})
		if err != nil {
			_ = store.Delete(req.Context(), storagePath)
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
		if auth.WriteAccessError(w, guard.EnforceEntity(req.Context(), entityType, entityID, false)) {
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
		if auth.WriteAccessError(w, guard.EnforceEntity(req.Context(), a.EntityType, a.EntityID, false)) {
			return
		}
		rc, err := store.Open(req.Context(), a.StoragePath)
		if err != nil {
			http.Error(w, `{"detail":"failed to open attachment"}`, http.StatusInternalServerError)
			return
		}
		defer func() { _ = rc.Close() }()
		w.Header().Set("Content-Type", a.ContentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "sandbox")
		disp := mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(a.Filename)})
		if disp == "" {
			disp = "attachment"
		}
		w.Header().Set("Content-Disposition", disp)
		_, _ = io.Copy(w, rc)
	})

	r.With(auth.RequireRole("tester")).Delete("/api/v1/attachments/{id}", func(w http.ResponseWriter, req *http.Request) {
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
		if auth.WriteAccessError(w, guard.EnforceEntity(req.Context(), a.EntityType, a.EntityID, true)) {
			return
		}
		if err := repo.Delete(req.Context(), id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, `{"detail":"attachment not found"}`, http.StatusNotFound)
				return
			}
			http.Error(w, `{"detail":"failed to delete attachment"}`, http.StatusInternalServerError)
			return
		}
		_ = store.Delete(req.Context(), a.StoragePath)
		w.WriteHeader(http.StatusNoContent)
	})
}
