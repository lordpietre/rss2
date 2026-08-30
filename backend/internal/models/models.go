package models

import (
	"time"
)

type News struct {
	ID          int64      `json:"id"`
	Titulo      string     `json:"titulo"`
	Resumen     string     `json:"resumen"`
	Contenido   string     `json:"contenido"`
	URL         string     `json:"url"`
	ImagenURL   *string    `json:"imagen_url"`
	Fecha       *time.Time `json:"fecha"`
	Lang        string     `json:"lang"`
	FeedID      int64      `json:"feed_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type NewsWithTranslations struct {
	ID                int64     `json:"id"`
	Titulo            string    `json:"titulo"`
	Resumen           string    `json:"resumen"`
	Contenido         string    `json:"contenido"`
	URL               string    `json:"url"`
	ImagenURL         *string   `json:"imagen_url"`
	Fecha             *string   `json:"fecha"`
	Lang              string    `json:"lang"`
	FeedID            int64     `json:"feed_id"`
	CategoryID        *int64    `json:"categoria_id"`
	CountryID         *int64    `json:"pais_id"`
	FuenteNombre      string    `json:"fuente_nombre"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	TitleTranslated   *string   `json:"title_translated"`
	SummaryTranslated *string   `json:"summary_translated"`
	ContentTranslated *string   `json:"content_translated"`
	LangTranslated    *string   `json:"lang_translated"`
	Entities          []Entity  `json:"entities,omitempty"`
}

type Entity struct {
	Valor       string  `json:"valor"`
	Tipo        string  `json:"tipo"`
	Apellido    string  `json:"apellido"`
	Count       int     `json:"count"`
	WikiSummary *string `json:"wiki_summary"`
	WikiURL     *string `json:"wiki_url"`
	ImagePath   *string `json:"image_path"`
}

type EntityListResponse struct {
	Entities   []Entity `json:"entities"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	PerPage    int      `json:"per_page"`
	TotalPages int      `json:"total_pages"`
}

type MentionPoint struct {
	Fecha string `json:"fecha"`
	Count int    `json:"count"`
}

type MentionSeries struct {
	Valor string         `json:"valor"`
	Tipo  string         `json:"tipo"`
	Count int            `json:"count"`
	Data  []MentionPoint `json:"data"`
}

type MentionsResponse struct {
	Days   int             `json:"days"`
	Series []MentionSeries `json:"series"`
}

type Feed struct {
	ID          int64      `json:"id"`
	Nombre      string     `json:"nombre"`
	URL         string     `json:"url"`
	SiteURL     *string    `json:"site_url"`
	Descripcion *string    `json:"descripcion"`
	ImagenURL   *string    `json:"imagen_url"`
	Idioma      *string    `json:"idioma"`
	CategoriaID *int64     `json:"categoria_id"`
	PaisID      *int64     `json:"pais_id"`
	Activo      bool       `json:"activo"`
	Fallos      *int64     `json:"fallos"`
	LastError   *string    `json:"last_error"`
	LastFetched *time.Time `json:"last_fetched"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Category struct {
	ID       int64  `json:"id"`
	Nombre   string `json:"nombre"`
	Color    string `json:"color"`
	Icon     string `json:"icon"`
	ParentID *int64 `json:"parent_id"`
}

type Country struct {
	ID         int64  `json:"id"`
	Nombre     string `json:"nombre"`
	Codigo     string `json:"codigo"`
	Continente string `json:"continente"`
	FlagEmoji  string `json:"flag_emoji"`
}

type Translation struct {
	ID        int64     `json:"id"`
	NoticiaID int64     `json:"noticia_id"`
	LangFrom  string    `json:"lang_from"`
	LangTo    string    `json:"lang_to"`
	Titulo    string    `json:"titulo"`
	Resumen   string    `json:"resumen"`
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
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SearchHistory struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Query        string    `json:"query"`
	CategoriaID  *int64    `json:"categoria_id"`
	PaisID       *int64    `json:"pais_id"`
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
	CategoriaID   int64  `json:"categoria_id"`
	CategoriaName string `json:"categoria_nombre"`
	Count         int64  `json:"count"`
}

type CountryStat struct {
	PaisID      int64  `json:"pais_id"`
	PaisName    string `json:"pais_nombre"`
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

type LastNameListResponse struct {
	LastNames []string `json:"last_names"`
	Total     int      `json:"total"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}
