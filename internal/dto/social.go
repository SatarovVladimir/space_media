package dto

import (
	"time"

	"my-awesome-project/internal/models"
)

type ProfileResponse struct {
	ID             int64  `json:"id"`
	Email          string `json:"email,omitempty"`
	DisplayName    string `json:"display_name"`
	Bio            string `json:"bio"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	ArticleCount   int64  `json:"article_count"`
	FollowersCount int64  `json:"followers_count"`
	FollowingCount int64  `json:"following_count,omitempty"`
}

func NewProfileResponse(profile models.Profile, includeEmail bool) ProfileResponse {
	response := ProfileResponse{
		ID:             profile.ID,
		DisplayName:    profile.DisplayName,
		Bio:            profile.Bio,
		AvatarURL:      profile.AvatarURL,
		ArticleCount:   profile.ArticleCount,
		FollowersCount: profile.FollowersCount,
		FollowingCount: profile.FollowingCount,
	}
	if includeEmail {
		response.Email = profile.Email
	}
	return response
}

type UpdateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
}

type CreateArticleRequest struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Body    string `json:"body"`
}

type ArticleResponse struct {
	ID           int64     `json:"id"`
	AuthorID     int64     `json:"author_id"`
	AuthorName   string    `json:"author_name"`
	AuthorAvatar string    `json:"author_avatar,omitempty"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	Body         string    `json:"body,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	Likes        int64     `json:"likes"`
	Dislikes     int64     `json:"dislikes"`
	CommentCount int64     `json:"comment_count"`
	MyReaction   string    `json:"my_reaction,omitempty"`
}

func NewArticleResponse(article models.Article) ArticleResponse {
	return ArticleResponse{
		ID:           article.ID,
		AuthorID:     article.AuthorID,
		AuthorName:   article.AuthorName,
		AuthorAvatar: article.AuthorAvatar,
		Title:        article.Title,
		Summary:      article.Summary,
		Body:         article.Body,
		CreatedAt:    article.CreatedAt,
		Likes:        article.Likes,
		Dislikes:     article.Dislikes,
		CommentCount: article.CommentCount,
		MyReaction:   article.MyReaction,
	}
}

type CreateCommentRequest struct {
	Body string `json:"body"`
}

type CommentResponse struct {
	ID         int64     `json:"id"`
	ArticleID  int64     `json:"article_id"`
	AuthorID   int64     `json:"author_id"`
	AuthorName string    `json:"author_name"`
	AvatarURL  string    `json:"avatar_url,omitempty"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

func NewCommentResponse(comment models.Comment) CommentResponse {
	return CommentResponse{
		ID:         comment.ID,
		ArticleID:  comment.ArticleID,
		AuthorID:   comment.AuthorID,
		AuthorName: comment.AuthorName,
		AvatarURL:  comment.AvatarURL,
		Body:       comment.Body,
		CreatedAt:  comment.CreatedAt,
	}
}

type ReactionRequest struct {
	Reaction string `json:"reaction"`
}

type ReactionResponse struct {
	Reaction string `json:"reaction"`
}

type FollowedAuthorResponse struct {
	ID             int64  `json:"id"`
	DisplayName    string `json:"display_name"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	ArticleCount   int64  `json:"article_count"`
	FollowersCount int64  `json:"followers_count"`
}

type PublicationPlanResponse struct {
	Active                bool       `json:"active"`
	Pending               bool       `json:"pending"`
	OrderID               int64      `json:"order_id,omitempty"`
	DurationMonths        int        `json:"duration_months,omitempty"`
	Status                string     `json:"status,omitempty"`
	UpdatedAt             time.Time  `json:"updated_at,omitempty"`
	SubscriptionExpiresAt *time.Time `json:"subscription_expires_at,omitempty"`
}

type CreatePublicationPlanRequest struct {
	DurationMonths int `json:"duration_months"`
}
