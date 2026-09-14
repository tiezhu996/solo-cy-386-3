package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/marketpal/marketpal/internal/model"
	"gorm.io/gorm"
)

// WantedRepository 求购需求仓储接口。
type WantedRepository interface {
	Create(wanted *model.Wanted) error
	GetByID(id uint) (*model.Wanted, error)
	List(query map[string]interface{}, page, pageSize int) ([]model.Wanted, int64, error)
	ListByUser(userID uint, page, pageSize int) ([]model.Wanted, int64, error)
	Update(wanted *model.Wanted) error
}

type wantedRepo struct {
	db *gorm.DB
}

// NewWantedRepository 构造求购需求仓储。
func NewWantedRepository(db *gorm.DB) WantedRepository {
	return &wantedRepo{db: db}
}

func (r *wantedRepo) Create(wanted *model.Wanted) error {
	if err := r.db.Create(wanted).Error; err != nil {
		return fmt.Errorf("create wanted: %w", err)
	}
	return nil
}

func (r *wantedRepo) GetByID(id uint) (*model.Wanted, error) {
	var w model.Wanted
	err := r.db.Preload("User").First(&w, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get wanted by id %d: %w", id, err)
	}
	return &w, nil
}

// List 求购大厅查询：关键词/分类/城市筛选，按最新发布时间排序。
// 模糊匹配使用 LOWER(...) LIKE（与 ILIKE 等效），兼容 PostgreSQL 与测试用 SQLite。
func (r *wantedRepo) List(query map[string]interface{}, page, pageSize int) ([]model.Wanted, int64, error) {
	var wanteds []model.Wanted
	var total int64
	q := r.db.Model(&model.Wanted{})
	if v, ok := query["keyword"]; ok && v != "" {
		kw := "%" + strings.ToLower(fmt.Sprintf("%v", v)) + "%"
		q = q.Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ?", kw, kw)
	}
	if v, ok := query["category"]; ok && v != "" {
		q = q.Where("category = ?", v)
	}
	if v, ok := query["city"]; ok && v != "" {
		q = q.Where("LOWER(city) LIKE ?", "%"+strings.ToLower(fmt.Sprintf("%v", v))+"%")
	}
	if v, ok := query["status"]; ok && v != "" {
		q = q.Where("status = ?", v)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count wanteds: %w", err)
	}
	if err := q.Preload("User").Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&wanteds).Error; err != nil {
		return nil, 0, fmt.Errorf("list wanteds: %w", err)
	}
	return wanteds, total, nil
}

func (r *wantedRepo) ListByUser(userID uint, page, pageSize int) ([]model.Wanted, int64, error) {
	var wanteds []model.Wanted
	var total int64
	q := r.db.Model(&model.Wanted{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count user wanteds: %w", err)
	}
	if err := q.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&wanteds).Error; err != nil {
		return nil, 0, fmt.Errorf("list user wanteds: %w", err)
	}
	return wanteds, total, nil
}

func (r *wantedRepo) Update(wanted *model.Wanted) error {
	if err := r.db.Save(wanted).Error; err != nil {
		return fmt.Errorf("update wanted %d: %w", wanted.ID, err)
	}
	return nil
}
