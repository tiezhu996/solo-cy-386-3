package repository

import (
	"testing"

	"github.com/marketpal/marketpal/internal/model"
)

// 注意：测试库为共享内存 SQLite（见 repository_test.go），用例内使用唯一字段值避免相互干扰。

func TestWantedRepository(t *testing.T) {
	db := newTestDB(t)
	repo := NewWantedRepository(db)

	t.Run("create_and_get", func(t *testing.T) {
		w := &model.Wanted{UserID: 1, Title: "求购 iPhone 13", Category: "digital", Condition: "almost_new", BudgetMin: 2000, BudgetMax: 3500, City: "杭州", Description: "想要一台成色好的 iPhone 13", Status: "open"}
		if err := repo.Create(w); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
		if w.ID == 0 {
			t.Fatal("id should be assigned")
		}
		got, err := repo.GetByID(w.ID)
		if err != nil {
			t.Fatalf("GetByID() error: %v", err)
		}
		if got.Title != "求购 iPhone 13" || got.City != "杭州" || got.Status != "open" {
			t.Fatalf("unexpected wanted: %+v", got)
		}
	})

	t.Run("get_missing", func(t *testing.T) {
		if _, err := repo.GetByID(999999); err != ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("list_filters_and_order", func(t *testing.T) {
		_ = repo.Create(&model.Wanted{UserID: 1, Title: "求购机械键盘", Category: "digital", Condition: "lightly_used", BudgetMin: 100, BudgetMax: 300, City: "上海", Description: "求一把成色不错的机械键盘", Status: "open"})
		_ = repo.Create(&model.Wanted{UserID: 2, Title: "求购羽毛球拍", Category: "sports", Condition: "almost_new", BudgetMin: 50, BudgetMax: 200, City: "上海", Description: "求购一支羽毛球拍", Status: "open"})
		_ = repo.Create(&model.Wanted{UserID: 2, Title: "求购已关闭的耳机", Category: "digital", Condition: "almost_new", BudgetMin: 100, BudgetMax: 500, City: "上海", Description: "已关闭的求购不应出现在大厅", Status: "closed"})

		// 分类筛选
		list, total, err := repo.List(map[string]interface{}{"category": "sports", "status": "open"}, 1, 10)
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if total != 1 || len(list) != 1 || list[0].Title != "求购羽毛球拍" {
			t.Fatalf("category filter failed: total=%d list=%+v", total, list)
		}

		// 城市 + 关键词组合筛选
		_, total, err = repo.List(map[string]interface{}{"city": "上海", "keyword": "键盘", "status": "open"}, 1, 10)
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if total != 1 {
			t.Fatalf("city+keyword filter failed: total=%d", total)
		}

		// 大厅只看 open：closed 不出现，且按创建时间倒序（最新在前）
		list, total, err = repo.List(map[string]interface{}{"city": "上海", "status": "open"}, 1, 10)
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if total != 2 || len(list) != 2 {
			t.Fatalf("expected 2 open wanteds, got total=%d len=%d", total, len(list))
		}
		for _, w := range list {
			if w.Status != "open" {
				t.Fatalf("closed wanted should not appear in hall: %+v", w)
			}
		}
		if list[0].Title != "求购羽毛球拍" {
			t.Fatalf("expected latest first, got %s", list[0].Title)
		}
	})

	t.Run("list_by_user_and_update", func(t *testing.T) {
		list, total, err := repo.ListByUser(1, 1, 50)
		if err != nil {
			t.Fatalf("ListByUser() error: %v", err)
		}
		if total < 2 {
			t.Fatalf("expected user 1 wanteds, got total=%d", total)
		}
		target := list[0]
		target.Status = "closed"
		if err := repo.Update(&target); err != nil {
			t.Fatalf("Update() error: %v", err)
		}
		got, _ := repo.GetByID(target.ID)
		if got.Status != "closed" {
			t.Fatalf("expected closed, got %s", got.Status)
		}
	})
}
