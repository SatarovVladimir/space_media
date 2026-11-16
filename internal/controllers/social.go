package controllers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"my-awesome-project/internal/dto"
	"my-awesome-project/internal/middleware"
	"my-awesome-project/internal/models"
	"my-awesome-project/internal/repositories"
	"my-awesome-project/internal/services"

	_ "golang.org/x/image/webp"
)

const maxAvatarBytes = 2 << 20

type socialService interface {
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

type publicationPlanService interface {
	CreatePublicationPlan(ctx context.Context, userID int64, durationMonths int) (models.Order, error)
	PublicationPlan(ctx context.Context, userID int64) (models.PublicationPlan, error)
}

type SocialController struct {
	service   socialService
	plans     publicationPlanService
	avatarDir string
}

func NewSocialController(service socialService, plans publicationPlanService, avatarDir string) *SocialController {
	return &SocialController{service: service, plans: plans, avatarDir: avatarDir}
}

func (c *SocialController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/articles":
		c.articles(w, r)
	case strings.HasPrefix(r.URL.Path, "/articles/"):
		c.article(w, r)
	case strings.HasPrefix(r.URL.Path, "/profiles/"):
		c.publicProfile(w, r)
	case r.URL.Path == "/profile/me":
		c.myProfile(w, r)
	case r.URL.Path == "/profile/me/avatar":
		c.uploadAvatar(w, r)
	case r.URL.Path == "/follows":
		c.follows(w, r)
	case strings.HasPrefix(r.URL.Path, "/follows/"):
		c.follow(w, r)
	case r.URL.Path == "/publication-plan":
		c.publicationPlan(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (c *SocialController) articles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		authorID := int64(0)
		if value := r.URL.Query().Get("author_id"); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil || parsed <= 0 {
				http.Error(w, "invalid author_id", http.StatusBadRequest)
				return
			}
			authorID = parsed
		}
		userID, _ := middleware.UserID(r.Context())
		followedOnly := r.URL.Query().Get("followed") == "true"
		if followedOnly && userID == 0 {
			http.Error(w, "sign in to view your followed authors", http.StatusUnauthorized)
			return
		}
		articles, err := c.service.ListArticles(r.Context(), r.URL.Query().Get("q"), authorID,
			userID, followedOnly)
		if err != nil {
			if errors.Is(err, services.ErrInvalidArticle) {
				http.Error(w, "invalid article search", http.StatusBadRequest)
				return
			}
			slog.ErrorContext(r.Context(), "search articles", "error", err)
			http.Error(w, "could not search articles", http.StatusInternalServerError)
			return
		}
		response := make([]dto.ArticleResponse, 0, len(articles))
		for _, article := range articles {
			response = append(response, dto.NewArticleResponse(article))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		userID, ok := middleware.UserID(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var input dto.CreateArticleRequest
		if err := decodeJSON(w, r, &input); err != nil {
			http.Error(w, "invalid JSON request", http.StatusBadRequest)
			return
		}
		article, err := c.service.CreateArticle(r.Context(), userID, input.Title, input.Summary, input.Body)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrInvalidArticle):
				http.Error(w, "title must be 3 to 160 characters, summary up to 1000 bytes, and article body 20 to 100000 bytes", http.StatusBadRequest)
			case errors.Is(err, models.ErrPublicationLimit):
				http.Error(w, "free limit reached; activate a publication plan to publish more articles", http.StatusPaymentRequired)
			default:
				slog.ErrorContext(r.Context(), "create article", "error", err)
				http.Error(w, "could not publish article", http.StatusInternalServerError)
			}
			return
		}
		writeJSON(w, http.StatusCreated, dto.NewArticleResponse(article))
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *SocialController) article(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/articles/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	articleID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || articleID <= 0 {
		http.Error(w, "invalid article ID", http.StatusBadRequest)
		return
	}
	userID, _ := middleware.UserID(r.Context())

	if len(parts) == 1 && r.Method == http.MethodGet {
		article, err := c.service.GetArticle(r.Context(), articleID, userID)
		if err != nil {
			c.writeSocialError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, dto.NewArticleResponse(article))
		return
	}
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	switch parts[1] {
	case "comments":
		switch r.Method {
		case http.MethodGet:
			comments, err := c.service.ListComments(r.Context(), articleID)
			if err != nil {
				c.writeSocialError(w, r, err)
				return
			}
			response := make([]dto.CommentResponse, 0, len(comments))
			for _, comment := range comments {
				response = append(response, dto.NewCommentResponse(comment))
			}
			writeJSON(w, http.StatusOK, response)
		case http.MethodPost:
			if userID == 0 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			var input dto.CreateCommentRequest
			if err := decodeJSON(w, r, &input); err != nil {
				http.Error(w, "invalid JSON request", http.StatusBadRequest)
				return
			}
			comment, err := c.service.CreateComment(r.Context(), articleID, userID, input.Body)
			if err != nil {
				c.writeSocialError(w, r, err)
				return
			}
			writeJSON(w, http.StatusCreated, dto.NewCommentResponse(comment))
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case "reaction":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if userID == 0 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var input dto.ReactionRequest
		if err := decodeJSON(w, r, &input); err != nil {
			http.Error(w, "invalid JSON request", http.StatusBadRequest)
			return
		}
		current, err := c.service.ToggleReaction(r.Context(), articleID, userID, input.Reaction)
		if err != nil {
			c.writeSocialError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, dto.ReactionResponse{Reaction: current})
	default:
		http.NotFound(w, r)
	}
}

func (c *SocialController) publicProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/profiles/"), 10, 64)
	if err != nil || userID <= 0 {
		http.Error(w, "invalid profile ID", http.StatusBadRequest)
		return
	}
	profile, err := c.service.GetProfile(r.Context(), userID)
	if err != nil {
		c.writeSocialError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewProfileResponse(profile, false))
}

func (c *SocialController) myProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		profile, err := c.service.GetProfile(r.Context(), userID)
		if err != nil {
			c.writeSocialError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, dto.NewProfileResponse(profile, true))
	case http.MethodPut:
		var input dto.UpdateProfileRequest
		if err := decodeJSON(w, r, &input); err != nil {
			http.Error(w, "invalid JSON request", http.StatusBadRequest)
			return
		}
		profile, err := c.service.UpdateProfile(r.Context(), userID, input.DisplayName, input.Bio)
		if err != nil {
			if errors.Is(err, services.ErrInvalidProfile) {
				http.Error(w, "display name must be 2 to 60 characters; bio up to 1000 bytes", http.StatusBadRequest)
				return
			}
			c.writeSocialError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, dto.NewProfileResponse(profile, true))
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *SocialController) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+(64<<10))
	file, _, err := r.FormFile("avatar")
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			http.Error(w, "avatar upload is too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "upload a JPG, PNG, or WebP avatar up to 2 MiB", http.StatusBadRequest)
		return
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxAvatarBytes+1))
	if err != nil {
		http.Error(w, "could not read avatar", http.StatusBadRequest)
		return
	}
	if len(content) == 0 || len(content) > maxAvatarBytes {
		http.Error(w, "avatar must be no larger than 2 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		http.Error(w, "invalid image file", http.StatusBadRequest)
		return
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 ||
		int64(config.Width)*int64(config.Height) > 16_000_000 {
		http.Error(w, "avatar dimensions must not exceed 4096 by 4096 pixels", http.StatusBadRequest)
		return
	}
	extensions := map[string]string{"jpeg": ".jpg", "png": ".png", "webp": ".webp"}
	extension, ok := extensions[format]
	if !ok {
		http.Error(w, "only JPG, PNG, and WebP avatars are supported", http.StatusBadRequest)
		return
	}

	nameBytes := make([]byte, 16)
	if _, err := rand.Read(nameBytes); err != nil {
		slog.ErrorContext(r.Context(), "generate avatar filename", "error", err)
		http.Error(w, "could not save avatar", http.StatusInternalServerError)
		return
	}
	filename := hex.EncodeToString(nameBytes) + extension
	if err := os.MkdirAll(c.avatarDir, 0750); err != nil {
		slog.ErrorContext(r.Context(), "create avatar directory", "error", err)
		http.Error(w, "could not save avatar", http.StatusInternalServerError)
		return
	}
	path := filepath.Join(c.avatarDir, filename)
	avatarFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
	if err != nil {
		slog.ErrorContext(r.Context(), "save avatar", "error", err)
		http.Error(w, "could not save avatar", http.StatusInternalServerError)
		return
	}
	if _, err := avatarFile.Write(content); err != nil {
		_ = avatarFile.Close()
		_ = os.Remove(path)
		slog.ErrorContext(r.Context(), "write avatar", "error", err)
		http.Error(w, "could not save avatar", http.StatusInternalServerError)
		return
	}
	if err := avatarFile.Close(); err != nil {
		_ = os.Remove(path)
		slog.ErrorContext(r.Context(), "close avatar", "error", err)
		http.Error(w, "could not save avatar", http.StatusInternalServerError)
		return
	}

	avatarURL := "/uploads/avatars/" + filename
	oldAvatarURL, err := c.service.UpdateAvatar(r.Context(), userID, avatarURL)
	if err != nil {
		_ = os.Remove(path)
		c.writeSocialError(w, r, err)
		return
	}
	removeOldAvatar(c.avatarDir, oldAvatarURL)
	profile, err := c.service.GetProfile(r.Context(), userID)
	if err != nil {
		c.writeSocialError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewProfileResponse(profile, true))
}

func (c *SocialController) follows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	authors, err := c.service.ListFollowedAuthors(r.Context(), userID)
	if err != nil {
		c.writeSocialError(w, r, err)
		return
	}
	response := make([]dto.FollowedAuthorResponse, 0, len(authors))
	for _, author := range authors {
		response = append(response, dto.FollowedAuthorResponse{
			ID: author.ID, DisplayName: author.DisplayName, AvatarURL: author.AvatarURL,
			ArticleCount: author.ArticleCount, FollowersCount: author.FollowersCount,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func (c *SocialController) follow(w http.ResponseWriter, r *http.Request) {
	followerID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	authorID, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/follows/"), 10, 64)
	if err != nil || authorID <= 0 {
		http.Error(w, "invalid author ID", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPost:
		err = c.service.Follow(r.Context(), followerID, authorID)
	case http.MethodDelete:
		err = c.service.Unfollow(r.Context(), followerID, authorID)
	default:
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		c.writeSocialError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *SocialController) publicationPlan(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		plan, err := c.plans.PublicationPlan(r.Context(), userID)
		if err != nil {
			c.writeSocialError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, dto.PublicationPlanResponse{
			Active: plan.Active, Pending: plan.Pending, OrderID: plan.OrderID,
			DurationMonths: plan.DurationMonths, Status: plan.Status, UpdatedAt: plan.UpdatedAt,
			SubscriptionExpiresAt: plan.SubscriptionExpiresAt,
		})
	case http.MethodPost:
		var input dto.CreatePublicationPlanRequest
		if err := decodeJSON(w, r, &input); err != nil {
			http.Error(w, "invalid JSON request", http.StatusBadRequest)
			return
		}
		order, err := c.plans.CreatePublicationPlan(r.Context(), userID, input.DurationMonths)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrPublicationPlanPending):
				http.Error(w, "publication plan is already active or being processed", http.StatusConflict)
			case errors.Is(err, services.ErrInvalidPlanDuration):
				http.Error(w, "subscription duration must be 1 or 12 months", http.StatusBadRequest)
			case errors.Is(err, services.ErrOrderDispatch):
				http.Error(w, "plan request was saved but could not be sent for processing", http.StatusServiceUnavailable)
			default:
				slog.ErrorContext(r.Context(), "create publication plan request", "error", err)
				http.Error(w, "could not request publication plan", http.StatusInternalServerError)
			}
			return
		}
		writeJSON(w, http.StatusAccepted, dto.PublicationPlanResponse{
			Pending: true, OrderID: order.ID, DurationMonths: input.DurationMonths,
			Status: order.Status, UpdatedAt: order.UpdatedAt,
		})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *SocialController) writeSocialError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, repositories.ErrArticleNotFound), errors.Is(err, repositories.ErrProfileNotFound):
		http.NotFound(w, r)
	case errors.Is(err, repositories.ErrCannotFollowSelf):
		http.Error(w, "you cannot follow your own profile", http.StatusBadRequest)
	case errors.Is(err, services.ErrInvalidComment):
		http.Error(w, "comment must contain 1 to 2000 characters", http.StatusBadRequest)
	case errors.Is(err, services.ErrInvalidReaction):
		http.Error(w, "reaction must be like or dislike", http.StatusBadRequest)
	case errors.Is(err, services.ErrInvalidProfile):
		http.Error(w, "invalid profile", http.StatusBadRequest)
	default:
		slog.ErrorContext(r.Context(), "social request", "error", err)
		http.Error(w, "could not complete request", http.StatusInternalServerError)
	}
}

func removeOldAvatar(avatarDir, avatarURL string) {
	const prefix = "/uploads/avatars/"
	if !strings.HasPrefix(avatarURL, prefix) {
		return
	}
	filename := strings.TrimPrefix(avatarURL, prefix)
	if filename == "" || filepath.Base(filename) != filename {
		return
	}
	if err := os.Remove(filepath.Join(avatarDir, filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("remove replaced avatar", "error", fmt.Errorf("remove old avatar: %w", err))
	}
}
