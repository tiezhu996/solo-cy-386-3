package model

import (
	"time"

	"gorm.io/gorm"
)

// Wanted 求购需求实体：状态枚举见 internal/constants/enums.go，成色/分类复用商品枚举。
type Wanted struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	Title       string         `gorm:"size:128;not null" json:"title"`
	Category    string         `gorm:"size:32;not null;default:other" json:"category"`
	Condition   string         `gorm:"size:32;not null;default:almost_new" json:"condition"`
	BudgetMin   float64        `gorm:"type:numeric(12,2);not null" json:"budget_min"`
	BudgetMax   float64        `gorm:"type:numeric(12,2);not null" json:"budget_max"`
	City        string         `gorm:"size:64;not null" json:"city"`
	Description string         `gorm:"type:text;not null" json:"description"`
	Status      string         `gorm:"size:32;not null;default:open;index" json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}
