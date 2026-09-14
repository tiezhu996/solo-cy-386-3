package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/marketpal/marketpal/internal/handler"
	"github.com/marketpal/marketpal/internal/middleware"
	"github.com/marketpal/marketpal/internal/model"
	"github.com/marketpal/marketpal/internal/repository"
	"github.com/marketpal/marketpal/internal/service"
	"github.com/marketpal/marketpal/internal/util"
	"gorm.io/gorm"
)

const testJWTSecret = "wanted_test_secret"

// setupWantedRouterWithDSN 基于指定 SQLite DSN 装配求购路由，模拟真实 HTTP 链路。
func setupWantedRouterWithDSN(t *testing.T, dsn string) *gin.Engine {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Wanted{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewWantedService(repository.NewWantedRepository(db), logger)
	h := handler.NewWantedHandler(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler(logger))
	api := r.Group("/api/v1")
	RegisterWantedRoutes(api, h, testJWTSecret)
	return r
}

func setupWantedRouter(t *testing.T) *gin.Engine {
	t.Helper()
	return setupWantedRouterWithDSN(t, "file::memory:")
}

func testToken(t *testing.T, userID uint, username string) string {
	t.Helper()
	token, err := util.GenerateToken(testJWTSecret, userID, username, "user", time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return token
}

func doAPIRequest(t *testing.T, r *gin.Engine, method, path, token string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp
}

func wantedPayload() map[string]interface{} {
	return map[string]interface{}{
		"title":       "求购一台 iPad Pro",
		"category":    "digital",
		"condition":   "almost_new",
		"budget_min":  2000,
		"budget_max":  4000,
		"city":        "深圳市",
		"description": "希望配件齐全，成色好一点",
	}
}

func TestWantedHTTPFlow(t *testing.T) {
	r := setupWantedRouter(t)
	alice := testToken(t, 1, "alice")
	bob := testToken(t, 2, "bob")

	// 未登录不能发布
	code, _ := doAPIRequest(t, r, http.MethodPost, "/api/v1/wanteds", "", wantedPayload())
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", code)
	}

	// 预算上下限颠倒不能生效
	code, _ = doAPIRequest(t, r, http.MethodPost, "/api/v1/wanteds", alice, map[string]interface{}{
		"title": "求购预算颠倒的需求", "category": "digital", "condition": "almost_new",
		"budget_min": 5000, "budget_max": 1000, "city": "深圳市", "description": "预算下限大于上限应被拒绝",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for inverted budget, got %d", code)
	}

	// 预算下限缺失或为空必须被拒绝（必填字段存在性校验）
	code, _ = doAPIRequest(t, r, http.MethodPost, "/api/v1/wanteds", alice, map[string]interface{}{
		"title": "求购缺少预算下限的需求", "category": "digital", "condition": "almost_new",
		"budget_max": 1000, "city": "深圳市", "description": "缺少 budget_min 字段应被拒绝",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing budget_min, got %d", code)
	}
	code, _ = doAPIRequest(t, r, http.MethodPost, "/api/v1/wanteds", alice, map[string]interface{}{
		"title": "求购预算下限为空的需求", "category": "digital", "condition": "almost_new",
		"budget_min": nil, "budget_max": 1000, "city": "深圳市", "description": "budget_min 为 null 应被拒绝",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for null budget_min, got %d", code)
	}

	// 预算下限为零可以正常创建
	code, resp := doAPIRequest(t, r, http.MethodPost, "/api/v1/wanteds", alice, map[string]interface{}{
		"title": "求购任意价位的旧书", "category": "books", "condition": "lightly_used",
		"budget_min": 0, "budget_max": 50, "city": "北京市", "description": "预算从零开始，成色不限",
	})
	if code != http.StatusOK {
		t.Fatalf("expected 200 for zero budget_min, got %d resp=%v", code, resp)
	}
	if resp["data"].(map[string]interface{})["budget_min"].(float64) != 0 {
		t.Fatalf("expected budget_min 0 persisted, got %v", resp["data"])
	}

	// 正常发布
	code, resp = doAPIRequest(t, r, http.MethodPost, "/api/v1/wanteds", alice, wantedPayload())
	if code != http.StatusOK {
		t.Fatalf("expected 200 for create, got %d resp=%v", code, resp)
	}
	data := resp["data"].(map[string]interface{})
	wantedID := data["id"].(float64)
	if data["status"].(string) != "open" {
		t.Fatalf("expected status open, got %v", data["status"])
	}

	// 大厅可见且能通过筛选命中
	code, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/wanteds?keyword=iPad&category=digital&city=深圳", "", nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200 for hall list, got %d", code)
	}
	list := resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 wanted in hall, got %d", len(list))
	}

	// 我的求购可见（验证 /wanteds/mine 未被 /wanteds/:id 吞掉）
	code, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/wanteds/mine", alice, nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200 for mine, got %d", code)
	}
	mine := resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(mine) != 2 {
		t.Fatalf("expected 2 wanteds in mine, got %d", len(mine))
	}

	path := "/api/v1/wanteds/" + strconv.FormatUint(uint64(wantedID), 10)

	// 越权关闭不能生效
	code, _ = doAPIRequest(t, r, http.MethodPost, path+"/close", bob, nil)
	if code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner close, got %d", code)
	}
	_, resp = doAPIRequest(t, r, http.MethodGet, path, "", nil)
	if resp["data"].(map[string]interface{})["status"].(string) != "open" {
		t.Fatal("forbidden close should not take effect")
	}

	// 发布者关闭成功
	code, _ = doAPIRequest(t, r, http.MethodPost, path+"/close", alice, nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200 for owner close, got %d", code)
	}

	// 关闭后不再出现在大厅（深圳剩 0 条，全部大厅只剩零预算旧书 1 条）
	_, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/wanteds?city=深圳", "", nil)
	list = resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 0 {
		t.Fatalf("closed wanted should not appear in hall, got %d", len(list))
	}

	// 详情仍可查看
	code, resp = doAPIRequest(t, r, http.MethodGet, path, "", nil)
	if code != http.StatusOK || resp["data"].(map[string]interface{})["status"].(string) != "closed" {
		t.Fatalf("closed wanted detail should still be visible, code=%d", code)
	}

	// 重复关闭返回冲突
	code, _ = doAPIRequest(t, r, http.MethodPost, path+"/close", alice, nil)
	if code != http.StatusConflict {
		t.Fatalf("expected 409 for repeated close, got %d", code)
	}
}

// TestWantedConcurrentClose 并发关闭同一求购：只允许一个请求成功，其余全部 409，最终状态一致为 closed。
func TestWantedConcurrentClose(t *testing.T) {
	// 文件库 + busy_timeout：多连接并发写时 SQLite 串行化写锁，行为与 PostgreSQL 行锁一致。
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", filepath.Join(t.TempDir(), "close_test.db"))
	r := setupWantedRouterWithDSN(t, dsn)
	alice := testToken(t, 1, "alice")

	code, resp := doAPIRequest(t, r, http.MethodPost, "/api/v1/wanteds", alice, wantedPayload())
	if code != http.StatusOK {
		t.Fatalf("create failed: %d", code)
	}
	wantedID := resp["data"].(map[string]interface{})["id"].(float64)
	path := "/api/v1/wanteds/" + strconv.FormatUint(uint64(wantedID), 10) + "/close"

	const n = 8
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c, _ := doAPIRequest(t, r, http.MethodPost, path, alice, nil)
			codes[idx] = c
		}(i)
	}
	wg.Wait()

	okCount, conflictCount := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			okCount++
		case http.StatusConflict:
			conflictCount++
		default:
			t.Fatalf("unexpected status %d in concurrent close", c)
		}
	}
	if okCount != 1 || conflictCount != n-1 {
		t.Fatalf("expected exactly 1 success and %d conflicts, got %d success %d conflict", n-1, okCount, conflictCount)
	}

	// 最终状态一致：详情 closed，大厅不可见
	_, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/wanteds/"+strconv.FormatUint(uint64(wantedID), 10), "", nil)
	if resp["data"].(map[string]interface{})["status"].(string) != "closed" {
		t.Fatalf("final status should be closed, got %v", resp["data"])
	}
	_, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/wanteds", "", nil)
	if len(resp["data"].(map[string]interface{})["list"].([]interface{})) != 0 {
		t.Fatal("closed wanted should not appear in hall after concurrent close")
	}
}
