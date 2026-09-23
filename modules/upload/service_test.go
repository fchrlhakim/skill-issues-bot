package upload

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"go-starter-kit/modules/primitive"

	"github.com/google/uuid"
)

type stubUploadRepository struct {
	createdFile primitive.UploadedFile
}

func (r *stubUploadRepository) Create(ctx context.Context, file primitive.UploadedFile) (primitive.UploadedFile, error) {
	r.createdFile = file
	file.ID = uuid.New()
	return file, nil
}

func (r *stubUploadRepository) FindByID(ctx context.Context, id string) (primitive.UploadedFile, error) {
	return primitive.UploadedFile{ID: uuid.MustParse(id)}, nil
}

type stubStorage struct {
	directory string
	extension string
	content   []byte
}

func (s *stubStorage) Save(ctx context.Context, directory, extension string, reader io.Reader) (string, error) {
	s.directory = directory
	s.extension = extension
	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	s.content = content
	return directory + "/stored" + extension, nil
}

var _ RepositoryInterface = (*stubUploadRepository)(nil)

func TestUploadRejectsUnsupportedContentType(t *testing.T) {
	repository := &stubUploadRepository{}
	storage := &stubStorage{}
	service := NewService(repository, storage, 1024)
	header := multipartHeader(t, "note.txt", []byte("plain text file"))

	_, err := service.Upload(context.Background(), uuid.New(), header)

	if err == nil || err.Error() != "unsupported file type" {
		t.Fatalf("expected unsupported file type error, got %v", err)
	}
	if storage.content != nil {
		t.Fatalf("expected unsupported file not to be stored")
	}
}

func TestUploadStoresAllowedFileAndPersistsMetadata(t *testing.T) {
	repository := &stubUploadRepository{}
	storage := &stubStorage{}
	service := NewService(repository, storage, 1024)
	ownerID := uuid.New()
	png := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, bytes.Repeat([]byte{0}, 32)...)
	header := multipartHeader(t, "avatar.png", png)

	file, err := service.Upload(context.Background(), ownerID, header)

	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if storage.directory != ownerID.String() {
		t.Fatalf("expected owner directory %q, got %q", ownerID.String(), storage.directory)
	}
	if storage.extension != ".png" {
		t.Fatalf("expected .png extension, got %q", storage.extension)
	}
	if !bytes.Equal(storage.content, png) {
		t.Fatalf("expected stored content to match uploaded file")
	}
	if repository.createdFile.Original != "avatar.png" {
		t.Fatalf("expected original filename to be persisted, got %q", repository.createdFile.Original)
	}
	if repository.createdFile.ContentType != "image/png" {
		t.Fatalf("expected image/png metadata, got %q", repository.createdFile.ContentType)
	}
	if file.Path != ownerID.String()+"/stored.png" {
		t.Fatalf("expected returned storage path, got %q", file.Path)
	}
}

func TestUploadUsesDetectedContentTypeExtension(t *testing.T) {
	repository := &stubUploadRepository{}
	storage := &stubStorage{}
	service := NewService(repository, storage, 1024)
	ownerID := uuid.New()
	png := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, bytes.Repeat([]byte{0}, 32)...)
	header := multipartHeader(t, "avatar.bin", png)

	_, err := service.Upload(context.Background(), ownerID, header)

	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if storage.extension != ".png" {
		t.Fatalf("expected extension from detected content type, got %q", storage.extension)
	}
	if repository.createdFile.Original != "avatar.bin" {
		t.Fatalf("expected original filename to be preserved in metadata, got %q", repository.createdFile.Original)
	}
}

func TestUploadRejectsFilesOverLimit(t *testing.T) {
	repository := &stubUploadRepository{}
	storage := &stubStorage{}
	service := NewService(repository, storage, 4)
	png := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, bytes.Repeat([]byte{0}, 32)...)
	header := multipartHeader(t, "avatar.png", png)

	_, err := service.Upload(context.Background(), uuid.New(), header)

	if err == nil || err.Error() != "file too large" {
		t.Fatalf("expected file too large error, got %v", err)
	}
	if storage.content != nil {
		t.Fatalf("expected oversized file not to be stored")
	}
}

func multipartHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, "/uploads", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(int64(body.Len())); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	_, header, err := request.FormFile("file")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	return header
}
