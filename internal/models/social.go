package models

import (
	"errors"
	"time"
)

var (
	ErrPublicationPlanExists = errors.New("publication plan order is already pending")
	ErrPublicationLimit      = errors.New("free article limit reached")
)

type Profile struct {
	ID             int64
	Email          string
	DisplayName    string
	Bio            string
	AvatarURL      string
	ArticleCount   int64
	FollowersCount int64
	FollowingCount int64
}

type Article struct {
	ID           int64
	AuthorID     int64
	AuthorName   string
	AuthorAvatar string
	Title        string
	Summary      string
	Body         string
	CreatedAt    time.Time
	Likes        int64
	Dislikes     int64
	CommentCount int64
	MyReaction   string
}

type Comment struct {
	ID         int64
	ArticleID  int64
	AuthorID   int64
	AuthorName string
	AvatarURL  string
	Body       string
	CreatedAt  time.Time
}

type FollowedAuthor struct {
	ID             int64
	DisplayName    string
	AvatarURL      string
	ArticleCount   int64
	FollowersCount int64
}

type PublicationPlan struct {
	Active                bool
	Pending               bool
	OrderID               int64
	DurationMonths        int
	Status                string
	UpdatedAt             time.Time
	SubscriptionExpiresAt *time.Time
}
