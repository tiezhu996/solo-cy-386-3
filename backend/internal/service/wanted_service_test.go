package service

import (
	"io"
	"log/slog"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/marketpal/marketpal/internal/constants"
	"github.com/marketpal/marketpal/internal/dto"
	"github.com/marketpal/marketpal/internal/model"
	"github.com/marketpal/marketpal/internal/repository"
	"github.com/marketpal/marketpal/internal/util"
	"gorm.io/gorm"
)

// newWantedTestService 基于内存 SQLite 构造求购服务。
func newWantedTestService(t *testing.T) *WantedService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Wanted{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewWantedService(repository.NewWantedRepository(db), logger)
}

func f64(v float64) *float64 { return &v }

func validWantedReq() dto.WantedCreateRequest {
	return dto.WantedCreateRequest{
		Title:       "求购一台 iPad",
		Category:    "digital",
		Condition:   "almost_new",
		BudgetMin:   f64(1000),
		BudgetMax:   2000,
		City:        "北京市",
		Description: "希望成色好一点，配件齐全",
	}
}

func TestWantedServiceCreate(t *testing.T) {
	svc := newWantedTestService(t)

	t.Run("create_ok_persisted", func(t *testing.T) {
		w, err := svc.Create(1, validWantedReq())
		if err != nil {
			t.Fatalf("Create() error: %v", err)
		}
		if w.ID == 0 || w.Status != constants.WantedStatusOpen {
			t.Fatalf("unexpected wanted: %+v", w)
		}
		// 回读持久化数据
		got, err := svc.GetDetail(w.ID)
		if err != nil {
			t.Fatalf("GetDetail() error: %v", err)
		}
		if got.Title != w.Title || got.BudgetMin != 1000 || got.BudgetMax != 2000 || got.City != "北京市" {
			t.Fatalf("read back mismatch: %+v", got)
		}
	})

	t.Run("zero_budget_min_ok", func(t *testing.T) {
		req := validWantedReq()
		req.BudgetMin = f64(0)
		w, err := svc.Create(1, req)
		if err != nil {
			t.Fatalf("Create() with zero budget_min error: %v", err)
		}
		got, _ := svc.GetDetail(w.ID)
		if got.BudgetMin != 0 {
			t.Fatalf("expected budget_min 0 persisted, got %v", got.BudgetMin)
		}
	})

	t.Run("budget_min_missing_rejected", func(t *testing.T) {
		req := validWantedReq()
		req.BudgetMin = nil
		if _, err := svc.Create(1, req); err == nil {
			t.Fatal("expected error for missing budget_min, got nil")
		} else {
			appErr, ok := err.(*util.AppError)
			if !ok || appErr.Code != constants.CodeBadRequest {
				t.Fatalf("expected CodeBadRequest, got %v", err)
			}
		}
	})

	t.Run("budget_inverted_rejected", func(t *testing.T) {
		req := validWantedReq()
		req.BudgetMin, req.BudgetMax = f64(5000), 1000
		if _, err := svc.Create(1, req); err == nil {
			t.Fatal("expected error for inverted budget, got nil")
		} else {
			appErr, ok := err.(*util.AppError)
			if !ok || appErr.Code != constants.CodeBadRequest {
				t.Fatalf("expected CodeBadRequest, got %v", err)
			}
		}
	})

	t.Run("invalid_category_rejected", func(t *testing.T) {
		req := validWantedReq()
		req.Category = "not_a_category"
		if _, err := svc.Create(1, req); err == nil {
			t.Fatal("expected error for invalid category, got nil")
		}
	})
}

func TestWantedServiceClose(t *testing.T) {
	svc := newWantedTestService(t)
	w, err := svc.Create(1, validWantedReq())
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	t.Run("close_by_other_user_forbidden", func(t *testing.T) {
		if _, err := svc.Close(2, w.ID); err == nil {
			t.Fatal("expected forbidden error, got nil")
		} else {
			appErr, ok := err.(*util.AppError)
			if !ok || appErr.Code != constants.CodeForbidden {
				t.Fatalf("expected CodeForbidden, got %v", err)
			}
		}
		// 越权关闭不能生效：仍为 open
		got, _ := svc.GetDetail(w.ID)
		if got.Status != constants.WantedStatusOpen {
			t.Fatalf("status should stay open, got %s", got.Status)
		}
	})

	t.Run("close_by_owner_ok", func(t *testing.T) {
		closed, err := svc.Close(1, w.ID)
		if err != nil {
			t.Fatalf("Close() error: %v", err)
		}
		if closed.Status != constants.WantedStatusClosed {
			t.Fatalf("expected closed, got %s", closed.Status)
		}
	})

	t.Run("closed_not_in_hall_but_detail_visible", func(t *testing.T) {
		res, err := svc.List(dto.WantedQuery{Page: 1, PageSize: 50})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		for _, item := range res.List {
			if item.ID == w.ID {
				t.Fatal("closed wanted should not appear in hall")
			}
		}
		got, err := svc.GetDetail(w.ID)
		if err != nil {
			t.Fatalf("closed wanted detail should still be visible: %v", err)
		}
		if got.Status != constants.WantedStatusClosed {
			t.Fatalf("expected closed, got %s", got.Status)
		}
		// 我的求购中仍可见
		mine, err := svc.ListMine(1, 1, 50)
		if err != nil {
			t.Fatalf("ListMine() error: %v", err)
		}
		found := false
		for _, item := range mine.List {
			if item.ID == w.ID {
				found = true
			}
		}
		if !found {
			t.Fatal("closed wanted should still appear in my list")
		}
	})

	t.Run("close_twice_conflict", func(t *testing.T) {
		if _, err := svc.Close(1, w.ID); err == nil {
			t.Fatal("expected conflict error, got nil")
		} else {
			appErr, ok := err.(*util.AppError)
			if !ok || appErr.Code != constants.CodeWantedClosed {
				t.Fatalf("expected CodeWantedClosed, got %v", err)
			}
		}
	})

	t.Run("close_missing_not_found", func(t *testing.T) {
		if _, err := svc.Close(1, 999999); err == nil {
			t.Fatal("expected not found error, got nil")
		} else {
			appErr, ok := err.(*util.AppError)
			if !ok || appErr.Code != constants.CodeWantedNotFound {
				t.Fatalf("expected CodeWantedNotFound, got %v", err)
			}
		}
	})
}

func TestWantedServiceListFilters(t *testing.T) {
	svc := newWantedTestService(t)
	mk := func(city, category, title string) {
		req := validWantedReq()
		req.City, req.Category, req.Title = city, category, title
		if _, err := svc.Create(1, req); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
	}
	mk("上海", "digital", "求购相机镜头")
	mk("上海", "books", "求购 golang 书籍")
	mk("广州", "digital", "求购显示器")

	t.Run("filter_by_city", func(t *testing.T) {
		res, err := svc.List(dto.WantedQuery{City: "上海", Page: 1, PageSize: 50})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if res.Total != 2 {
			t.Fatalf("expected 2 wanteds in 上海, got %d", res.Total)
		}
	})

	t.Run("filter_by_category_and_keyword", func(t *testing.T) {
		res, _ := svc.List(dto.WantedQuery{Category: "books", Page: 1, PageSize: 50})
		if res.Total != 1 {
			t.Fatalf("expected 1 books wanted, got %d", res.Total)
		}
		res, _ = svc.List(dto.WantedQuery{Keyword: "相机", Page: 1, PageSize: 50})
		if res.Total != 1 || res.List[0].Title != "求购相机镜头" {
			t.Fatalf("keyword filter failed: %+v", res.List)
		}
	})

	t.Run("latest_first", func(t *testing.T) {
		res, _ := svc.List(dto.WantedQuery{Page: 1, PageSize: 50})
		if len(res.List) != 3 {
			t.Fatalf("expected 3 wanteds, got %d", len(res.List))
		}
		if res.List[0].Title != "求购显示器" {
			t.Fatalf("expected latest first, got %s", res.List[0].Title)
		}
	})
}
