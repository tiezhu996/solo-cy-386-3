package dto

// WantedCreateRequest 求购需求发布入参。
// BudgetMin 使用指针：required 对指针判 nil，可区分"字段缺失/null"（拒绝）与"显式填 0"（合法）。
type WantedCreateRequest struct {
	Title       string   `json:"title" binding:"required,min=2,max=128"`
	Category    string   `json:"category" binding:"required,oneof=digital clothing books home sports other"`
	Condition   string   `json:"condition" binding:"required,oneof=brand_new almost_new lightly_used obviously_used"`
	BudgetMin   *float64 `json:"budget_min" binding:"required,gte=0"`
	BudgetMax   float64  `json:"budget_max" binding:"required,gt=0"`
	City        string   `json:"city" binding:"required,min=2,max=64"`
	Description string   `json:"description" binding:"required,min=5"`
}

// WantedQuery 求购大厅查询入参（关键词/分类/城市筛选，按最新发布排序）。
type WantedQuery struct {
	Keyword  string `form:"keyword"`
	Category string `form:"category" binding:"omitempty,oneof=digital clothing books home sports other"`
	City     string `form:"city" binding:"omitempty,max=64"`
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=50"`
}

// WantedVO 求购需求视图对象。
type WantedVO struct {
	ID          uint    `json:"id"`
	UserID      uint    `json:"user_id"`
	Title       string  `json:"title"`
	Category    string  `json:"category"`
	Condition   string  `json:"condition"`
	BudgetMin   float64 `json:"budget_min"`
	BudgetMax   float64 `json:"budget_max"`
	City        string  `json:"city"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	User        *UserVO `json:"user,omitempty"`
}

// WantedListResponse 求购需求列表分页响应。
type WantedListResponse struct {
	List  []WantedVO `json:"list"`
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"page_size"`
}
