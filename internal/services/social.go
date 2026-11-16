package services

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"my-awesome-project/internal/models"
)

var (
	ErrInvalidProfile  = errors.New("invalid profile")
	ErrInvalidArticle  = errors.New("invalid article")
	ErrInvalidComment  = errors.New("invalid comment")
	ErrInvalidReaction = errors.New("invalid reaction")
)

type SocialRepository interface {
	GetProfile(ctx context.Context, userID int64) (models.Profile, error)
	UpdateProfile(ctx context.Context, userID int64, displayName, bio string) (models.Profile, error)
	UpdateAvatar(ctx context.Context, userID int64, avatarURL string) (string, error)
	CreateArticle(ctx context.Context, authorID int64, title, summary, body string) (models.Article, error)
	ListArticles(ctx context.Context, query string, authorID, viewerID int64, followedOnly bool) ([]models.Article, error)
	GetArticle(ctx context.Context, articleID, viewerID int64) (models.Article, error)
	ListComments(ctx context.Context, articleID int64) ([]models.Comment, error)
	CreateComment(ctx context.Context, articleID, authorID int64, body string) (models.Comment, error)
	ToggleReaction(ctx context.Context, articleID, userID int64, reaction string) (string, error)
	Follow(ctx context.Context, followerID, authorID int64) error
	Unfollow(ctx context.Context, followerID, authorID int64) error
	ListFollowedAuthors(ctx context.Context, followerID int64) ([]models.FollowedAuthor, error)
}

type SocialService struct {
	repository SocialRepository
}

func NewSocialService(repository SocialRepository) *SocialService {
	return &SocialService{repository: repository}
}

func (s *SocialService) GetProfile(ctx context.Context, userID int64) (models.Profile, error) {
	if userID <= 0 {
		return models.Profile{}, ErrInvalidProfile
	}
	return s.repository.GetProfile(ctx, userID)
}

func (s *SocialService) UpdateProfile(ctx context.Context, userID int64, displayName, bio string) (models.Profile, error) {
	displayName = strings.TrimSpace(displayName)
	bio = strings.TrimSpace(bio)
	if userID <= 0 || !utf8.ValidString(displayName) || utf8.RuneCountInString(displayName) < 2 ||
		utf8.RuneCountInString(displayName) > 60 || !utf8.ValidString(bio) || len(bio) > 1000 {
		return models.Profile{}, ErrInvalidProfile
	}
	return s.repository.UpdateProfile(ctx, userID, displayName, bio)
}

func (s *SocialService) UpdateAvatar(ctx context.Context, userID int64, avatarURL string) (string, error) {
	if userID <= 0 || avatarURL == "" {
		return "", ErrInvalidProfile
	}
	return s.repository.UpdateAvatar(ctx, userID, avatarURL)
}

func (s *SocialService) CreateArticle(ctx context.Context, authorID int64, title, summary, body string) (models.Article, error) {
	title = strings.TrimSpace(title)
	summary = strings.TrimSpace(summary)
	body = strings.TrimSpace(body)
	if authorID <= 0 || !utf8.ValidString(title) || utf8.RuneCountInString(title) < 3 ||
		utf8.RuneCountInString(title) > 160 || !utf8.ValidString(summary) || len(summary) > 1000 ||
		!utf8.ValidString(body) || len(body) < 20 || len(body) > 100000 {
		return models.Article{}, ErrInvalidArticle
	}
	article, err := s.repository.CreateArticle(ctx, authorID, title, summary, body)
	return article, err
}

func (s *SocialService) ListArticles(ctx context.Context, query string, authorID, viewerID int64, followedOnly bool) ([]models.Article, error) {
	query = strings.TrimSpace(query)
	if len(query) > 200 || !utf8.ValidString(query) || authorID < 0 || viewerID < 0 || (followedOnly && viewerID <= 0) {
		return nil, ErrInvalidArticle
	}
	return s.repository.ListArticles(ctx, query, authorID, viewerID, followedOnly)
}

func (s *SocialService) GetArticle(ctx context.Context, articleID, viewerID int64) (models.Article, error) {
	if articleID <= 0 || viewerID < 0 {
		return models.Article{}, ErrInvalidArticle
	}
	return s.repository.GetArticle(ctx, articleID, viewerID)
}

func (s *SocialService) ListComments(ctx context.Context, articleID int64) ([]models.Comment, error) {
	if articleID <= 0 {
		return nil, ErrInvalidComment
	}
	if _, err := s.repository.GetArticle(ctx, articleID, 0); err != nil {
		return nil, err
	}
	return s.repository.ListComments(ctx, articleID)
}

func (s *SocialService) CreateComment(ctx context.Context, articleID, authorID int64, body string) (models.Comment, error) {
	body = strings.TrimSpace(body)
	if articleID <= 0 || authorID <= 0 || !utf8.ValidString(body) || utf8.RuneCountInString(body) == 0 ||
		utf8.RuneCountInString(body) > 2000 {
		return models.Comment{}, ErrInvalidComment
	}
	return s.repository.CreateComment(ctx, articleID, authorID, body)
}

func (s *SocialService) ToggleReaction(ctx context.Context, articleID, userID int64, reaction string) (string, error) {
	if articleID <= 0 || userID <= 0 || (reaction != "like" && reaction != "dislike") {
		return "", ErrInvalidReaction
	}
	return s.repository.ToggleReaction(ctx, articleID, userID, reaction)
}

func (s *SocialService) Follow(ctx context.Context, followerID, authorID int64) error {
	if followerID <= 0 || authorID <= 0 {
		return ErrInvalidProfile
	}
	return s.repository.Follow(ctx, followerID, authorID)
}

func (s *SocialService) Unfollow(ctx context.Context, followerID, authorID int64) error {
	if followerID <= 0 || authorID <= 0 {
		return ErrInvalidProfile
	}
	return s.repository.Unfollow(ctx, followerID, authorID)
}

func (s *SocialService) ListFollowedAuthors(ctx context.Context, followerID int64) ([]models.FollowedAuthor, error) {
	if followerID <= 0 {
		return nil, ErrInvalidProfile
	}
	return s.repository.ListFollowedAuthors(ctx, followerID)
}
