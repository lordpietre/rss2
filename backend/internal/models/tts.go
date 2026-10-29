package models

import "time"

// TTSVoice describes un perfil de timbre utilizable por el motor de voz.
// Engine "chatterbox" = voz por defecto o clon con audio de referencia.
// Engine "melotts" = voz base del motor. Engine "openvoice" = conversion de
// timbre con un audio de referencia.
type TTSVoice struct {
	ID              int                    `json:"id"`
	Slug            string                 `json:"slug"`
	DisplayName     string                 `json:"display_name"`
	Engine          string                 `json:"engine"`
	Language        string                 `json:"language"`
	SpeakerID       *string                `json:"speaker_id"`
	DurationSeconds *float64               `json:"duration_seconds"`
	SampleRate      *int                   `json:"sample_rate"`
	QualityStatus   string                 `json:"quality_status"`
	QualityReport   map[string]interface{} `json:"quality_report,omitempty"`
	IsBuiltin       bool                   `json:"is_builtin"`
	CreatedAt       time.Time              `json:"created_at"`
}

// TTSScript es el guion editorial que se va a locutar. RawContent guarda el
// texto tal cual venia de la noticia; Content la version normalizada.
type TTSScript struct {
	ID            int64      `json:"id"`
	NoticiaID     *string    `json:"noticia_id"`
	TranslationID *int64     `json:"translation_id"`
	Language      string     `json:"language"`
	Title         *string    `json:"title"`
	RawContent    *string    `json:"raw_content"`
	Content       string     `json:"content"`
	ContentLength *int       `json:"content_length"`
	SourceTitle   *string    `json:"source_title"`
	SourceURL     *string    `json:"source_url"`
	SourceName    *string    `json:"source_name"`
	Status        string     `json:"status"`
	CreatedBy     *int64     `json:"created_by"`
	ApprovedBy    *int64     `json:"approved_by"`
	ApprovedAt    *time.Time `json:"approved_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TTSJob es una tarea asincrona de sintesis.
type TTSJob struct {
	ID           int64      `json:"id"`
	ScriptID     int64      `json:"script_id"`
	VoiceID      *int64     `json:"voice_id"`
	Engine       string     `json:"engine"`
	Speed        float64    `json:"speed"`
	Status       string     `json:"status"`
	Attempts     int        `json:"attempts"`
	MaxAttempts  int        `json:"max_attempts"`
	ErrorCode    *string    `json:"error_code"`
	ErrorMessage *string    `json:"error_message"`
	QueuedAt     time.Time  `json:"queued_at"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	// ElapsedSeconds es el tiempo en marcha (solo en tareas 'running').
	ElapsedSeconds int       `json:"elapsed_seconds"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Audio          *TTSAudio `json:"audio,omitempty"`
}

// TTSAudio es el resultado de una sintesis. El archivo vive en el volumen
// persistente; aqui solo se guarda la ruta relativa.
type TTSAudio struct {
	ID              int64                  `json:"id"`
	JobID           int64                  `json:"job_id"`
	ScriptID        int64                  `json:"script_id"`
	VoiceID         *int64                 `json:"voice_id"`
	Engine          string                 `json:"engine"`
	EngineVersion   *string                `json:"engine_version"`
	StoragePath     string                 `json:"storage_path"`
	Format          string                 `json:"format"`
	DurationSeconds *float64               `json:"duration_seconds"`
	SampleRate      *int                   `json:"sample_rate"`
	SizeBytes       *int64                 `json:"size_bytes"`
	SHA256          *string                `json:"sha256"`
	Parameters      map[string]interface{} `json:"parameters,omitempty"`
	Status          string                 `json:"status"`
	ApprovedBy      *int64                 `json:"approved_by"`
	ApprovedAt      *time.Time             `json:"approved_at"`
	CreatedAt       time.Time              `json:"created_at"`
	NoticiaID       *string                `json:"noticia_id,omitempty"`
	SourceURL       *string                `json:"source_url,omitempty"`
	SourceTitle     *string                `json:"source_title,omitempty"`
	SourceName      *string                `json:"source_name,omitempty"`
	ScriptTitle     *string                `json:"script_title,omitempty"`
}

// TTSVoiceListResponse envuelve la lista de voces.
type TTSVoiceListResponse struct {
	Voices []TTSVoice `json:"voices"`
}

// TTSScriptListResponse envuelve la lista de guiones.
type TTSScriptListResponse struct {
	Scripts []TTSScript `json:"scripts"`
	Total   int         `json:"total"`
}

// TTSJobListResponse envuelve la lista de tareas.
type TTSJobListResponse struct {
	Jobs []TTSJob `json:"jobs"`
}

// TTSAudioListResponse envuelve la lista de audios.
type TTSAudioListResponse struct {
	Audios []TTSAudio `json:"audios"`
}

// TTSJobCreatedResponse responde a la creacion de una tarea. Created es
// false cuando la clave de idempotencia ya existia.
type TTSJobCreatedResponse struct {
	Job     TTSJob `json:"job"`
	Created bool   `json:"created"`
}

// TTSBulletin es un noticiario: varias narraciones unidas en un solo audio.
// Status: pending (faltan piezas) | ready | failed | approved | rejected.
type TTSBulletin struct {
	ID              int64             `json:"id"`
	Title           string            `json:"title"`
	Language        string            `json:"language"`
	Status          string            `json:"status"`
	VoiceID         *int64            `json:"voice_id"`
	VoiceSlug       string            `json:"voice_slug"`
	Engine          string            `json:"engine"`
	Speed           float64           `json:"speed"`
	AudioPath       *string           `json:"audio_path"`
	DurationSeconds *float64          `json:"duration_seconds"`
	SizeBytes       *int64            `json:"size_bytes"`
	SHA256          *string           `json:"sha256"`
	ItemCount       int               `json:"item_count"`
	SilenceMs       int               `json:"silence_ms"`
	CreatedBy       *int64            `json:"created_by"`
	ApprovedBy      *int64            `json:"approved_by"`
	ApprovedAt      *time.Time        `json:"approved_at"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Items           []TTSBulletinItem `json:"items,omitempty"`
}

// TTSBulletinItem es una noticia dentro de un noticiario. StartOffset indica
// donde empieza dentro del audio final.
type TTSBulletinItem struct {
	ID              int64    `json:"id"`
	BulletinID      int64    `json:"bulletin_id"`
	ScriptID        int64    `json:"script_id"`
	Position        int      `json:"position"`
	AudioID         *int64   `json:"audio_id"`
	DurationSeconds *float64 `json:"duration_seconds"`
	StartOffset     *float64 `json:"start_offset"`
	SourceURL       *string  `json:"source_url"`
	SourceName      *string  `json:"source_name"`
	SourceTitle     *string  `json:"source_title"`
	NoticiaID       *string  `json:"noticia_id"`
	ScriptTitle     *string  `json:"script_title"`
	ScriptStatus    *string  `json:"script_status"`
}

// TTSBulletinListResponse envuelve la lista de noticiarios.
type TTSBulletinListResponse struct {
	Bulletins []TTSBulletin `json:"bulletins"`
}
