package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-starter-kit/infrastructure/jwt"
	"go-starter-kit/infrastructure/middleware"
	"go-starter-kit/modules/primitive"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubService struct{ file primitive.UploadedFile }

func (s stubService) Upload(context.Context, uuid.UUID, *multipart.FileHeader) (primitive.UploadedFile, error) {
	return s.file, nil
}

func (s stubService) Get(context.Context, string) (primitive.UploadedFile, error) {
	return primitive.UploadedFile{}, nil
}

// The upload response must not carry the storage path. That value is an
// absolute server filesystem location (e.g. /srv/app/storage/<owner>/<id>.png),
// and returning it told every caller where the deployment lives and what the
// on-disk layout is.
func TestUploadResponseOmitsStoragePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokens := jwt.NewService(strings.Repeat("u", 32), time.Minute, time.Hour)
	pair, err := tokens.GeneratePair(uuid.New())
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	service := stubService{file: primitive.UploadedFile{
		ID:          uuid.New(),
		OwnerID:     uuid.New(),
		Original:    "evidence.png",
		Path:        "/srv/skill-issues-bot/storage/secret-owner/9f8e.png",
		Size:        208,
		ContentType: "image/png",
	}}

	r := gin.New()
	group := r.Group("/uploads")
	group.Use(func(c *gin.Context) {
		if claims, err := tokens.ParseAccessToken("Bearer " + pair.AccessToken); err == nil {
			c.Set(middleware.UserIDKey, claims.UserID)
		}
		c.Next()
	})
	NewHttp(service).GroupUpload(group)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "evidence.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("png-bytes")); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	raw := w.Body.String()
	if strings.Contains(raw, "storage") || strings.Contains(raw, "/srv/") || strings.Contains(raw, "secret-owner") {
		t.Fatalf("upload response leaks the storage path: %s", raw)
	}

	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, present := envelope.Data["path"]; present {
		t.Fatalf("response still exposes a path field: %s", raw)
	}
	// The caller still needs the id to reference the upload later.
	if envelope.Data["id"] == nil || envelope.Data["original"] != "evidence.png" {
		t.Fatalf("response lost identifying fields: %s", raw)
	}
}
