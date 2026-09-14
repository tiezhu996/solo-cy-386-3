package service

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/marketpal/marketpal/internal/constants"
	"github.com/marketpal/marketpal/internal/dto"
	"github.com/marketpal/marketpal/internal/model"
	"github.com/marketpal/marketpal/internal/repository"
	"github.com/marketpal/marketpal/internal/util"
)

// WantedService 求购需求业务服务。
type WantedService struct {
	wantedRepo repository.WantedRepository
	logger     *slog.Logger
}

// NewWantedService 构造求购需求服务。
func NewWantedService(wantedRepo repository.WantedRepository, logger *slog.Logger) *WantedService {
	return &WantedService{wantedRepo: wantedRepo, logger: logger}
}

// Create 发布求购需求（预算下限不得大于上限）。
func (s *WantedService) Create(userID uint, req dto.WantedCreateRequest) (*model.Wanted, error) {
	if !constants.ValidProductCategory(req.Category) {
		return nil, util.NewAppError(constants.CodeBadRequest, "求购发布失败：分类 "+req.Category+" 非法", nil)
	}
	if !constants.ValidProductCondition(req.Condition) {
		return nil, util.NewAppError(constants.CodeBadRequest, "求购发布失败：成色 "+req.Condition+" 非法", nil)
	}
	if req.BudgetMin > req.BudgetMax {
		return nil, util.NewAppError(constants.CodeBadRequest, "求购发布失败：预算下限 "+util.FormatPrice(req.BudgetMin)+" 不能大于预算上限 "+util.FormatPrice(req.BudgetMax), nil)
	}
	wanted := &model.Wanted{
		UserID:      userID,
		Title:       req.Title,
		Category:    req.Category,
		Condition:   req.Condition,
		BudgetMin:   req.BudgetMin,
		BudgetMax:   req.BudgetMax,
		City:        req.City,
		Description: req.Description,
		Status:      constants.WantedStatusOpen,
	}
	if err := s.wantedRepo.Create(wanted); err != nil {
		return nil, fmt.Errorf("create wanted user=%d: %w", userID, err)
	}
	s.logger.Info(constants.LogWantedCreated, "wanted_id", wanted.ID, "user_id", userID, "category", wanted.Category, "city", wanted.City)
	return wanted, nil
}

// Close 发布者关闭求购需求（仅本人可操作，重复关闭返回冲突）。
func (s *WantedService) Close(userID, wantedID uint) (*model.Wanted, error) {
	wanted, err := s.wantedRepo.GetByID(wantedID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeWantedNotFound, "求购关闭失败：求购需求 id="+fmt.Sprint(wantedID)+" 不存在", err)
		}
		return nil, fmt.Errorf("get wanted %d for close: %w", wantedID, err)
	}
	if wanted.UserID != userID {
		return nil, util.NewAppError(constants.CodeForbidden, "求购关闭失败：只有发布者（用户 id="+fmt.Sprint(wanted.UserID)+"）可关闭该求购需求", nil)
	}
	if wanted.Status == constants.WantedStatusClosed {
		return nil, util.NewAppError(constants.CodeWantedClosed, "求购关闭失败：求购需求 id="+fmt.Sprint(wantedID)+" 已关闭", nil)
	}
	wanted.Status = constants.WantedStatusClosed
	if err := s.wantedRepo.Update(wanted); err != nil {
		return nil, fmt.Errorf("close wanted %d: %w", wantedID, err)
	}
	s.logger.Info(constants.LogWantedClosed, "wanted_id", wantedID, "user_id", userID, "status", wanted.Status)
	return wanted, nil
}

// GetDetail 求购需求详情（已关闭的仍可查看）。
func (s *WantedService) GetDetail(wantedID uint) (*model.Wanted, error) {
	wanted, err := s.wantedRepo.GetByID(wantedID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeWantedNotFound, "求购详情查询失败：求购需求 id="+fmt.Sprint(wantedID)+" 不存在", err)
		}
		return nil, fmt.Errorf("get wanted detail %d: %w", wantedID, err)
	}
	s.logger.Info(constants.LogWantedViewed, "wanted_id", wantedID)
	return wanted, nil
}

// List 求购大厅：仅展示求购中的需求，按最新发布时间排序。
func (s *WantedService) List(q dto.WantedQuery) (*dto.WantedListResponse, error) {
	page, pageSize := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := map[string]interface{}{
		"keyword":  q.Keyword,
		"category": q.Category,
		"city":     q.City,
		"status":   constants.WantedStatusOpen,
	}
	wanteds, total, err := s.wantedRepo.List(query, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("list wanteds: %w", err)
	}
	list := make([]dto.WantedVO, 0, len(wanteds))
	for i := range wanteds {
		list = append(list, dto.FromWanted(&wanteds[i]))
	}
	return &dto.WantedListResponse{List: list, Total: total, Page: page, Size: pageSize}, nil
}

// ListMine 我的求购（含已关闭，发布者视角）。
func (s *WantedService) ListMine(userID uint, page, pageSize int) (*dto.WantedListResponse, error) {
	p := util.NormalizePage(page, pageSize)
	wanteds, total, err := s.wantedRepo.ListByUser(userID, p.Page, p.PageSize)
	if err != nil {
		return nil, fmt.Errorf("list user wanteds user=%d: %w", userID, err)
	}
	list := make([]dto.WantedVO, 0, len(wanteds))
	for i := range wanteds {
		list = append(list, dto.FromWanted(&wanteds[i]))
	}
	return &dto.WantedListResponse{List: list, Total: total, Page: p.Page, Size: p.PageSize}, nil
}
