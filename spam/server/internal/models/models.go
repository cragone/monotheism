package models

import "time"

type User struct {
	ID           uint `gorm:"primaryKey"`
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type Session struct {
	TokenHash string `gorm:"primaryKey"`
	UserID    uint
	ExpiresAt time.Time
}

type Thread struct {
	ID           uint `gorm:"primaryKey"`
	UserID       uint
	Title        string
	Body         string
	CreatedAt    time.Time
	LastActivity time.Time
	Deleted      bool
}

type Reply struct {
	ID        uint `gorm:"primaryKey"`
	ThreadID  uint
	ParentID  *uint
	UserID    uint
	Body      string
	CreatedAt time.Time
	Deleted   bool
}

const (
	KindThread = "thread"
	KindReply  = "reply"
)

type Like struct {
	UserID   uint   `gorm:"primaryKey"`
	Kind     string `gorm:"primaryKey"`
	TargetID uint   `gorm:"primaryKey"`
}
