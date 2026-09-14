package router

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/marketpal/marketpal/internal/handler"
	"github.com/marketpal/marketpal/internal/middleware"
	"github.com/marketpal/marketpal/internal/model"
	"github.com/marketpal/marketpal/internal/repository"
	"github.com/marketpal/marketpal/internal/service"
	"gorm.io/gorm"
)

// wanted_scenarios_test.go 求购模块端到端场景测试（可重复运行）。
// 每条用例独立装配路由与临时文件数据库：数据各自准备、用例结束自动清理
// （t.TempDir 目录由 testing 框架删除，数据库句柄随 t.Cleanup 关闭），
// 用例间互不影响，go test -count=N 重复执行结果稳定。

// newScenarioDB 为单条用例创建独立的临时文件 SQLite 数据库。
// busy_timeout 保证并发写时串行等待，行为与 PostgreSQL 行锁一致。
func newScenarioDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", filepath.Join(t.TempDir(), "scenario.db"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// newScenarioEngine 装配求购 + 私信路由的独立环境（每条用例一套，数据互不影响）。
func newScenarioEngine(t *testing.T) *gin.Engine {
	t.Helper()
	db := newScenarioDB(t, &model.Wanted{}, &model.User{}, &model.Product{}, &model.Message{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wantedSvc := service.NewWantedService(repository.NewWantedRepository(db), logger)
	hub := service.NewHub(nil, logger)
	messageSvc := service.NewMessageService(repository.NewMessageRepository(db), hub, logger)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler(logger))
	api := r.Group("/api/v1")
	RegisterWantedRoutes(api, handler.NewWantedHandler(wantedSvc), testJWTSecret)
	RegisterMessageRoutes(api, handler.NewMessageHandler(messageSvc), handler.NewWSHandler(hub, logger), testJWTSecret)
	return r
}

// hallList 读取求购大厅列表。
func hallList(t *testing.T, r *gin.Engine) []interface{} {
	t.Helper()
	code, resp := doAPIRequest(t, r, http.MethodGet, "/api/v1/wanteds", "", nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200 for hall list, got %d", code)
	}
	return resp["data"].(map[string]interface{})["list"].([]interface{})
}

// TestScenarioBudgetValidation 预算下限字段校验：
// 显式零元预算能创建并落库；字段缺失、空值、负值必须被拒绝且不产生数据。
func TestScenarioBudgetValidation(t *testing.T) {
	t.Run("explicit_zero_created", func(t *testing.T) {
		r := newScenarioEngine(t)
		payload := wantedPayload()
		payload["budget_min"] = 0
		code, resp := doAPIRequest(t, r, http.MethodPost, "/api/v1/wanteds", testToken(t, 1, "alice"), payload)
		if code != http.StatusOK {
			t.Fatalf("expected 200 for explicit zero budget_min, got %d resp=%v", code, resp)
		}
		data := resp["data"].(map[string]interface{})
		if data["budget_min"].(float64) != 0 {
			t.Fatalf("expected budget_min 0, got %v", data["budget_min"])
		}
		// 回读验证落库
		id := strconv.FormatUint(uint64(data["id"].(float64)), 10)
		code, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/wanteds/"+id, "", nil)
		if code != http.StatusOK || resp["data"].(map[string]interface{})["budget_min"].(float64) != 0 {
			t.Fatalf("read back zero budget_min failed, code=%d data=%v", code, resp["data"])
		}
	})

	rejectCases := []struct {
		name   string
		mutate func(map[string]interface{})
	}{
		{"missing_rejected", func(p map[string]interface{}) { delete(p, "budget_min") }},
		{"null_rejected", func(p map[string]interface{}) { p["budget_min"] = nil }},
		{"negative_rejected", func(p map[string]interface{}) { p["budget_min"] = -100 }},
	}
	for _, tc := range rejectCases {
		t.Run(tc.name, func(t *testing.T) {
			r := newScenarioEngine(t)
			payload := wantedPayload()
			tc.mutate(payload)
			code, _ := doAPIRequest(t, r, http.MethodPost, "/api/v1/wanteds", testToken(t, 1, "alice"), payload)
			if code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", code)
			}
			// 拒绝不能生效：大厅中不应出现任何求购
			if got := len(hallList(t, r)); got != 0 {
				t.Fatalf("rejected create should not persist, hall has %d entries", got)
			}
		})
	}
}

// TestScenarioContactPublisherMessage 从求购详情联系发起人：
// 不带商品的私信能正常发送，双方均可在会话中回读。
func TestScenarioContactPublisherMessage(t *testing.T) {
	r := newScenarioEngine(t)
	publisher := testToken(t, 1, "alice") // 求购发布者
	contactor := testToken(t, 2, "bob")   // 从详情页发起联系的用户

	// 准备：发布者创建一条求购
	code, resp := doAPIRequest(t, r, http.MethodPost, "/api/v1/wanteds", publisher, wantedPayload())
	if code != http.StatusOK {
		t.Fatalf("create wanted failed: %d", code)
	}

	// 联系发起人：发送不带 product_id 的私信（求购详情页"联系发布者"场景）
	content := "你好，我有一台符合你要求的 iPad 想出手"
	code, _ = doAPIRequest(t, r, http.MethodPost, "/api/v1/messages", contactor, map[string]interface{}{
		"receiver_id": 1,
		"content":     content,
	})
	if code != http.StatusOK {
		t.Fatalf("expected 200 for message without product, got %d", code)
	}

	// 发起方会话内回读：内容一致、不关联商品
	_, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/messages/conversations/1", contactor, nil)
	list := resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 message in contactor conversation, got %d", len(list))
	}
	msg := list[0].(map[string]interface{})
	if msg["content"].(string) != content || msg["product_id"].(float64) != 0 {
		t.Fatalf("read back mismatch: %+v", msg)
	}

	// 发布者会话列表出现发起方，未读数为 1
	_, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/messages/conversations", publisher, nil)
	convs := resp["data"].([]interface{})
	if len(convs) != 1 {
		t.Fatalf("expected 1 conversation for publisher, got %d", len(convs))
	}
	conv := convs[0].(map[string]interface{})
	if conv["peer_id"].(float64) != 2 || conv["unread_count"].(float64) != 1 {
		t.Fatalf("unexpected publisher conversation: %+v", conv)
	}

	// 发布者会话内回读同一条消息
	_, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/messages/conversations/2", publisher, nil)
	list = resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 1 || list[0].(map[string]interface{})["content"].(string) != content {
		t.Fatalf("publisher read back mismatch: %+v", list)
	}
}

// TestScenarioConcurrentClose 两个并发关闭请求：只允许一个成功、另一个冲突；
// 关闭后大厅不可见且详情仍可读。
func TestScenarioConcurrentClose(t *testing.T) {
	r := newScenarioEngine(t)
	alice := testToken(t, 1, "alice")

	// 准备：发布一条求购
	code, resp := doAPIRequest(t, r, http.MethodPost, "/api/v1/wanteds", alice, wantedPayload())
	if code != http.StatusOK {
		t.Fatalf("create wanted failed: %d", code)
	}
	wantedID := resp["data"].(map[string]interface{})["id"].(float64)
	idPath := "/api/v1/wanteds/" + strconv.FormatUint(uint64(wantedID), 10)

	// 两个并发关闭请求（统一起跑线，确保真正并发）
	codes := make([]int, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			c, _ := doAPIRequest(t, r, http.MethodPost, idPath+"/close", alice, nil)
			codes[idx] = c
		}(i)
	}
	close(start)
	wg.Wait()

	// 恰好一个成功、另一个冲突
	byCode := map[int]int{}
	for _, c := range codes {
		byCode[c]++
	}
	if byCode[http.StatusOK] != 1 || byCode[http.StatusConflict] != 1 {
		t.Fatalf("expected exactly one 200 and one 409, got %v", codes)
	}

	// 关闭后大厅不可见
	if got := len(hallList(t, r)); got != 0 {
		t.Fatalf("closed wanted should not appear in hall, got %d entries", got)
	}

	// 详情仍可读且状态为 closed
	code, resp = doAPIRequest(t, r, http.MethodGet, idPath, "", nil)
	if code != http.StatusOK {
		t.Fatalf("closed wanted detail should still be readable, got %d", code)
	}
	if status := resp["data"].(map[string]interface{})["status"].(string); status != "closed" {
		t.Fatalf("expected status closed, got %s", status)
	}
}
