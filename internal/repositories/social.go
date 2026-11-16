package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"my-awesome-project/internal/models"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrArticleNotFound  = errors.New("article not found")
	ErrProfileNotFound  = errors.New("profile not found")
	ErrCannotFollowSelf = errors.New("cannot follow yourself")
)

type SocialRepository struct {
	db *sql.DB
}

func NewSocialRepository(db *sql.DB) *SocialRepository {
	return &SocialRepository{db: db}
}

func (r *SocialRepository) GetProfile(ctx context.Context, userID int64) (models.Profile, error) {
	var profile models.Profile
	var avatarURL sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT u.id, u.email, u.display_name, u.bio, u.avatar_url,
		        (SELECT COUNT(*) FROM articles a WHERE a.author_id = u.id),
		        (SELECT COUNT(*) FROM author_follows f WHERE f.author_id = u.id),
		        (SELECT COUNT(*) FROM author_follows f WHERE f.follower_id = u.id)
		 FROM users u
		 WHERE u.id = $1`,
		userID,
	).Scan(&profile.ID, &profile.Email, &profile.DisplayName, &profile.Bio, &avatarURL,
		&profile.ArticleCount, &profile.FollowersCount, &profile.FollowingCount)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Profile{}, ErrProfileNotFound
	}
	if err != nil {
		return models.Profile{}, fmt.Errorf("get profile: %w", err)
	}
	profile.AvatarURL = avatarURL.String
	return profile, nil
}

func (r *SocialRepository) UpdateProfile(ctx context.Context, userID int64, displayName, bio string) (models.Profile, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE users SET display_name = $1, bio = $2 WHERE id = $3`,
		displayName, bio, userID,
	)
	if err != nil {
		return models.Profile{}, fmt.Errorf("update profile: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return models.Profile{}, fmt.Errorf("check updated profile: %w", err)
	}
	if updated == 0 {
		return models.Profile{}, ErrProfileNotFound
	}
	return r.GetProfile(ctx, userID)
}

func (r *SocialRepository) UpdateAvatar(ctx context.Context, userID int64, avatarURL string) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin avatar update: %w", err)
	}
	defer tx.Rollback()

	var oldURL sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT avatar_url FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&oldURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrProfileNotFound
	}
	if err != nil {
		return "", fmt.Errorf("update profile avatar: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET avatar_url = $1 WHERE id = $2`, avatarURL, userID); err != nil {
		return "", fmt.Errorf("save profile avatar: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit avatar update: %w", err)
	}
	return oldURL.String, nil
}

func (r *SocialRepository) CreateArticle(ctx context.Context, authorID int64, title, summary, body string) (models.Article, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Article{}, fmt.Errorf("begin article transaction: %w", err)
	}
	defer tx.Rollback()

	var lockedUserID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, authorID).Scan(&lockedUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Article{}, ErrProfileNotFound
		}
		return models.Article{}, fmt.Errorf("lock article author: %w", err)
	}

	var articleCount int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM articles WHERE author_id = $1`, authorID,
	).Scan(&articleCount); err != nil {
		return models.Article{}, fmt.Errorf("count published articles: %w", err)
	}
	var hasPublicationPlan bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM orders
			WHERE user_id = $1 AND order_type = 'publication_plan'
			  AND status = 'completed' AND subscription_expires_at > CURRENT_TIMESTAMP
		)`,
		authorID,
	).Scan(&hasPublicationPlan); err != nil {
		return models.Article{}, fmt.Errorf("check publication plan: %w", err)
	}
	if articleCount >= 3 && !hasPublicationPlan {
		return models.Article{}, models.ErrPublicationLimit
	}

	var article models.Article
	var avatarURL sql.NullString
	err = tx.QueryRowContext(ctx,
		`INSERT INTO articles (author_id, title, summary, body)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, author_id, title, summary, body, created_at`,
		authorID, title, summary, body,
	).Scan(&article.ID, &article.AuthorID, &article.Title, &article.Summary, &article.Body, &article.CreatedAt)
	if err != nil {
		return models.Article{}, fmt.Errorf("create article: %w", err)
	}
	if err := tx.QueryRowContext(ctx,
		`SELECT display_name, avatar_url FROM users WHERE id = $1`,
		authorID,
	).Scan(&article.AuthorName, &avatarURL); err != nil {
		return models.Article{}, fmt.Errorf("load article author: %w", err)
	}
	article.AuthorAvatar = avatarURL.String

	if err := tx.Commit(); err != nil {
		return models.Article{}, fmt.Errorf("commit article: %w", err)
	}
	return article, nil
}

func (r *SocialRepository) ListArticles(ctx context.Context, query string, authorID, viewerID int64, followedOnly bool) ([]models.Article, error) {
	var conditions []string
	var args []any
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if query != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
		placeholder := addArg("%" + escaped + "%")
		conditions = append(conditions, "(a.title ILIKE "+placeholder+" ESCAPE '\\' OR a.summary ILIKE "+placeholder+" ESCAPE '\\' OR a.body ILIKE "+placeholder+" ESCAPE '\\')")
	}
	if authorID > 0 {
		conditions = append(conditions, "a.author_id = "+addArg(authorID))
	}
	if followedOnly {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM author_follows f WHERE f.follower_id = "+addArg(viewerID)+" AND f.author_id = a.author_id)")
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	viewerPlaceholder := addArg(viewerID)
	statement := `SELECT a.id, a.author_id, u.display_name, COALESCE(u.avatar_url, ''),
	                     a.title, a.summary, '', a.created_at,
	                     (SELECT COUNT(*) FROM article_reactions r WHERE r.article_id = a.id AND r.reaction = 'like'),
	                     (SELECT COUNT(*) FROM article_reactions r WHERE r.article_id = a.id AND r.reaction = 'dislike'),
	                     (SELECT COUNT(*) FROM article_comments c WHERE c.article_id = a.id),
	                     COALESCE((SELECT r.reaction FROM article_reactions r WHERE r.article_id = a.id AND r.user_id = ` + viewerPlaceholder + `), '')
	              FROM articles a
	              JOIN users u ON u.id = a.author_id ` + where + `
	              ORDER BY a.created_at DESC, a.id DESC
	              LIMIT 100`

	rows, err := r.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	defer rows.Close()
	articles := make([]models.Article, 0)
	for rows.Next() {
		var article models.Article
		if err := rows.Scan(&article.ID, &article.AuthorID, &article.AuthorName, &article.AuthorAvatar,
			&article.Title, &article.Summary, &article.Body, &article.CreatedAt,
			&article.Likes, &article.Dislikes, &article.CommentCount, &article.MyReaction); err != nil {
			return nil, fmt.Errorf("scan article: %w", err)
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate articles: %w", err)
	}
	return articles, nil
}

func (r *SocialRepository) GetArticle(ctx context.Context, articleID, viewerID int64) (models.Article, error) {
	var article models.Article
	err := r.db.QueryRowContext(ctx,
		`SELECT a.id, a.author_id, u.display_name, COALESCE(u.avatar_url, ''),
		        a.title, a.summary, a.body, a.created_at,
		        (SELECT COUNT(*) FROM article_reactions r WHERE r.article_id = a.id AND r.reaction = 'like'),
		        (SELECT COUNT(*) FROM article_reactions r WHERE r.article_id = a.id AND r.reaction = 'dislike'),
		        (SELECT COUNT(*) FROM article_comments c WHERE c.article_id = a.id),
		        COALESCE((SELECT r.reaction FROM article_reactions r WHERE r.article_id = a.id AND r.user_id = $2), '')
		 FROM articles a
		 JOIN users u ON u.id = a.author_id
		 WHERE a.id = $1`,
		articleID, viewerID,
	).Scan(&article.ID, &article.AuthorID, &article.AuthorName, &article.AuthorAvatar,
		&article.Title, &article.Summary, &article.Body, &article.CreatedAt,
		&article.Likes, &article.Dislikes, &article.CommentCount, &article.MyReaction)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Article{}, ErrArticleNotFound
	}
	if err != nil {
		return models.Article{}, fmt.Errorf("get article: %w", err)
	}
	return article, nil
}

func (r *SocialRepository) ListComments(ctx context.Context, articleID int64) ([]models.Comment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT c.id, c.article_id, c.author_id, u.display_name, COALESCE(u.avatar_url, ''), c.body, c.created_at
		 FROM article_comments c
		 JOIN users u ON u.id = c.author_id
		 WHERE c.article_id = $1
		 ORDER BY c.created_at, c.id`,
		articleID,
	)
	if err != nil {
		return nil, fmt.Errorf("list article comments: %w", err)
	}
	defer rows.Close()
	comments := make([]models.Comment, 0)
	for rows.Next() {
		var comment models.Comment
		if err := rows.Scan(&comment.ID, &comment.ArticleID, &comment.AuthorID, &comment.AuthorName,
			&comment.AvatarURL, &comment.Body, &comment.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan article comment: %w", err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate article comments: %w", err)
	}
	return comments, nil
}

func (r *SocialRepository) CreateComment(ctx context.Context, articleID, authorID int64, body string) (models.Comment, error) {
	var comment models.Comment
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO article_comments (article_id, author_id, body)
		 VALUES ($1, $2, $3)
		 RETURNING id, article_id, author_id, body, created_at`,
		articleID, authorID, body,
	).Scan(&comment.ID, &comment.ArticleID, &comment.AuthorID, &comment.Body, &comment.CreatedAt)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23503" &&
			postgresError.ConstraintName == "article_comments_article_id_fkey" {
			return models.Comment{}, ErrArticleNotFound
		}
		return models.Comment{}, fmt.Errorf("create article comment: %w", err)
	}
	if err := r.db.QueryRowContext(ctx,
		`SELECT display_name, COALESCE(avatar_url, '') FROM users WHERE id = $1`,
		authorID,
	).Scan(&comment.AuthorName, &comment.AvatarURL); err != nil {
		return models.Comment{}, fmt.Errorf("load comment author: %w", err)
	}
	return comment, nil
}

func (r *SocialRepository) ToggleReaction(ctx context.Context, articleID, userID int64, reaction string) (string, error) {
	var current string
	err := r.db.QueryRowContext(ctx,
		`WITH removed AS (
		    DELETE FROM article_reactions
		    WHERE article_id = $1 AND user_id = $2 AND reaction = $3
		    RETURNING reaction
		 ), upserted AS (
		    INSERT INTO article_reactions (article_id, user_id, reaction)
		    SELECT $1, $2, $3
		    WHERE NOT EXISTS (SELECT 1 FROM removed)
		    ON CONFLICT (article_id, user_id)
		    DO UPDATE SET reaction = EXCLUDED.reaction, created_at = CURRENT_TIMESTAMP
		    RETURNING reaction
		 )
		 SELECT COALESCE((SELECT reaction FROM upserted), '')`,
		articleID, userID, reaction,
	).Scan(&current)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23503" &&
			postgresError.ConstraintName == "article_reactions_article_id_fkey" {
			return "", ErrArticleNotFound
		}
		return "", fmt.Errorf("toggle article reaction: %w", err)
	}
	return current, nil
}

func (r *SocialRepository) Follow(ctx context.Context, followerID, authorID int64) error {
	if followerID == authorID {
		return ErrCannotFollowSelf
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO author_follows (follower_id, author_id)
		 VALUES ($1, $2)
		 ON CONFLICT (follower_id, author_id) DO NOTHING`,
		followerID, authorID,
	)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23503" &&
			postgresError.ConstraintName == "author_follows_author_id_fkey" {
			return ErrProfileNotFound
		}
		return fmt.Errorf("follow author: %w", err)
	}
	return nil
}

func (r *SocialRepository) Unfollow(ctx context.Context, followerID, authorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM author_follows WHERE follower_id = $1 AND author_id = $2`,
		followerID, authorID,
	)
	if err != nil {
		return fmt.Errorf("unfollow author: %w", err)
	}
	return nil
}

func (r *SocialRepository) ListFollowedAuthors(ctx context.Context, followerID int64) ([]models.FollowedAuthor, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT u.id, u.display_name, COALESCE(u.avatar_url, ''),
		        (SELECT COUNT(*) FROM articles a WHERE a.author_id = u.id),
		        (SELECT COUNT(*) FROM author_follows f WHERE f.author_id = u.id)
		 FROM author_follows followed
		 JOIN users u ON u.id = followed.author_id
		 WHERE followed.follower_id = $1
		 ORDER BY followed.created_at DESC`,
		followerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list followed authors: %w", err)
	}
	defer rows.Close()
	authors := make([]models.FollowedAuthor, 0)
	for rows.Next() {
		var author models.FollowedAuthor
		if err := rows.Scan(&author.ID, &author.DisplayName, &author.AvatarURL,
			&author.ArticleCount, &author.FollowersCount); err != nil {
			return nil, fmt.Errorf("scan followed author: %w", err)
		}
		authors = append(authors, author)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate followed authors: %w", err)
	}
	return authors, nil
}
