package models

import "time"

type RemoteWorker struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	APIKey       string    `json:"api_key,omitempty"`
	Capabilities string    `json:"capabilities"`
	Status       string    `json:"status"`
	LastSeen     *time.Time `json:"last_seen,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type TranslationJob struct {
	ID        int64  `json:"id"`
	NewsID    int64  `json:"noticia_id"`
	LangFrom  string `json:"lang_from"`
	LangTo    string `json:"lang_to"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
}

type TranslationResult struct {
	JobID      int64  `json:"job_id"`
	TitleTr   string `json:"title_trad"`
	SummaryTr string `json:"resumen_trad"`
	Error     string `json:"error,omitempty"`
}

type WSClientMessage struct {
	Type         string              `json:"type"`
	Capabilities string              `json:"capabilities,omitempty"`
	WorkerName  string              `json:"worker_name,omitempty"`
	Result      *TranslationResult  `json:"result,omitempty"`
}

type WSServerMessage struct {
	Type string          `json:"type"`
	Job  *TranslationJob `json:"job,omitempty"`
}