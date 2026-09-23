package httplib

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Query struct {
	Page      int
	Size      int
	Offset    int
	SortBy    string
	SortOrder string
}

type CursorQuery struct {
	Limit     int
	Cursor    string
	CreatedAt *time.Time
	ID        string
}

func GetCursorPaginationFromCtx(c *gin.Context) (CursorQuery, error) {
	limit := positiveInt(c.Query("limit"), 20)
	if limit > 100 {
		limit = 100
	}
	query := CursorQuery{Limit: limit, Cursor: c.Query("cursor")}
	if query.Cursor == "" {
		return query, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(query.Cursor)
	if err != nil {
		return CursorQuery{}, err
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return CursorQuery{}, fmt.Errorf("invalid cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return CursorQuery{}, err
	}
	query.CreatedAt = &createdAt
	query.ID = parts[1]
	return query, nil
}

func EncodeCursor(createdAt time.Time, id string) string {
	value := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func GetPaginationFromCtx(c *gin.Context, defaultSort string, allowedSorts map[string]string) Query {
	page := positiveInt(c.Query("page"), 1)
	size := positiveInt(c.Query("size"), 10)
	if size > 100 {
		size = 100
	}

	sortBy := defaultSort
	if column, ok := allowedSorts[c.Query("orderBy")]; ok {
		sortBy = column
	}

	sortOrder := "DESC"
	if strings.EqualFold(c.Query("sortOrder"), "asc") {
		sortOrder = "ASC"
	}

	return Query{Page: page, Size: size, Offset: (page - 1) * size, SortBy: sortBy, SortOrder: sortOrder}
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
