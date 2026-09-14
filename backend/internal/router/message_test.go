package router

import (
	"io"
	"log/slog"
	"net/http"
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

// setupMessageRouter 基于内存 SQLite 装配私信路由（Hub 无 Redis，退化为进程内推送）。
func setupMessageRouter(t *testing.T) *gin.Engine {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Product{}, &model.Message{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	// 造一个商品，供"带商品私信"回归用。
	if err := db.Exec("INSERT INTO products (seller_id, title, description, original_price, price, condition, category, status) VALUES (2, 'iPad Pro', '九成新', 6000, 4200, 'almost_new', 'digital', 'on_sale')").Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := service.NewHub(nil, logger)
	msgSvc := service.NewMessageService(repository.NewMessageRepository(db), hub, logger)
	msgHandler := handler.NewMessageHandler(msgSvc)
	wsHandler := handler.NewWSHandler(hub, logger)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler(logger))
	api := r.Group("/api/v1")
	RegisterMessageRoutes(api, msgHandler, wsHandler, testJWTSecret)
	return r
}

// TestMessageSendWithoutProduct 求购场景：联系发布者发起的私信不关联商品，
// 发送应成功（不违反商品外键），且双方会话可正常回读。
func TestMessageSendWithoutProduct(t *testing.T) {
	r := setupMessageRouter(t)
	alice := testToken(t, 1, "alice")
	bob := testToken(t, 2, "bob")

	// 不带 product_id 发送（求购详情页"联系发布者"场景）
	code, resp := doAPIRequest(t, r, http.MethodPost, "/api/v1/messages", alice, map[string]interface{}{
		"receiver_id": 2,
		"content":     "你好，我在求购大厅看到你想买 iPad，我有一台想出手",
	})
	if code != http.StatusOK {
		t.Fatalf("expected 200 for message without product, got %d resp=%v", code, resp)
	}

	// 发送者会话内回读
	code, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/messages/conversations/2", alice, nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200 for conversation, got %d", code)
	}
	list := resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 message in conversation, got %d", len(list))
	}
	msg := list[0].(map[string]interface{})
	if msg["content"].(string) != "你好，我在求购大厅看到你想买 iPad，我有一台想出手" {
		t.Fatalf("read back content mismatch: %v", msg["content"])
	}
	if msg["product_id"].(float64) != 0 {
		t.Fatalf("expected product_id 0 for wanted message, got %v", msg["product_id"])
	}

	// 接收者会话列表可见且未读数为 1
	code, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/messages/conversations", bob, nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200 for conversations, got %d", code)
	}
	convs := resp["data"].([]interface{})
	if len(convs) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convs))
	}
	conv := convs[0].(map[string]interface{})
	if conv["peer_id"].(float64) != 1 || conv["unread_count"].(float64) != 1 {
		t.Fatalf("unexpected conversation: %+v", conv)
	}

	// 回归：带商品 id 的私信仍然正常
	code, _ = doAPIRequest(t, r, http.MethodPost, "/api/v1/messages", alice, map[string]interface{}{
		"receiver_id": 2,
		"product_id":  1,
		"content":     "这台 iPad 还在吗",
	})
	if code != http.StatusOK {
		t.Fatalf("expected 200 for message with product, got %d", code)
	}
	_, resp = doAPIRequest(t, r, http.MethodGet, "/api/v1/messages/conversations/2", alice, nil)
	list = resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 2 {
		t.Fatalf("expected 2 messages after product message, got %d", len(list))
	}
}
