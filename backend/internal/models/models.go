package models

import (
	"time"
)

type News struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	Content     string     `json:"content"`
	URL         string     `json:"url"`
	ImageURL    *string    `json:"image_url"`
	PublishedAt *time.Time `json:"published_at"`
	Lang        string     `json:"lang"`
	FeedID      int64      `json:"feed_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type NewsWithTranslations struct {
	ID                int64     `json:"id"`
	Title             string    `json:"title"`
	Summary           string    `json:"summary"`
	Content           string    `json:"content"`
	URL               string    `json:"url"`
	ImageURL          *string   `json:"image_url"`
	PublishedAt       *string   `json:"published_at"`
	Lang              string    `json:"lang"`
	FeedID            int64     `json:"feed_id"`
	CategoryID        *int64    `json:"category_id"`
	CountryID         *int64    `json:"country_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	TitleTranslated   *string   `json:"title_translated"`
	SummaryTranslated *string   `json:"summary_translated"`
	ContentTranslated *string   `json:"content_translated"`
	LangTranslated    *string   `json:"lang_translated"`
}

type Feed struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	SiteURL     *string    `json:"site_url"`
	Description *string    `json:"description"`
	ImageURL    *string    `json:"image_url"`
	Language    *string    `json:"language"`
	CategoryID  *int64     `json:"category_id"`
	CountryID   *int64     `json:"country_id"`
	Active      bool       `json:"active"`
	LastFetched *time.Time `json:"last_fetched"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Category struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Icon     string `json:"icon"`
	ParentID *int64 `json:"parent_id"`
}

type Country struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Continent string `json:"continent"`
	FlagEmoji string `json:"flag_emoji"`
}

type Translation struct {
	ID        int64     `json:"id"`
	NewsID    int64     `json:"news_id"`
	LangFrom  string    `json:"lang_from"`
	LangTo    string    `json:"lang_to"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Status    string    `json:"status"`
	Error     *string   `json:"error"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	IsAdmin      bool      `json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SearchHistory struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Query        string    `json:"query"`
	CategoryID   *int64    `json:"category_id"`
	CountryID    *int64    `json:"country_id"`
	ResultsCount int       `json:"results_count"`
	SearchedAt   time.Time `json:"searched_at"`
}

type NewsListResponse struct {
	News       []NewsWithTranslations `json:"news"`
	Total      int                    `json:"total"`
	Page       int                    `json:"page"`
	PerPage    int                    `json:"per_page"`
	TotalPages int                    `json:"total_pages"`
}

type FeedListResponse struct {
	Feeds      []Feed `json:"feeds"`
	Total      int    `json:"total"`
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	TotalPages int    `json:"total_pages"`
}

type Stats struct {
	TotalNews       int64          `json:"total_news"`
	TotalFeeds      int64          `json:"total_feeds"`
	TotalUsers      int64          `json:"total_users"`
	TotalTranslated int64          `json:"total_translated"`
	NewsToday       int64          `json:"news_today"`
	NewsThisWeek    int64          `json:"news_this_week"`
	NewsThisMonth   int64          `json:"news_this_month"`
	TopCategories   []CategoryStat `json:"top_categories"`
	TopCountries    []CountryStat  `json:"top_countries"`
}

type CategoryStat struct {
	CategoryID   int64  `json:"category_id"`
	CategoryName string `json:"category_name"`
	Count        int64  `json:"count"`
}

type CountryStat struct {
	CountryID   int64  `json:"country_id"`
	CountryName string `json:"country_name"`
	FlagEmoji   string `json:"flag_emoji"`
	Count       int64  `json:"count"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
}

type AuthResponse struct {
	Token       string `json:"token"`
	User        User   `json:"user"`
	IsFirstUser bool   `json:"is_first_user,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}
