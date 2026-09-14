package router

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
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

const wantedTestSecret = "wanted_test_secret"

// setupWantedRouter 基于内存 SQLite 装配求购路由，模拟真实 HTTP 链路。
func setupWantedRouter(t *testing.T) *gin.Engine {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
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
	RegisterWantedRoutes(api, h, wantedTestSecret)
	return r
}

func wantedToken(t *testing.T, userID uint, username string) string {
	t.Helper()
	token, err := util.GenerateToken(wantedTestSecret, userID, username, "user", time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return token
}

func doWantedRequest(t *testing.T, r *gin.Engine, method, path, token string, body interface{}) (int, map[string]interface{}) {
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
	alice := wantedToken(t, 1, "alice")
	bob := wantedToken(t, 2, "bob")

	// 未登录不能发布
	code, _ := doWantedRequest(t, r, http.MethodPost, "/api/v1/wanteds", "", wantedPayload())
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", code)
	}

	// 预算上下限颠倒不能生效
	code, _ = doWantedRequest(t, r, http.MethodPost, "/api/v1/wanteds", alice, map[string]interface{}{
		"title": "求购预算颠倒的需求", "category": "digital", "condition": "almost_new",
		"budget_min": 5000, "budget_max": 1000, "city": "深圳市", "description": "预算下限大于上限应被拒绝",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for inverted budget, got %d", code)
	}

	// 正常发布
	code, resp := doWantedRequest(t, r, http.MethodPost, "/api/v1/wanteds", alice, wantedPayload())
	if code != http.StatusOK {
		t.Fatalf("expected 200 for create, got %d resp=%v", code, resp)
	}
	data := resp["data"].(map[string]interface{})
	wantedID := data["id"].(float64)
	if data["status"].(string) != "open" {
		t.Fatalf("expected status open, got %v", data["status"])
	}

	// 大厅可见且最新在前
	code, resp = doWantedRequest(t, r, http.MethodGet, "/api/v1/wanteds?keyword=iPad&category=digital&city=深圳", "", nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200 for hall list, got %d", code)
	}
	list := resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 wanted in hall, got %d", len(list))
	}

	// 我的求购可见（验证 /wanteds/mine 未被 /wanteds/:id 吞掉）
	code, resp = doWantedRequest(t, r, http.MethodGet, "/api/v1/wanteds/mine", alice, nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200 for mine, got %d", code)
	}
	mine := resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(mine) != 1 {
		t.Fatalf("expected 1 wanted in mine, got %d", len(mine))
	}

	// 越权关闭不能生效
	path := "/api/v1/wanteds/" + strconv.FormatUint(uint64(wantedID), 10)
	code, _ = doWantedRequest(t, r, http.MethodPost, path+"/close", bob, nil)
	if code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner close, got %d", code)
	}
	_, resp = doWantedRequest(t, r, http.MethodGet, path, "", nil)
	if resp["data"].(map[string]interface{})["status"].(string) != "open" {
		t.Fatal("forbidden close should not take effect")
	}

	// 发布者关闭成功
	code, _ = doWantedRequest(t, r, http.MethodPost, path+"/close", alice, nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200 for owner close, got %d", code)
	}

	// 关闭后不再出现在大厅
	_, resp = doWantedRequest(t, r, http.MethodGet, "/api/v1/wanteds", "", nil)
	list = resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 0 {
		t.Fatalf("closed wanted should not appear in hall, got %d", len(list))
	}

	// 详情仍可查看
	code, resp = doWantedRequest(t, r, http.MethodGet, path, "", nil)
	if code != http.StatusOK || resp["data"].(map[string]interface{})["status"].(string) != "closed" {
		t.Fatalf("closed wanted detail should still be visible, code=%d", code)
	}

	// 重复关闭返回冲突
	code, _ = doWantedRequest(t, r, http.MethodPost, path+"/close", alice, nil)
	if code != http.StatusConflict {
		t.Fatalf("expected 409 for repeated close, got %d", code)
	}
}
