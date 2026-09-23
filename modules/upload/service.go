package upload

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"go-starter-kit/infrastructure/storage"
	"go-starter-kit/modules/primitive"

	"github.com/google/uuid"
)

type ServiceInterface interface {
	Upload(ctx context.Context, ownerID uuid.UUID, header *multipart.FileHeader) (primitive.UploadedFile, error)
	Get(ctx context.Context, id string) (primitive.UploadedFile, error)
}

type Service struct {
	repository RepositoryInterface
	storage    storage.Storage
	maxBytes   int64
}

func NewService(repository RepositoryInterface, storage storage.Storage, maxBytes int64) ServiceInterface {
	return &Service{repository: repository, storage: storage, maxBytes: maxBytes}
}

func (s *Service) Upload(ctx context.Context, ownerID uuid.UUID, header *multipart.FileHeader) (primitive.UploadedFile, error) {
	if header == nil {
		return primitive.UploadedFile{}, fmt.Errorf("file is required")
	}
	if s.maxBytes > 0 && header.Size > s.maxBytes {
		return primitive.UploadedFile{}, fmt.Errorf("file too large")
	}

	file, err := header.Open()
	if err != nil {
		return primitive.UploadedFile{}, err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, _ := io.ReadFull(file, buffer)
	contentType := http.DetectContentType(buffer[:n])
	if !allowedContentType(contentType) {
		return primitive.UploadedFile{}, fmt.Errorf("unsupported file type")
	}
	if seeker, ok := file.(io.Seeker); ok {
		_, _ = seeker.Seek(0, io.SeekStart)
	}

	ext := extensionForContentType(contentType)
	path, err := s.storage.Save(ctx, ownerID.String(), ext, file)
	if err != nil {
		return primitive.UploadedFile{}, err
	}

	return s.repository.Create(ctx, primitive.UploadedFile{
		OwnerID:     ownerID,
		Original:    filepath.Base(header.Filename),
		Path:        path,
		Size:        header.Size,
		ContentType: contentType,
	})
}

func extensionForContentType(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "application/pdf":
		return ".pdf"
	default:
		return ".bin"
	}
}

func (s *Service) Get(ctx context.Context, id string) (primitive.UploadedFile, error) {
	return s.repository.FindByID(ctx, id)
}

func allowedContentType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "application/pdf":
		return true
	default:
		return false
	}
}
