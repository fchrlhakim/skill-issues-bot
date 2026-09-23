package httplib

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetPaginationFromCtxWhitelistsSortColumn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestURL := "/?orderBy=" + url.QueryEscape("id;drop table users") + "&sortOrder=asc&size=999"
	req := httptest.NewRequest("GET", requestURL, nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	query := GetPaginationFromCtx(c, "created_at", map[string]string{"name": "name"})
	if query.SortBy != "created_at" {
		t.Fatalf("SortBy = %q, want default", query.SortBy)
	}
	if query.Size != 100 {
		t.Fatalf("Size = %d, want capped 100", query.Size)
	}
	if query.SortOrder != "ASC" {
		t.Fatalf("SortOrder = %q, want ASC", query.SortOrder)
	}
}
