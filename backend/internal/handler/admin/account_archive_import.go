package admin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const importArchiveMaxBytes = 256 << 20 // 256MB

// ImportDataArchive imports accounts from a zip or gzip-compressed JSON archive.
// POST /api/v1/admin/accounts/data/archive
func (h *AccountHandler) ImportDataArchive(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	if fileHeader.Size <= 0 {
		response.BadRequest(c, "empty archive file")
		return
	}
	if fileHeader.Size > importArchiveMaxBytes {
		response.BadRequest(c, fmt.Sprintf("archive exceeds maximum size of %d bytes", importArchiveMaxBytes))
		return
	}

	fastImport := formBoolDefault(c, "fast_import", true)
	skipDefaultGroupBind := formBoolDefault(c, "skip_default_group_bind", true)
	defaultProxyID := formOptionalInt64(c, "default_proxy_id")
	defaultGroupIDs := formInt64Slice(c, "default_group_ids")

	opened, err := fileHeader.Open()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = opened.Close() }()

	body, err := io.ReadAll(io.LimitReader(opened, importArchiveMaxBytes+1))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if int64(len(body)) > importArchiveMaxBytes {
		response.BadRequest(c, fmt.Sprintf("archive exceeds maximum size of %d bytes", importArchiveMaxBytes))
		return
	}

	accounts, skipped, parseErr := parseArchiveFile(body, fileHeader.Filename)
	if parseErr != nil {
		response.BadRequest(c, parseErr.Error())
		return
	}

	req := DataImportRequest{
		Data: DataPayload{
			Proxies:  nil,
			Accounts: accounts,
		},
		SkipDefaultGroupBind: &skipDefaultGroupBind,
		FastImport:           &fastImport,
		DefaultProxyID:       defaultProxyID,
		DefaultGroupIDs:      defaultGroupIDs,
	}

	result, importErr := h.importData(c.Request.Context(), req)
	if len(skipped) > 0 {
		result.AccountSkipped += len(skipped)
		for _, item := range skipped {
			recordImportItem(&result, item)
		}
	}
	if importErr != nil {
		response.ErrorFrom(c, importErr)
		return
	}
	response.Success(c, result)
}

func parseArchiveFile(body []byte, filename string) ([]DataAccount, []DataImportItem, error) {
	lowerName := strings.ToLower(strings.TrimSpace(filename))
	switch {
	case strings.HasSuffix(lowerName, ".zip"):
		return parseZipArchive(body)
	case strings.HasSuffix(lowerName, ".gz") || strings.HasSuffix(lowerName, ".json.gz"):
		accounts, skipped, err := parseArchiveAccounts(bytes.NewReader(body), filename, true)
		return accounts, skipped, err
	default:
		return nil, nil, fmt.Errorf("unsupported archive type: %s (supported: .zip, .gz)", filepath.Ext(filename))
	}
}

func parseZipArchive(body []byte) ([]DataAccount, []DataImportItem, error) {
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, nil, fmt.Errorf("open zip archive: %w", err)
	}

	var accounts []DataAccount
	var skipped []DataImportItem

	for _, file := range reader.File {
		if file.FileInfo().IsDir() || shouldSkipArchiveEntry(file.Name) {
			continue
		}
		if file.UncompressedSize64 > 8<<20 {
			skipped = append(skipped, DataImportItem{
				Kind:    "account",
				Name:    file.Name,
				Action:  "skipped",
				Message: "json entry exceeds 8MB limit",
			})
			continue
		}

		rc, err := file.Open()
		if err != nil {
			skipped = append(skipped, DataImportItem{
				Kind:    "account",
				Name:    file.Name,
				Action:  "skipped",
				Message: "open zip entry failed: " + err.Error(),
			})
			continue
		}
		entryBody, err := io.ReadAll(io.LimitReader(rc, 8<<20+1))
		_ = rc.Close()
		if err != nil {
			skipped = append(skipped, DataImportItem{
				Kind:    "account",
				Name:    file.Name,
				Action:  "skipped",
				Message: "read zip entry failed: " + err.Error(),
			})
			continue
		}

		parsed := parseImportJSONBytes(entryBody, file.Name)
		accounts = append(accounts, parsed.accounts...)
		skipped = append(skipped, parsed.skipped...)
	}

	if len(accounts) == 0 && len(skipped) == 0 {
		return nil, nil, fmt.Errorf("zip archive contains no importable .json account files")
	}
	return accounts, skipped, nil
}

func formBoolDefault(c *gin.Context, key string, defaultValue bool) bool {
	raw := strings.TrimSpace(c.PostForm(key))
	if raw == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func formOptionalInt64(c *gin.Context, key string) *int64 {
	raw := strings.TrimSpace(c.PostForm(key))
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return nil
	}
	return &parsed
}

func formInt64Slice(c *gin.Context, key string) []int64 {
	raw := strings.TrimSpace(c.PostForm(key))
	if raw == "" {
		return nil
	}
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}
