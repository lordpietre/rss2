package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/cache"
	"github.com/rss2/backend/internal/config"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
	"github.com/rss2/backend/internal/services"
)

// =============================================================================
// API de narracion (TTS)
//
// Flujo editorial:
//   1. POST /tts/scripts      -> se crea un guion (borrador) desde una noticia
//   2. POST /tts/scripts/:id/approve -> un editor aprueba el guion
//   3. POST /tts/jobs         -> se encola la sintesis (idempotente)
//   4. GET  /tts/jobs/:id     -> se consulta el estado
//   5. GET  /tts/audios/:id   -> metadatos del audio
//   6. POST /tts/audios/:id/approve -> el editor aprueba el audio
//
// Nada se publica automaticamente: el audio queda 'pending_review'.
// =============================================================================

const (
	ttsScriptStatusDraft    = "draft"
	ttsScriptStatusApproved = "approved"
	ttsScriptStatusRejected = "rejected"

	ttsJobStatusQueued   = "queued"
	ttsJobStatusRunning  = "running"
	ttsAudioPending      = "pending_review"
	ttsAudioStatusApprov = "approved"
	ttsAudioStatusReject = "rejected"

	// Limite de la muestra de voz que se puede registrar (50 MB).
	ttsMaxVoiceFileBytes = 50 * 1024 * 1024
)

// -----------------------------------------------------------------------------
// Estado del motor
// -----------------------------------------------------------------------------

// GetTTSStatus devuelve el estado del motor de voz.
// @Summary Estado del motor de TTS
// @Tags tts
// @Produce json
// @Success 200 {object} services.TTSHealthResponse
// @Failure 503 {object} models.ErrorResponse
// @Router /tts/status [get]
func GetTTSStatus(c *gin.Context) {
	ctx := c.Request.Context()
	health, err := services.TTSHealth(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
			Error:   "TTS service unavailable",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, health)
}

// -----------------------------------------------------------------------------
// Voces
// -----------------------------------------------------------------------------

// ListTTSSpeakers devuelve los hablantes del modelo base.
// @Summary Hablantes disponibles en el modelo
// @Tags tts
// @Produce json
// @Param engine query string false "Motor (el configurado en TTS_ENGINE por defecto)"
// @Success 200 {object} services.TTSVoicesResponse
// @Failure 503 {object} models.ErrorResponse
// @Router /tts/speakers [get]
func ListTTSSpeakers(c *gin.Context) {
	result, err := services.TTSModelSpeakers(c.Request.Context(), c.Query("engine"))
	if err != nil {
		log.Printf("ListTTSSpeakers: %v", err)
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
			Error:   "TTS service unavailable",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

// RegisterTTSSpeakerRequest pide registrar un hablante del modelo como voz.
type RegisterTTSSpeakerRequest struct {
	SpeakerID   int    `json:"speaker_id"`
	Engine      string `json:"engine"`
	DisplayName string `json:"display_name"`
}

// RegisterTTSSpeaker registra un hablante del modelo como voz utilizable.
//
// El modelo en español tiene 256 hablantes: cada uno es una voz distinta.
// Registrarlos bajo demanda evita ensuciar el catalogo con 256 filas.
// @Summary Registra un hablante del modelo como voz
// @Tags tts
// @Accept json
// @Produce json
// @Param request body RegisterTTSSpeakerRequest true "Hablante"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Failure 503 {object} models.ErrorResponse
// @Router /tts/speakers [post]
func RegisterTTSSpeaker(c *gin.Context) {
	var req RegisterTTSSpeakerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}
	if req.SpeakerID < 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid speaker ID"})
		return
	}

	result, err := services.TTSRegisterSpeaker(
		c.Request.Context(),
		req.SpeakerID,
		req.Engine,
		req.DisplayName,
	)
	if err != nil {
		log.Printf("RegisterTTSSpeaker: %v", err)
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
			Error:   "Could not register speaker",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// ListTTSVoice devuelve las voces registradas.
// @Summary Lista de voces
// @Tags tts
// @Produce json
// @Success 200 {object} models.TTSVoiceListResponse
// @Router /tts/voices [get]
func ListTTSVoice(c *gin.Context) {
	rows, err := db.GetPool().Query(c.Request.Context(), `
		SELECT id, slug, display_name, engine, TRIM(language), speaker_id,
		       duration_seconds, sample_rate, quality_status, quality_report,
		       is_builtin, created_at
		FROM tts_voices
		ORDER BY is_builtin DESC, slug`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch voices", Message: err.Error()})
		return
	}
	defer rows.Close()

	voices := []models.TTSVoice{}
	for rows.Next() {
		var v models.TTSVoice
		var language string
		if err := rows.Scan(&v.ID, &v.Slug, &v.DisplayName, &v.Engine, &language,
			&v.SpeakerID, &v.DurationSeconds, &v.SampleRate, &v.QualityStatus,
			&v.QualityReport, &v.IsBuiltin, &v.CreatedAt); err != nil {
			continue
		}
		v.Language = language
		voices = append(voices, v)
	}

	c.JSON(http.StatusOK, models.TTSVoiceListResponse{Voices: voices})
}

// CreateTTSVoice registra una muestra de voz propia.
// La muestra se guarda en el volumen del servicio de TTS, nunca en el
// backend ni en un directorio publicado por nginx.
// @Summary Registra una muestra de voz
// @Tags tts
// @Accept mp3
// @Produce json
// @Param sample formData file true "Audio de referencia"
// @Param slug formData string true "Identificador de la voz"
// @Param display_name formData string false "Nombre legible"
// @Param language formData string false "Idioma (por defecto es)"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Router /tts/voices [post]
func CreateTTSVoice(c *gin.Context) {
	file, header, err := c.Request.FormFile("sample")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Missing sample file", Message: err.Error()})
		return
	}
	defer file.Close()

	if header.Size > ttsMaxVoiceFileBytes {
		c.JSON(http.StatusRequestEntityTooLarge, models.ErrorResponse{
			Error: "Sample file too large",
		})
		return
	}

	slug := strings.TrimSpace(c.PostForm("slug"))
	if slug == "" {
		slug = strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	}
	if len(slug) > 64 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Slug too long"})
		return
	}

	displayName := strings.TrimSpace(c.PostForm("display_name"))
	if displayName == "" {
		displayName = slug
	}
	if len(displayName) > 128 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Display name too long"})
		return
	}

	language := strings.TrimSpace(c.PostForm("language"))
	if language == "" {
		language = "es"
	}

	// Motor de la voz: vacio = el por defecto del servicio (Chatterbox).
	engine := strings.TrimSpace(c.PostForm("engine"))

	result, err := services.TTSCreateVoice(c.Request.Context(), file, header, slug, displayName, language, engine)
	if err != nil {
		log.Printf("CreateTTSVoice: %v", err)
		c.JSON(http.StatusBadGateway, models.ErrorResponse{Error: "Failed to register voice", Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// DeleteTTSVoice elimina una voz y su muestra de voz.
// @Summary Elimina una voz
// @Tags tts
// @Param id path int true "ID de la voz"
// @Success 200 {object} models.SuccessResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/voices/{id} [delete]
func DeleteTTSVoice(c *gin.Context) {
	voiceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid voice ID"})
		return
	}

	// La voz debe existir en la base antes de borrar el fichero.
	var referenceFile *string
	err = db.GetPool().QueryRow(c.Request.Context(),
		`SELECT reference_file FROM tts_voices WHERE id = $1`, voiceID).Scan(&referenceFile)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Voice not found"})
		return
	}

	if err := services.TTSDeleteVoice(c.Request.Context(), voiceID); err != nil {
		log.Printf("DeleteTTSVoice: %v", err)
		c.JSON(http.StatusBadGateway, models.ErrorResponse{Error: "Failed to delete voice", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "Voice deleted"})
}

// GetTTSVoiceQuality devuelve el informe de calidad de una muestra de voz.
// @Summary Calidad de una muestra de voz
// @Tags tts
// @Produce json
// @Param id path int true "ID de la voz"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/voices/{id}/quality [get]
func GetTTSVoiceQuality(c *gin.Context) {
	voiceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid voice ID"})
		return
	}

	var exists bool
	err = db.GetPool().QueryRow(c.Request.Context(),
		`SELECT EXISTS(SELECT 1 FROM tts_voices WHERE id = $1)`, voiceID).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Voice not found"})
		return
	}

	report, err := services.TTSVoiceQuality(c.Request.Context(), voiceID)
	if err != nil {
		log.Printf("GetTTSVoiceQuality: %v", err)
		c.JSON(http.StatusBadGateway, models.ErrorResponse{Error: "Failed to get voice quality", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

// -----------------------------------------------------------------------------
// Guiones
// -----------------------------------------------------------------------------

// ListTTSScripts devuelve los guiones, filtrables por estado y noticia.
// @Summary Lista de guiones
// @Tags tts
// @Produce json
// @Param status query string false "draft, approved o rejected"
// @Param noticia_id query string false "ID de la noticia"
// @Param page query int false "Pagina"
// @Param per_page query int false "Elementos por pagina"
// @Success 200 {object} models.TTSScriptListResponse
// @Router /tts/scripts [get]
func ListTTSScripts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	page, perPage = validatePageParams(page, perPage, 50, 200)
	offset := (page - 1) * perPage

	where := "1=1"
	args := []interface{}{}
	argNum := 1

	if status := c.Query("status"); status != "" {
		where += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, status)
		argNum++
	}
	if noticiaID := c.Query("noticia_id"); noticiaID != "" {
		where += fmt.Sprintf(" AND noticia_id = $%d", argNum)
		args = append(args, noticiaID)
		argNum++
	}

	var total int
	if err := db.GetPool().QueryRow(c.Request.Context(),
		`SELECT COUNT(*) FROM tts_scripts WHERE `+where, args...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to count scripts", Message: err.Error()})
		return
	}

	args = append(args, perPage, offset)
	query := `
		SELECT id, noticia_id, translation_id, TRIM(language), title, raw_content,
		       content, source_title, source_url, source_name, status, created_by,
		       approved_by, approved_at, created_at, updated_at, length(content)
		FROM tts_scripts
		WHERE ` + where + `
		ORDER BY created_at DESC
		LIMIT $` + strconv.Itoa(argNum) + ` OFFSET $` + strconv.Itoa(argNum+1)

	rows, err := db.GetPool().Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch scripts", Message: err.Error()})
		return
	}
	defer rows.Close()

	scripts := []models.TTSScript{}
	for rows.Next() {
		var s models.TTSScript
		if err := rows.Scan(&s.ID, &s.NoticiaID, &s.TranslationID, &s.Language, &s.Title,
			&s.RawContent, &s.Content, &s.SourceTitle, &s.SourceURL, &s.SourceName,
			&s.Status, &s.CreatedBy, &s.ApprovedBy, &s.ApprovedAt, &s.CreatedAt,
			&s.UpdatedAt, &s.ContentLength); err != nil {
			continue
		}
		scripts = append(scripts, s)
	}

	c.JSON(http.StatusOK, models.TTSScriptListResponse{Scripts: scripts, Total: total})
}

// CreateTTSScriptRequest es la peticion para crear un guion.
type CreateTTSScriptRequest struct {
	NoticiaID string `json:"noticia_id"`
	Language  string `json:"language"`
	Title     string `json:"title"`
	Content   string `json:"content"`
}

// CreateTTSScript crea un guion (borrador) a partir de una noticia.
//
// Si no se indica contenido, se toma el texto editorial de la noticia en el
// idioma pedido (traduccion si existe). El texto se normaliza para locucion
// en el servicio de TTS; el original se conserva en raw_content.
// @Summary Crea un guion para narracion
// @Tags tts
// @Accept json
// @Produce json
// @Param request body CreateTTSScriptRequest true "Datos del guion"
// @Success 201 {object} models.TTSScript
// @Failure 400 {object} models.ErrorResponse
// @Failure 503 {object} models.ErrorResponse
// @Router /tts/scripts [post]
func CreateTTSScript(c *gin.Context) {
	var req CreateTTSScriptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	cfg := config.Load()
	language := strings.TrimSpace(req.Language)
	if language == "" {
		language = cfg.DefaultLang
	}
	language = strings.ToLower(language)
	if len(language) > 5 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid language"})
		return
	}

	noticiaID := strings.TrimSpace(req.NoticiaID)
	content := strings.TrimSpace(req.Content)
	title := strings.TrimSpace(req.Title)

	// Si viene el texto, se normaliza y se guarda directamente. Si no, se
	// toma de la noticia (preferentemente su traduccion).
	if content != "" {
		script, err := createTTSScript(c, content, title, language, nil, nil, nil, nil, nil)
		if err != nil {
			respondTTSScriptError(c, err)
			return
		}
		c.JSON(http.StatusCreated, script)
		return
	}

	if noticiaID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Empty script",
			Message: "Indica una noticia o el texto del guion",
		})
		return
	}

	script, err := createTTSScriptFromNews(c, noticiaID, language)
	if err != nil {
		respondTTSScriptError(c, err)
		return
	}

	c.JSON(http.StatusCreated, script)
}

// ttsScriptError identifica los fallos del servicio de TTS para responder con
// el codigo HTTP adecuado.
type ttsScriptError struct {
	code    int
	err     string
	message string
}

func (e *ttsScriptError) Error() string { return e.message }

// respondTTSScriptError traduce el error a una respuesta HTTP.
func respondTTSScriptError(c *gin.Context, err error) {
	var scriptErr *ttsScriptError
	if !errors.As(err, &scriptErr) {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create script", Message: err.Error()})
		return
	}
	c.JSON(scriptErr.code, models.ErrorResponse{Error: scriptErr.err, Message: scriptErr.message})
}

// resolveUserID devuelve el id del usuario si sigue existiendo en la tabla
// `users`, y NULL si no.
//
// El token JWT puede ser valido aunque la fila del usuario ya no exista
// (borrado, restauracion de una copia): sin esta comprobacion, los inserts
// fallarian con un error de clave foranea.
func resolveUserID(ctx context.Context, userID int) interface{} {
	if userID <= 0 {
		return nil
	}
	var exists bool
	if err := db.GetPool().QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil || !exists {
		return nil
	}
	return userID
}

// createTTSScript normaliza el texto y guarda el guion.
func createTTSScript(
	c *gin.Context,
	content, title, language string,
	noticiaID *string,
	translationID *int64,
	sourceTitle, sourceURL, sourceName *string,
) (*models.TTSScript, error) {
	cfg := config.Load()

	if len(content) > cfg.TTSMaxScriptChars {
		return nil, &ttsScriptError{
			code:    http.StatusRequestEntityTooLarge,
			err:     "Script too long",
			message: fmt.Sprintf("El guion supera el limite de %d caracteres", cfg.TTSMaxScriptChars),
		}
	}

	// La normalizacion vive en el servicio de TTS: una sola implementacion.
	normalized, err := services.TTSNormalizeScript(c.Request.Context(), content, language)
	if err != nil {
		log.Printf("createTTSScript: normalizacion fallida: %v", err)
		return nil, &ttsScriptError{
			code:    http.StatusServiceUnavailable,
			err:     "TTS service unavailable",
			message: "No se pudo preparar el guion para locucion",
		}
	}
	if strings.TrimSpace(normalized.Text) == "" {
		return nil, &ttsScriptError{code: http.StatusBadRequest, err: "Empty script after normalization"}
	}

	userID := resolveUserID(c.Request.Context(), c.GetInt("user_id"))
	var scriptID int64
	err = db.GetPool().QueryRow(c.Request.Context(), `
		INSERT INTO tts_scripts (noticia_id, translation_id, language, title,
		                         raw_content, content, source_title, source_url,
		                         source_name, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`,
		noticiaID, translationID, language, nullable(title),
		content, normalized.Text, sourceTitle, sourceURL, sourceName,
		ttsScriptStatusDraft, userID,
	).Scan(&scriptID)
	if err != nil {
		log.Printf("createTTSScript: %v", err)
		return nil, &ttsScriptError{code: http.StatusInternalServerError, err: "Failed to create script", message: err.Error()}
	}

	return fetchTTSScript(c.Request.Context(), scriptID)
}

// createTTSScriptFromNews crea un guion en borrador a partir de una noticia.
//
// El guion se compone del TITULAR seguido del cuerpo: el titular da el
// contexto de lo que se va a leer (y es lo que se ve en pantalla), y el
// cuerpo desarrolla la noticia.
func createTTSScriptFromNews(c *gin.Context, noticiaID, language string) (*models.TTSScript, error) {
	if len(noticiaID) > 32 {
		return nil, &ttsScriptError{code: http.StatusBadRequest, err: "Invalid news ID"}
	}

	source, err := fetchTTSSource(c.Request.Context(), noticiaID, language)
	if err != nil {
		return nil, &ttsScriptError{code: http.StatusNotFound, err: "News not found", message: err.Error()}
	}
	if source.Content == "" {
		return nil, &ttsScriptError{
			code:    http.StatusBadRequest,
			err:     "Empty script",
			message: "La noticia no tiene texto disponible en el idioma pedido",
		}
	}

	// Titular + cuerpo, evitando repetir el titular si el cuerpo ya empieza
	// con el (los resumenes de RSS suelen repetirlo).
	content := source.Content
	headline := strings.TrimSpace(source.Headline)
	if headline != "" && !strings.HasPrefix(strings.TrimSpace(content), headline) {
		content = headline + "\n\n" + content
	}

	var translationID *int64 = source.TranslationID
	return createTTSScript(
		c,
		content,
		source.Headline,
		language,
		&source.NoticiaID,
		translationID,
		source.Title,
		source.URL,
		source.Name,
	)
}

// PrepareTTSScriptsRequest pide los guiones de una lista de noticias.
type PrepareTTSScriptsRequest struct {
	NoticiaIDs []string `json:"noticia_ids"`
	Language   string   `json:"language"`
}

// TTSScriptPreview es el guion listo para revision: el texto completo que se
// va a locutar, con su referencia a la noticia de origen.
type TTSScriptPreview struct {
	ScriptID   int64  `json:"script_id"`
	NoticiaID  string `json:"noticia_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	SourceName string `json:"source_name"`
	SourceURL  string `json:"source_url"`
	Status     string `json:"status"`
	Existing   bool   `json:"existing"`
}

// PrepareTTSScripts deja listos los guiones de las noticias elegidas para que
// el editor pueda revisarlos y modificarlos antes de locutar.
//
// Si ya hay un borrador del mismo usuario para esa noticia, lo devuelve:
// repetir la operacion no debe crear guiones duplicados.
// @Summary Prepara los guiones para revision
// @Tags tts
// @Accept json
// @Produce json
// @Param request body PrepareTTSScriptsRequest true "Noticias elegidas"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Failure 503 {object} models.ErrorResponse
// @Router /tts/scripts/prepare [post]
func PrepareTTSScripts(c *gin.Context) {
	var req PrepareTTSScriptsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}
	if len(req.NoticiaIDs) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No news selected"})
		return
	}
	if len(req.NoticiaIDs) > ttsMaxBulletinItems {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Too many items",
			Message: fmt.Sprintf("Se admiten como mucho %d noticias", ttsMaxBulletinItems),
		})
		return
	}

	cfg := config.Load()
	language := strings.TrimSpace(req.Language)
	if language == "" {
		language = cfg.DefaultLang
	}
	language = strings.ToLower(language)
	userID := c.GetInt("user_id")

	previews := []TTSScriptPreview{}
	for _, noticiaID := range req.NoticiaIDs {
		noticiaID = strings.TrimSpace(noticiaID)
		if noticiaID == "" {
			continue
		}

		// ¿Habia ya un borrador de este usuario para esa noticia?
		var existingID int64
		var existingContent, existingStatus string
		err := db.GetPool().QueryRow(c.Request.Context(), `
			SELECT id, content, status FROM tts_scripts
			WHERE noticia_id = $1 AND created_by = $2 AND status = 'draft'
			ORDER BY id DESC LIMIT 1`, noticiaID, userID).Scan(&existingID, &existingContent, &existingStatus)
		if err == nil {
			script, err := fetchTTSScript(c.Request.Context(), existingID)
			if err == nil {
				previews = append(previews, TTSScriptPreview{
					ScriptID:   script.ID,
					NoticiaID:  noticiaID,
					Title:      stringValue(script.Title),
					Content:    script.Content,
					SourceName: stringValue(script.SourceName),
					SourceURL:  stringValue(script.SourceURL),
					Status:     script.Status,
					Existing:   true,
				})
				continue
			}
		}

		script, err := createTTSScriptFromNews(c, noticiaID, language)
		if err != nil {
			respondTTSScriptError(c, err)
			return
		}
		previews = append(previews, TTSScriptPreview{
			ScriptID:   script.ID,
			NoticiaID:  noticiaID,
			Title:      stringValue(script.Title),
			Content:    script.Content,
			SourceName: stringValue(script.SourceName),
			SourceURL:  stringValue(script.SourceURL),
			Status:     script.Status,
			Existing:   false,
		})
	}

	c.JSON(http.StatusOK, gin.H{"scripts": previews})
}

// UpdateTTSScriptRequest es la peticion para corregir el texto de un guion.
type UpdateTTSScriptRequest struct {
	Content string `json:"content"`
}

// UpdateTTSScript guarda el texto revisado por el editor.
//
// El texto se vuelve a normalizar (cifras, fechas, abreviaturas) antes de
// locutarlo, pero se conserva la version escrita por el usuario en
// “raw_content“. Solo se pueden editar guiones en borrador: si ya esta
// aprobado, el audio habria que regenerarlo.
// @Summary Corrige el texto de un guion
// @Tags tts
// @Accept json
// @Produce json
// @Param id path int true "ID del guion"
// @Param request body UpdateTTSScriptRequest true "Texto revisado"
// @Success 200 {object} models.TTSScript
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Router /tts/scripts/{id} [put]
func UpdateTTSScript(c *gin.Context) {
	scriptID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid script ID"})
		return
	}

	var req UpdateTTSScriptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Empty script"})
		return
	}

	cfg := config.Load()
	if len(content) > cfg.TTSMaxScriptChars {
		c.JSON(http.StatusRequestEntityTooLarge, models.ErrorResponse{
			Error:   "Script too long",
			Message: fmt.Sprintf("El guion supera el limite de %d caracteres", cfg.TTSMaxScriptChars),
		})
		return
	}

	var status string
	err = db.GetPool().QueryRow(c.Request.Context(),
		`SELECT status FROM tts_scripts WHERE id = $1`, scriptID).Scan(&status)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Script not found"})
		return
	}
	if status != ttsScriptStatusDraft {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "Script not editable",
			Message: "El guion ya esta aprobado: hay que crear uno nuevo para(locutar) el texto corregido",
		})
		return
	}

	normalized, err := services.TTSNormalizeScript(c.Request.Context(), content, cfg.DefaultLang)
	if err != nil {
		log.Printf("UpdateTTSScript: normalizacion fallida: %v", err)
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
			Error:   "TTS service unavailable",
			Message: "No se pudo preparar el texto para locucion",
		})
		return
	}

	_, err = db.GetPool().Exec(c.Request.Context(), `
		UPDATE tts_scripts
		SET content = $1, raw_content = $2, updated_at = NOW()
		WHERE id = $3`, normalized.Text, content, scriptID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update script", Message: err.Error()})
		return
	}

	script, err := fetchTTSScript(c.Request.Context(), scriptID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch script", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, script)
}

// GetTTSScript devuelve un guion.
// @Summary Detalle de un guion
// @Tags tts
// @Produce json
// @Param id path int true "ID del guion"
// @Success 200 {object} models.TTSScript
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/scripts/{id} [get]
func GetTTSScript(c *gin.Context) {
	scriptID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid script ID"})
		return
	}

	script, err := fetchTTSScript(c.Request.Context(), scriptID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Script not found"})
		return
	}

	c.JSON(http.StatusOK, script)
}

// SetTTSScriptStatus aprueba o rechaza un guion.
// @Summary Aprueba o rechaza un guion
// @Tags tts
// @Produce json
// @Param id path int true "ID del guion"
// @Param status query string true "approved o rejected"
// @Success 200 {object} models.TTSScript
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/scripts/{id}/status [post]
func SetTTSScriptStatus(c *gin.Context) {
	scriptID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid script ID"})
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	if status != ttsScriptStatusApproved && status != ttsScriptStatusRejected {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid status",
			Message: "status debe ser 'approved' o 'rejected'",
		})
		return
	}

	userID := resolveUserID(c.Request.Context(), c.GetInt("user_id"))
	commandTag, err := db.GetPool().Exec(c.Request.Context(), `
		UPDATE tts_scripts
		SET status = $1,
		    approved_by = $2,
		    approved_at = NOW(),
		    updated_at = NOW()
		WHERE id = $3`, status, userID, scriptID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update script", Message: err.Error()})
		return
	}
	if commandTag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Script not found"})
		return
	}

	script, err := fetchTTSScript(c.Request.Context(), scriptID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch script", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, script)
}

// -----------------------------------------------------------------------------
// Tareas
// -----------------------------------------------------------------------------

// ListTTSJobs devuelve las tareas de sintesis.
// @Summary Lista de tareas de sintesis
// @Tags tts
// @Produce json
// @Param status query string false "queued, running, succeeded, failed o cancelled"
// @Param script_id query int false "ID del guion"
// @Success 200 {object} models.TTSJobListResponse
// @Router /tts/jobs [get]
func ListTTSJobs(c *gin.Context) {
	where := "1=1"
	args := []interface{}{}
	argNum := 1

	if status := c.Query("status"); status != "" {
		where += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, status)
		argNum++
	}
	if scriptID := c.Query("script_id"); scriptID != "" {
		where += fmt.Sprintf(" AND script_id = $%d", argNum)
		args = append(args, scriptID)
		argNum++
	}

	args = append(args, 100)
	query := `
		SELECT id, script_id, voice_id, engine, speed, status, attempts, max_attempts,
		       error_code, error_message, queued_at, started_at, finished_at,
		       created_at, updated_at
		FROM tts_jobs
		WHERE ` + where + `
		ORDER BY created_at DESC
		LIMIT $` + strconv.Itoa(argNum)

	rows, err := db.GetPool().Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch jobs", Message: err.Error()})
		return
	}
	defer rows.Close()

	jobs := []models.TTSJob{}
	for rows.Next() {
		var j models.TTSJob
		if err := rows.Scan(&j.ID, &j.ScriptID, &j.VoiceID, &j.Engine, &j.Speed,
			&j.Status, &j.Attempts, &j.MaxAttempts, &j.ErrorCode, &j.ErrorMessage,
			&j.QueuedAt, &j.StartedAt, &j.FinishedAt, &j.CreatedAt, &j.UpdatedAt); err != nil {
			continue
		}
		jobs = append(jobs, j)
	}

	c.JSON(http.StatusOK, models.TTSJobListResponse{Jobs: jobs})
}

// CreateTTSJobRequest es la peticion para encolar una sintesis.
type CreateTTSJobRequest struct {
	ScriptID       int64   `json:"script_id"`
	Voice          string  `json:"voice"`
	Engine         string  `json:"engine"`
	Speed          float64 `json:"speed"`
	IdempotencyKey string  `json:"idempotency_key"`
}

// CreateTTSJob encola la sintesis de un guion aprobado.
//
// La clave de idempotencia evita tareas duplicadas: si no se indica, se
// calcula con el guion, la voz y el motor, de modo que pedir dos veces la
// misma narracion devuelve la tarea existente.
// @Summary Encola una sintesis
// @Tags tts
// @Accept json
// @Produce json
// @Param request body CreateTTSJobRequest true "Datos de la tarea"
// @Success 201 {object} models.TTSJobCreatedResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Router /tts/jobs [post]
func CreateTTSJob(c *gin.Context) {
	var req CreateTTSJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}
	if req.ScriptID <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid script ID"})
		return
	}

	// El guion debe existir y estar aprobado por un editor.
	var scriptStatus string
	err := db.GetPool().QueryRow(c.Request.Context(),
		`SELECT status FROM tts_scripts WHERE id = $1`, req.ScriptID).Scan(&scriptStatus)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Script not found"})
		return
	}
	if scriptStatus != ttsScriptStatusApproved {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "Script not approved",
			Message: "El guion debe estar aprobado antes de locutarlo",
		})
		return
	}

	// Voz: por defecto la primera voz integrada; si se indica, debe existir.
	voiceSlug := strings.TrimSpace(req.Voice)
	var voiceID *int64
	if voiceSlug == "" {
		voiceSlug = "es-base"
	}
	err = db.GetPool().QueryRow(c.Request.Context(),
		`SELECT id FROM tts_voices WHERE slug = $1`, voiceSlug).Scan(&voiceID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Voice not found", Message: "La voz indicada no existe"})
		return
	}

	// Sin motor pedido, el de la voz: una voz de MeloTTS no se puede narrar
	// con Chatterbox sin cambiar de voz.
	engine := resolveTTSEngine(c.Request.Context(), req.Engine, voiceID)

	// Clave de idempotencia.
	speed := clampTTSSpeed(req.Speed)
	key := strings.TrimSpace(req.IdempotencyKey)
	if key == "" {
		key = ttsIdempotencyKey(req.ScriptID, voiceSlug, engine, speed)
	}
	if len(key) > 160 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Idempotency key too long"})
		return
	}

	jobID, created, err := enqueueTTSJob(c, req.ScriptID, voiceSlug, engine, speed, key)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Failed to create job", Message: err.Error()})
		return
	}

	job, err := fetchTTSJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch job", Message: err.Error()})
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, models.TTSJobCreatedResponse{Job: *job, Created: created})
}

// enqueueTTSJob encola la sintesis de un guion de forma idempotente.
//
// Si ya existe una tarea con la misma clave (guion, voz, motor y velocidad),
// devuelve esa tarea sin crear otra. Devuelve el id de la tarea y si es nueva.
// enqueueTTSJob encola la sintesis de un guion de forma idempotente.
//
// idemKey es la clave de idempotencia que puede traer el cliente. Si viene
// vacia se calcula la canonica (script + voz + motor + velocidad). Antes la
// clave del cliente se calculaba y se descartaba (`_ = key`), y aqui se
// recalculaba siempre: un cliente que reintentase con su propia clave NO
// tenia idempotencia real.
func enqueueTTSJob(c *gin.Context, scriptID int64, voiceSlug, engine string, speed float64, idemKey string) (int64, bool, error) {
	cfg := config.Load()

	if voiceSlug == "" {
		voiceSlug = "es-base"
	}

	var voiceID *int64
	err := db.GetPool().QueryRow(c.Request.Context(),
		`SELECT id FROM tts_voices WHERE slug = $1`, voiceSlug).Scan(&voiceID)
	if err != nil {
		return 0, false, fmt.Errorf("la voz indicada no existe")
	}
	engine = resolveTTSEngine(c.Request.Context(), engine, voiceID)
	speed = clampTTSSpeed(speed)

	key := strings.TrimSpace(idemKey)
	if key == "" {
		key = ttsIdempotencyKey(scriptID, voiceSlug, engine, speed)
	}
	if len(key) > 160 {
		return 0, false, fmt.Errorf("la clave de idempotencia es demasiado larga")
	}

	var jobID int64
	created := true
	err = db.GetPool().QueryRow(c.Request.Context(), `
		INSERT INTO tts_jobs (script_id, voice_id, engine, idempotency_key, max_attempts, speed)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id`, scriptID, voiceID, engine, key, cfg.TTSMaxAttempts, speed).Scan(&jobID)
	if err != nil {
		// Ya existe una tarea con esa clave: se recupera.
		err = db.GetPool().QueryRow(c.Request.Context(),
			`SELECT id FROM tts_jobs WHERE idempotency_key = $1`, key).Scan(&jobID)
		if err != nil {
			return 0, false, fmt.Errorf("no se pudo encolar la tarea")
		}
		created = false
	}

	if created {
		cache.PushTTSJob(c.Request.Context(), jobID)
	}

	return jobID, created, nil
}

// GetTTSJob devuelve el estado de una tarea.
// @Summary Estado de una tarea de sintesis
// @Tags tts
// @Produce json
// @Param id path int true "ID de la tarea"
// @Success 200 {object} models.TTSJob
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/jobs/{id} [get]
func GetTTSJob(c *gin.Context) {
	jobID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid job ID"})
		return
	}

	job, err := fetchTTSJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

// CancelTTSJob cancela una tarea en cola.
// @Summary Cancela una tarea pendiente
// @Tags tts
// @Produce json
// @Param id path int true "ID de la tarea"
// @Success 200 {object} models.TTSJob
// @Failure 409 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/jobs/{id}/cancel [post]
func CancelTTSJob(c *gin.Context) {
	jobID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid job ID"})
		return
	}

	var status string
	err = db.GetPool().QueryRow(c.Request.Context(),
		`SELECT status FROM tts_jobs WHERE id = $1`, jobID).Scan(&status)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Job not found"})
		return
	}
	if status != ttsJobStatusQueued {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "Job not cancellable",
			Message: "Solo se pueden cancelar tareas que aun no han empezado",
		})
		return
	}

	commandTag, err := db.GetPool().Exec(c.Request.Context(), `
		UPDATE tts_jobs
		SET status = 'cancelled', finished_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'queued'`, jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to cancel job", Message: err.Error()})
		return
	}
	if commandTag.RowsAffected() == 0 {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "Job not cancellable",
			Message: "La tarea ya no se puede cancelar",
		})
		return
	}

	job, err := fetchTTSJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch job", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, job)
}

// -----------------------------------------------------------------------------
// Audios
// -----------------------------------------------------------------------------

// ListTTSAudios devuelve los audios generados.
// @Summary Lista de audios
// @Tags tts
// @Produce json
// @Param status query string false "pending_review, approved o rejected"
// @Success 200 {object} models.TTSAudioListResponse
// @Router /tts/audios [get]
func ListTTSAudios(c *gin.Context) {
	where := "1=1"
	args := []interface{}{}
	argNum := 1

	if status := c.Query("status"); status != "" {
		where += fmt.Sprintf(" AND a.status = $%d", argNum)
		args = append(args, status)
		argNum++
	}
	if scriptID := c.Query("script_id"); scriptID != "" {
		where += fmt.Sprintf(" AND a.script_id = $%d", argNum)
		args = append(args, scriptID)
		argNum++
	}

	args = append(args, 100)
	query := `
		SELECT a.id, a.job_id, a.script_id, a.voice_id, a.engine, a.engine_version,
		       a.storage_path, a.format, a.duration_seconds, a.sample_rate,
		       a.size_bytes, a.sha256, a.parameters, a.status, a.approved_by,
		       a.approved_at, a.created_at,
		       s.noticia_id, s.source_url, s.source_name
		FROM tts_audios a
		JOIN tts_scripts s ON s.id = a.script_id
		WHERE ` + where + `
		ORDER BY a.created_at DESC
		LIMIT $` + strconv.Itoa(argNum)

	rows, err := db.GetPool().Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch audios", Message: err.Error()})
		return
	}
	defer rows.Close()

	audios := []models.TTSAudio{}
	for rows.Next() {
		var a models.TTSAudio
		if err := rows.Scan(&a.ID, &a.JobID, &a.ScriptID, &a.VoiceID, &a.Engine,
			&a.EngineVersion, &a.StoragePath, &a.Format, &a.DurationSeconds,
			&a.SampleRate, &a.SizeBytes, &a.SHA256, &a.Parameters, &a.Status,
			&a.ApprovedBy, &a.ApprovedAt, &a.CreatedAt,
			&a.NoticiaID, &a.SourceURL, &a.SourceName); err != nil {
			continue
		}
		audios = append(audios, a)
	}

	c.JSON(http.StatusOK, models.TTSAudioListResponse{Audios: audios})
}

// GetTTSAudio devuelve los metadatos de un audio.
// @Summary Metadatos de un audio
// @Tags tts
// @Produce json
// @Param id path int true "ID del audio"
// @Success 200 {object} models.TTSAudio
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/audios/{id} [get]
func GetTTSAudio(c *gin.Context) {
	audioID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid audio ID"})
		return
	}

	audio, err := fetchTTSAudio(c.Request.Context(), audioID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Audio not found"})
		return
	}

	c.JSON(http.StatusOK, audio)
}

// GetTTSAudioFile sirve el archivo de audio.
//
// El archivo se sirve desde el backend con autenticacion: no esta publicado
// por nginx y no se puede descargar sin token.
// @Summary Descarga el audio generado
// @Tags tts
// @Produce audio/wav
// @Param id path int true "ID del audio"
// @Success 200 {file} binary
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/audios/{id}/file [get]
func GetTTSAudioFile(c *gin.Context) {
	audioID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid audio ID"})
		return
	}

	var storagePath string
	err = db.GetPool().QueryRow(c.Request.Context(),
		`SELECT storage_path FROM tts_audios WHERE id = $1`, audioID).Scan(&storagePath)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Audio not found"})
		return
	}

	cfg := config.Load()
	fullPath, err := resolveTTSAudioPath(cfg.TTSAudioPath, storagePath)
	if err != nil {
		log.Printf("GetTTSAudioFile: ruta invalida para el audio %d: %v", audioID, err)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid audio path"})
		return
	}

	if _, err := os.Stat(fullPath); err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Audio file not found"})
		return
	}

	c.Header("Content-Type", "audio/wav")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"audio-%d.wav\"", audioID))
	c.File(fullPath)
}

// SetTTSAudioStatus aprueba o rechaza un audio generado.
// @Summary Aprueba o rechaza un audio
// @Tags tts
// @Produce json
// @Param id path int true "ID del audio"
// @Param status query string true "approved o rejected"
// @Success 200 {object} models.TTSAudio
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/audios/{id}/status [post]
func SetTTSAudioStatus(c *gin.Context) {
	audioID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid audio ID"})
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	if status != ttsAudioStatusApprov && status != ttsAudioStatusReject {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid status",
			Message: "status debe ser 'approved' o 'rejected'",
		})
		return
	}

	userID := resolveUserID(c.Request.Context(), c.GetInt("user_id"))
	commandTag, err := db.GetPool().Exec(c.Request.Context(), `
		UPDATE tts_audios
		SET status = $1, approved_by = $2, approved_at = NOW()
		WHERE id = $3`, status, userID, audioID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update audio", Message: err.Error()})
		return
	}
	if commandTag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Audio not found"})
		return
	}

	audio, err := fetchTTSAudio(c.Request.Context(), audioID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch audio", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, audio)
}

// -----------------------------------------------------------------------------
// Noticiario (edicion de varias narraciones)
// -----------------------------------------------------------------------------

// CreateTTSBulletinRequest es la peticion para crear un noticiario.
//
// Se puede pasar una lista de noticias (noticia_ids): el backend crea los
// guiones en borrador. O una lista de guiones ya aprobados (script_ids).
type CreateTTSBulletinRequest struct {
	Title      string   `json:"title"`
	Language   string   `json:"language"`
	NoticiaIDs []string `json:"noticia_ids"`
	ScriptIDs  []int64  `json:"script_ids"`
	SilenceMs  int      `json:"silence_ms"`
	Voice      string   `json:"voice"`
	Engine     string   `json:"engine"`
	Speed      float64  `json:"speed"`
}

// Limite de noticias por noticiario.
const ttsMaxBulletinItems = 50

// Limites de velocidad de locucion.
const (
	ttsMinSpeed = 0.5
	ttsMaxSpeed = 2.0
)

// clampTTSSpeed limita la velocidad de locucion.
func clampTTSSpeed(value float64) float64 {
	if value <= 0 {
		return 1.0
	}
	if value < ttsMinSpeed {
		return ttsMinSpeed
	}
	if value > ttsMaxSpeed {
		return ttsMaxSpeed
	}
	return value
}

// CreateTTSBulletin crea un noticiario a partir de las noticias elegidas.
//
// Los guiones se crean en borrador: el noticiario se approve con
// POST /api/tts/bulletins/:id/approve, que es el momento en que se encolan
// las narraciones.
// @Summary Crea un noticiario
// @Tags tts
// @Accept json
// @Produce json
// @Param request body CreateTTSBulletinRequest true "Datos del noticiario"
// @Success 201 {object} models.TTSBulletin
// @Failure 400 {object} models.ErrorResponse
// @Failure 503 {object} models.ErrorResponse
// @Router /tts/bulletins [post]
func CreateTTSBulletin(c *gin.Context) {
	var req CreateTTSBulletinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Title is required"})
		return
	}
	if len(title) > 200 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Title too long"})
		return
	}

	total := len(req.NoticiaIDs) + len(req.ScriptIDs)
	if total == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Empty bulletin",
			Message: "Indica las noticias (noticia_ids) o los guiones (script_ids)",
		})
		return
	}
	if total > ttsMaxBulletinItems {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Too many items",
			Message: fmt.Sprintf("Un noticiario admite hasta %d noticias", ttsMaxBulletinItems),
		})
		return
	}

	cfg := config.Load()
	language := strings.TrimSpace(req.Language)
	if language == "" {
		language = cfg.DefaultLang
	}
	language = strings.ToLower(language)

	silenceMs := clampTTSSilenceMs(req.SilenceMs)
	speed := clampTTSSpeed(req.Speed)

	// Voz y motor del noticiario: se resuelven ahora para no dejar el
	// noticiario a medias si la voz no existe.
	voiceSlug := strings.TrimSpace(req.Voice)
	if voiceSlug == "" {
		voiceSlug = "es-base"
	}
	var voiceID *int64
	err := db.GetPool().QueryRow(c.Request.Context(),
		`SELECT id FROM tts_voices WHERE slug = $1`, voiceSlug).Scan(&voiceID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Voice not found",
			Message: "La voz indicada no existe",
		})
		return
	}
	// Sin motor pedido, el de la voz (guardado en el noticiario).
	engine := resolveTTSEngine(c.Request.Context(), req.Engine, voiceID)

	userID := resolveUserID(c.Request.Context(), c.GetInt("user_id"))

	// Los guiones se crean antes que el noticiario: si algo falla, no queda
	// un noticiario a medias.
	scriptIDs := make([]int64, 0, total)
	for _, scriptID := range req.ScriptIDs {
		script, err := fetchTTSScript(c.Request.Context(), scriptID)
		if err != nil {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Script not found", Message: fmt.Sprintf("Guion %d no existe", scriptID)})
			return
		}
		// Los guiones en borrador son los normales: todavia no se han
		// aprobado y es la aprobacion del noticiario la que lo hara. Solo se
		// rechazan los que un editor habia descartado.
		if script.Status == ttsScriptStatusRejected {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error:   "Script rejected",
				Message: fmt.Sprintf("El guion %d fue descartado", scriptID),
			})
			return
		}
		scriptIDs = append(scriptIDs, scriptID)
	}

	for _, noticiaID := range req.NoticiaIDs {
		noticiaID = strings.TrimSpace(noticiaID)
		if noticiaID == "" {
			continue
		}
		script, err := createTTSScriptFromNews(c, noticiaID, language)
		if err != nil {
			respondTTSScriptError(c, err)
			return
		}
		scriptIDs = append(scriptIDs, script.ID)
	}

	if len(scriptIDs) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Empty bulletin"})
		return
	}

	var bulletinID int64
	err = db.GetPool().QueryRow(c.Request.Context(), `
		INSERT INTO tts_bulletins (title, language, silence_ms, item_count, created_by,
		                           voice_id, engine, speed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`, title, language, silenceMs, len(scriptIDs), userID,
		voiceID, engine, speed).Scan(&bulletinID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create bulletin", Message: err.Error()})
		return
	}

	for position, scriptID := range scriptIDs {
		script, err := fetchTTSScript(c.Request.Context(), scriptID)
		if err != nil {
			continue
		}
		_, err = db.GetPool().Exec(c.Request.Context(), `
			INSERT INTO tts_bulletin_items (bulletin_id, script_id, position, source_url,
			                                source_name, source_title)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (bulletin_id, script_id) DO NOTHING`,
			bulletinID, scriptID, position+1, script.SourceURL, script.SourceName, script.SourceTitle)
		if err != nil {
			log.Printf("CreateTTSBulletin: no se pudo añadir el item %d: %v", scriptID, err)
		}
	}

	bulletin, err := fetchTTSBulletin(c.Request.Context(), bulletinID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch bulletin", Message: err.Error()})
		return
	}
	if items, err := fetchTTSBulletinItems(c.Request.Context(), bulletinID); err == nil {
		bulletin.Items = items
	}

	c.JSON(http.StatusCreated, bulletin)
}

// ApproveTTSBulletin aprueba el contenido del noticiario y encola las
// narraciones.
//
// Es el momento en que el editor da el visto bueno: los guiones en borrador
// pasan a aprobados y se encola la locucion de cada item.
// @Summary Aprueba el noticiario y encola sus narraciones
// @Tags tts
// @Produce json
// @Param id path int true "ID del noticiario"
// @Success 200 {object} models.TTSBulletin
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/bulletins/{id}/approve [post]
func ApproveTTSBulletin(c *gin.Context) {
	bulletinID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid bulletin ID"})
		return
	}

	bulletin, err := fetchTTSBulletin(c.Request.Context(), bulletinID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Bulletin not found"})
		return
	}
	if bulletin.Status != "pending" && bulletin.Status != "failed" {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "Bulletin not approvable",
			Message: "El noticiario ya se aprobo o esta en produccion",
		})
		return
	}

	userID := resolveUserID(c.Request.Context(), c.GetInt("user_id"))

	// La voz, el motor y la velocidad son los del noticiario: asi todos los
	// items se locutan igual.
	var (
		voiceID *int64
		engine  = ""
		speed   = 1.0
	)
	if bulletin.VoiceID != nil {
		voiceID = bulletin.VoiceID
	}
	if bulletin.Engine != "" {
		engine = bulletin.Engine
	}
	if bulletin.Speed > 0 {
		speed = bulletin.Speed
	}
	voiceSlug := "es-base"
	if voiceID != nil {
		var slug string
		if err := db.GetPool().QueryRow(c.Request.Context(),
			`SELECT slug FROM tts_voices WHERE id = $1`, *voiceID).Scan(&slug); err == nil {
			voiceSlug = slug
		}
	}

	items, err := fetchTTSBulletinItems(c.Request.Context(), bulletinID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch items", Message: err.Error()})
		return
	}

	queued := 0
	for _, item := range items {
		// El guion del item debe estar aprobado (los creados al vuelo se
		// aprueban aqui).
		_, err := db.GetPool().Exec(c.Request.Context(),
			`UPDATE tts_scripts SET status = 'approved', approved_by = $1, approved_at = NOW(), updated_at = NOW()
			 WHERE id = $2 AND status = 'draft'`, userID, item.ScriptID)
		if err != nil {
			log.Printf("ApproveTTSBulletin: no se pudo aprobar el guion %d: %v", item.ScriptID, err)
			continue
		}

		jobID, created, err := enqueueTTSJob(c, item.ScriptID, voiceSlug, engine, speed, "")
		if err != nil {
			log.Printf("ApproveTTSBulletin: no se pudo encolar la narracion %d: %v", item.ScriptID, err)
			continue
		}
		if created {
			queued++
		}
		_ = jobID
	}

	// El noticiario queda a la espera de que el worker monte el audio.
	_, err = db.GetPool().Exec(c.Request.Context(),
		`UPDATE tts_bulletins SET status = 'pending', updated_at = NOW() WHERE id = $1`, bulletinID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update bulletin", Message: err.Error()})
		return
	}

	bulletin, err = fetchTTSBulletin(c.Request.Context(), bulletinID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch bulletin", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bulletin": bulletin, "queued": queued})
}

// CancelTTSBulletin cancela un noticiario en curso.
//
// Cancela las tareas en cola de sus items y marca el noticiario como
// cancelado. Una narracion que ya esta en marcha no se interrumpe (no es
// seguro): termina y su audio queda registrado, pero el noticiario ya no se
// usara.
// @Summary Cancela un noticiario en curso
// @Tags tts
// @Produce json
// @Param id path int true "ID del noticiario"
// @Success 200 {object} models.TTSBulletin
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Router /tts/bulletins/{id}/cancel [post]
func CancelTTSBulletin(c *gin.Context) {
	bulletinID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid bulletin ID"})
		return
	}

	bulletin, err := fetchTTSBulletin(c.Request.Context(), bulletinID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Bulletin not found"})
		return
	}
	if bulletin.Status == "approved" || bulletin.Status == "ready" {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "Bulletin not cancellable",
			Message: "El noticiario ya esta terminado",
		})
		return
	}

	_, err = db.GetPool().Exec(c.Request.Context(), `
		UPDATE tts_jobs
		SET status = 'cancelled', finished_at = NOW(), updated_at = NOW(),
		    error_code = 'cancelled', error_message = 'El noticiario fue cancelado'
		WHERE status = 'queued'
		  AND script_id IN (SELECT script_id FROM tts_bulletin_items WHERE bulletin_id = $1)
	`, bulletinID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to cancel jobs", Message: err.Error()})
		return
	}

	_, err = db.GetPool().Exec(c.Request.Context(), `
		UPDATE tts_bulletins
		SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1`, bulletinID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to cancel bulletin", Message: err.Error()})
		return
	}

	bulletin, err = fetchTTSBulletin(c.Request.Context(), bulletinID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch bulletin", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, bulletin)
}

// ListTTSBulletins devuelve los noticiarios.
// @Summary Lista de noticiarios
// @Tags tts
// @Produce json
// @Param status query string false "pending, ready, approved, rejected o failed"
// @Success 200 {object} models.TTSBulletinListResponse
// @Router /tts/bulletins [get]
func ListTTSBulletins(c *gin.Context) {
	where := "1=1"
	args := []interface{}{}
	argNum := 1

	if status := c.Query("status"); status != "" {
		where += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, status)
		argNum++
	}
	args = append(args, 100)

	query := `
		SELECT b.id, b.title, TRIM(b.language), b.status, b.voice_id,
		       COALESCE(v.slug, ''), b.engine, b.speed, b.audio_path, b.duration_seconds,
		       b.size_bytes, b.sha256, b.item_count, b.silence_ms, b.created_by, b.approved_by,
		       b.approved_at, b.created_at, b.updated_at
		FROM tts_bulletins b
		LEFT JOIN tts_voices v ON v.id = b.voice_id
		WHERE ` + where + `
		ORDER BY created_at DESC
		LIMIT $` + strconv.Itoa(argNum)

	rows, err := db.GetPool().Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch bulletins", Message: err.Error()})
		return
	}
	defer rows.Close()

	bulletins := []models.TTSBulletin{}
	for rows.Next() {
		var b models.TTSBulletin
		if err := rows.Scan(&b.ID, &b.Title, &b.Language, &b.Status, &b.VoiceID,
			&b.VoiceSlug, &b.Engine, &b.Speed, &b.AudioPath,
			&b.DurationSeconds, &b.SizeBytes, &b.SHA256, &b.ItemCount, &b.SilenceMs,
			&b.CreatedBy, &b.ApprovedBy, &b.ApprovedAt, &b.CreatedAt, &b.UpdatedAt); err != nil {
			continue
		}
		bulletins = append(bulletins, b)
	}

	c.JSON(http.StatusOK, models.TTSBulletinListResponse{Bulletins: bulletins})
}

// GetTTSBulletin devuelve un noticiario con sus items y el manifiesto.
// @Summary Detalle de un noticiario
// @Tags tts
// @Produce json
// @Param id path int true "ID del noticiario"
// @Success 200 {object} models.TTSBulletin
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/bulletins/{id} [get]
func GetTTSBulletin(c *gin.Context) {
	bulletinID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid bulletin ID"})
		return
	}

	bulletin, err := fetchTTSBulletin(c.Request.Context(), bulletinID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Bulletin not found"})
		return
	}

	items, err := fetchTTSBulletinItems(c.Request.Context(), bulletinID)
	if err == nil {
		bulletin.Items = items
	}

	c.JSON(http.StatusOK, bulletin)
}

// GetTTSBulletinFile sirve el audio del noticiario.
// @Summary Descarga el audio del noticiario
// @Tags tts
// @Produce audio/wav
// @Param id path int true "ID del noticiario"
// @Success 200 {file} binary
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/bulletins/{id}/file [get]
func GetTTSBulletinFile(c *gin.Context) {
	bulletinID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid bulletin ID"})
		return
	}

	var storagePath *string
	err = db.GetPool().QueryRow(c.Request.Context(),
		`SELECT audio_path FROM tts_bulletins WHERE id = $1`, bulletinID).Scan(&storagePath)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Bulletin not found"})
		return
	}
	if storagePath == nil || strings.TrimSpace(*storagePath) == "" {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Bulletin audio not ready",
			Message: "El noticiario aun no tiene audio",
		})
		return
	}

	cfg := config.Load()
	fullPath, err := resolveTTSAudioPath(cfg.TTSAudioPath, *storagePath)
	if err != nil {
		log.Printf("GetTTSBulletinFile: ruta invalida para el noticiario %d: %v", bulletinID, err)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid audio path"})
		return
	}

	if _, err := os.Stat(fullPath); err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Bulletin file not found"})
		return
	}

	c.Header("Content-Type", "audio/wav")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"noticiario-%d.wav\"", bulletinID))
	c.File(fullPath)
}

// SetTTSBulletinStatus aprueba o rechaza el audio montado del noticiario.
// @Summary Aprueba o rechaza el noticiario
// @Tags tts
// @Produce json
// @Param id path int true "ID del noticiario"
// @Param status query string true "approved o rejected"
// @Success 200 {object} models.TTSBulletin
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /tts/bulletins/{id}/status [post]
func SetTTSBulletinStatus(c *gin.Context) {
	bulletinID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid bulletin ID"})
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	if status != "approved" && status != "rejected" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid status",
			Message: "status debe ser 'approved' o 'rejected'",
		})
		return
	}

	userID := resolveUserID(c.Request.Context(), c.GetInt("user_id"))
	commandTag, err := db.GetPool().Exec(c.Request.Context(), `
		UPDATE tts_bulletins
		SET status = $1, approved_by = $2, approved_at = NOW(), updated_at = NOW()
		WHERE id = $3 AND status IN ('ready', 'approved', 'rejected')`, status, userID, bulletinID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update bulletin", Message: err.Error()})
		return
	}
	if commandTag.RowsAffected() == 0 {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "Bulletin not ready",
			Message: "El noticiario aun no tiene audio montado",
		})
		return
	}

	bulletin, err := fetchTTSBulletin(c.Request.Context(), bulletinID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch bulletin", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, bulletin)
}

// -----------------------------------------------------------------------------
// Utilidades internas
// -----------------------------------------------------------------------------

// ttsSource agrupa el texto editorial de una noticia con su procedencia.
type ttsSource struct {
	NoticiaID     string
	TranslationID *int64
	Title         *string
	Headline      string
	Content       string
	URL           *string
	Name          *string
}

// fetchTTSSource obtiene el texto de una noticia en el idioma pedido,
// prefiriendo la traduccion editorial.
func fetchTTSSource(ctx context.Context, noticiaID, language string) (*ttsSource, error) {
	var (
		titulo, resumen, contenido *string
		url, fuente                *string
		translationID              *int64
		tituloTrad, resumenTrad    *string
		contenidoTrad              *string
	)

	err := db.GetPool().QueryRow(ctx, `
		SELECT n.titulo, n.resumen, n.contenido, n.url, n.fuente_nombre,
		       t.id, t.titulo_trad, t.resumen_trad, t.contenido_trad
		FROM noticias n
		LEFT JOIN traducciones t
		       ON t.noticia_id = n.id AND TRIM(t.lang_to) = $1 AND t.status = 'done'
		WHERE n.id = $2`, language, noticiaID).Scan(
		&titulo, &resumen, &contenido, &url, &fuente,
		&translationID, &tituloTrad, &resumenTrad, &contenidoTrad,
	)
	if err != nil {
		return nil, err
	}

	source := &ttsSource{
		NoticiaID:     noticiaID,
		TranslationID: translationID,
		Title:         titulo,
		URL:           url,
		Name:          fuente,
	}

	// Titulo: traduccion si existe, si no el original.
	if tituloTrad != nil && strings.TrimSpace(*tituloTrad) != "" {
		source.Headline = strings.TrimSpace(*tituloTrad)
	} else if titulo != nil {
		source.Headline = strings.TrimSpace(*titulo)
	}

	// Cuerpo: contenido traducido, resumen traducido, contenido o resumen.
	for _, candidate := range []*string{contenidoTrad, resumenTrad, contenido, resumen} {
		if candidate != nil && strings.TrimSpace(*candidate) != "" {
			source.Content = strings.TrimSpace(*candidate)
			break
		}
	}

	return source, nil
}

// fetchTTSScript devuelve un guion completo.
func fetchTTSScript(ctx context.Context, scriptID int64) (*models.TTSScript, error) {
	var s models.TTSScript
	err := db.GetPool().QueryRow(ctx, `
		SELECT id, noticia_id, translation_id, TRIM(language), title, raw_content,
		       content, source_title, source_url, source_name, status, created_by,
		       approved_by, approved_at, created_at, updated_at, length(content)
		FROM tts_scripts WHERE id = $1`, scriptID).Scan(
		&s.ID, &s.NoticiaID, &s.TranslationID, &s.Language, &s.Title, &s.RawContent,
		&s.Content, &s.SourceTitle, &s.SourceURL, &s.SourceName, &s.Status,
		&s.CreatedBy, &s.ApprovedBy, &s.ApprovedAt, &s.CreatedAt, &s.UpdatedAt,
		&s.ContentLength,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// fetchTTSJob devuelve una tarea con su audio asociado (si lo hay).
func fetchTTSJob(ctx context.Context, jobID int64) (*models.TTSJob, error) {
	var j models.TTSJob
	var startedAt, finishedAt *time.Time
	err := db.GetPool().QueryRow(ctx, `
		SELECT id, script_id, voice_id, engine, speed, status, attempts, max_attempts,
		       error_code, error_message, queued_at, started_at, finished_at,
		       created_at, updated_at
		FROM tts_jobs WHERE id = $1`, jobID).Scan(
		&j.ID, &j.ScriptID, &j.VoiceID, &j.Engine, &j.Speed, &j.Status, &j.Attempts,
		&j.MaxAttempts, &j.ErrorCode, &j.ErrorMessage, &j.QueuedAt, &startedAt,
		&finishedAt, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	j.StartedAt = startedAt
	j.FinishedAt = finishedAt

	// Tiempo en marcha: la interfaz lo muestra para que se vea que el
	// proceso avanza (o no).
	if startedAt != nil && j.Status == "running" {
		j.ElapsedSeconds = int(time.Since(*startedAt).Seconds())
	}

	audio, err := fetchTTSAudioByJob(ctx, jobID)
	if err == nil {
		j.Audio = audio
	}

	return &j, nil
}

// fetchTTSAudio devuelve los metadatos de un audio con su procedencia.
func fetchTTSAudio(ctx context.Context, audioID int64) (*models.TTSAudio, error) {
	var a models.TTSAudio
	err := db.GetPool().QueryRow(ctx, `
		SELECT a.id, a.job_id, a.script_id, a.voice_id, a.engine, a.engine_version,
		       a.storage_path, a.format, a.duration_seconds, a.sample_rate,
		       a.size_bytes, a.sha256, a.parameters, a.status, a.approved_by,
		       a.approved_at, a.created_at,
		       s.noticia_id, s.source_url, s.source_title, s.source_name, s.title
		FROM tts_audios a
		JOIN tts_scripts s ON s.id = a.script_id
		WHERE a.id = $1`, audioID).Scan(
		&a.ID, &a.JobID, &a.ScriptID, &a.VoiceID, &a.Engine, &a.EngineVersion,
		&a.StoragePath, &a.Format, &a.DurationSeconds, &a.SampleRate,
		&a.SizeBytes, &a.SHA256, &a.Parameters, &a.Status, &a.ApprovedBy,
		&a.ApprovedAt, &a.CreatedAt,
		&a.NoticiaID, &a.SourceURL, &a.SourceTitle, &a.SourceName, &a.ScriptTitle,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// fetchTTSAudioByJob devuelve el ultimo audio generado por una tarea.
func fetchTTSAudioByJob(ctx context.Context, jobID int64) (*models.TTSAudio, error) {
	var a models.TTSAudio
	err := db.GetPool().QueryRow(ctx, `
		SELECT id, job_id, script_id, voice_id, engine, engine_version, storage_path,
		       format, duration_seconds, sample_rate, size_bytes, sha256, parameters,
		       status, approved_by, approved_at, created_at
		FROM tts_audios WHERE job_id = $1 ORDER BY id DESC LIMIT 1`, jobID).Scan(
		&a.ID, &a.JobID, &a.ScriptID, &a.VoiceID, &a.Engine, &a.EngineVersion,
		&a.StoragePath, &a.Format, &a.DurationSeconds, &a.SampleRate, &a.SizeBytes,
		&a.SHA256, &a.Parameters, &a.Status, &a.ApprovedBy, &a.ApprovedAt, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// fetchTTSBulletin devuelve un noticiario.
func fetchTTSBulletin(ctx context.Context, bulletinID int64) (*models.TTSBulletin, error) {
	var b models.TTSBulletin
	err := db.GetPool().QueryRow(ctx, `
		SELECT b.id, b.title, TRIM(b.language), b.status, b.voice_id,
		       COALESCE(v.slug, ''), b.engine, b.speed, b.audio_path, b.duration_seconds,
		       b.size_bytes, b.sha256, b.item_count, b.silence_ms, b.created_by, b.approved_by,
		       b.approved_at, b.created_at, b.updated_at
		FROM tts_bulletins b
		LEFT JOIN tts_voices v ON v.id = b.voice_id
		WHERE b.id = $1`, bulletinID).Scan(
		&b.ID, &b.Title, &b.Language, &b.Status, &b.VoiceID, &b.VoiceSlug, &b.Engine,
		&b.Speed, &b.AudioPath, &b.DurationSeconds,
		&b.SizeBytes, &b.SHA256, &b.ItemCount, &b.SilenceMs, &b.CreatedBy,
		&b.ApprovedBy, &b.ApprovedAt, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// fetchTTSBulletinItems devuelve los items de un noticiario en orden, con la
// referencia a la noticia original de cada uno.
func fetchTTSBulletinItems(ctx context.Context, bulletinID int64) ([]models.TTSBulletinItem, error) {
	rows, err := db.GetPool().Query(ctx, `
		SELECT i.id, i.bulletin_id, i.script_id, i.position, i.audio_id,
		       i.duration_seconds, i.start_offset, i.source_url, i.source_name,
		       i.source_title, s.noticia_id, s.title, s.status
		FROM tts_bulletin_items i
		JOIN tts_scripts s ON s.id = i.script_id
		WHERE i.bulletin_id = $1
		ORDER BY i.position`, bulletinID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.TTSBulletinItem{}
	for rows.Next() {
		var item models.TTSBulletinItem
		if err := rows.Scan(&item.ID, &item.BulletinID, &item.ScriptID, &item.Position,
			&item.AudioID, &item.DurationSeconds, &item.StartOffset, &item.SourceURL,
			&item.SourceName, &item.SourceTitle, &item.NoticiaID, &item.ScriptTitle,
			&item.ScriptStatus); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// resolveTTSAudioPath resuelve la ruta de un audio dentro del volumen y
// descarta cualquier intento de salir de ese directorio.
func resolveTTSAudioPath(baseDir, storagePath string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("no hay directorio base configurado para los audios")
	}
	if storagePath == "" {
		return "", fmt.Errorf("ruta de audio vacia")
	}
	if filepath.IsAbs(storagePath) {
		return "", fmt.Errorf("la ruta debe ser relativa")
	}

	base, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	full := filepath.Clean(filepath.Join(base, storagePath))
	if full != base && !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("la ruta sale del directorio de audios")
	}
	return full, nil
}

// resolveTTSEngine elige el motor de una narracion.
//
// El orden importa: si el cliente pide un motor, manda el pedido. Si no, el
// motor de la VOZ (melotts-speaker-42 no sirve con chatterbox: el motor
// ignoraria el hablante y saldria otra voz). Solo si la voz no declara motor
// se cae al global.
func resolveTTSEngine(ctx context.Context, requested string, voiceID *int64) string {
	if engine := strings.TrimSpace(requested); engine != "" {
		return engine
	}
	if voiceID != nil {
		var engine string
		err := db.GetPool().QueryRow(ctx,
			`SELECT TRIM(COALESCE(engine, '')) FROM tts_voices WHERE id = $1`, *voiceID).Scan(&engine)
		if err == nil && engine != "" {
			return engine
		}
	}
	return config.Load().TTSEngine
}

// ttsIdempotencyKey calcula la clave por defecto de una narracion.
//
// La velocidad forma parte de la clave: la misma noticia locutada mas rapido
// es una narracion distinta.
func ttsIdempotencyKey(scriptID int64, voiceSlug, engine string, speed float64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("script:%d|voice:%s|engine:%s|speed:%.2f", scriptID, voiceSlug, engine, speed)))
	return hex.EncodeToString(sum[:])
}

// clampTTSSilenceMs limita la pausa entre noticias del noticiario.
func clampTTSSilenceMs(value int) int {
	if value < 0 {
		return 0
	}
	if value > 10000 {
		return 10000
	}
	return value
}

// nullable devuelve nil si el texto esta vacio (para columnas TEXT NULL).
func nullable(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
