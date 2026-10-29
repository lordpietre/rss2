package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Cliente HTTP hacia el servicio de TTS. Es una dependencia interna: el
// servicio no se publica fuera de la red de Docker.
const ttsRequestTimeout = 120 * time.Second

// TTSHealthResponse es el estado del motor segun el servicio de TTS.
type TTSHealthResponse struct {
	Status  string                 `json:"status"`
	Engines map[string]interface{} `json:"engines"`
	Queue   map[string]interface{} `json:"queue"`
	Stats   map[string]interface{} `json:"stats"`
}

// TTSNormalizeResponse es el resultado de normalizar un guion.
type TTSNormalizeResponse struct {
	Text      string `json:"text"`
	Sentences int    `json:"sentences"`
	Length    int    `json:"length"`
}

func ttsClient() *http.Client {
	return &http.Client{Timeout: ttsRequestTimeout}
}

// ttsRequest construye una peticion autenticada hacia el servicio de TTS.
func ttsRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, cfg.TTSURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if cfg.TTSAPIKey != "" {
		req.Header.Set("X-API-Key", cfg.TTSAPIKey)
	}
	return req, nil
}

// TTSHealth consulta el estado del motor de voz.
func TTSHealth(ctx context.Context) (*TTSHealthResponse, error) {
	req, err := ttsRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return nil, err
	}

	resp, err := ttsClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts health request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tts service returned status %d", resp.StatusCode)
	}

	var result TTSHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TTSNormalizeScript pide al servicio de TTS la version locutible de un guion.
// La normalizacion vive en el servicio para que exista una sola
// implementacion y el backend no tenga que reimplementarla.
func TTSNormalizeScript(ctx context.Context, text, language string) (*TTSNormalizeResponse, error) {
	payload, err := json.Marshal(map[string]string{
		"text":     text,
		"language": language,
	})
	if err != nil {
		return nil, err
	}

	req, err := ttsRequest(ctx, http.MethodPost, "/api/scripts/normalize", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ttsClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts normalize request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("tts service returned status %d: %s", resp.StatusCode, string(body))
	}

	var result TTSNormalizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TTSCreateVoice registra una muestra de voz propia en el servicio de TTS.
// La muestra se guarda en el volumen del servicio: el backend nunca la
// almacena ni la sirve. engine vacio = el motor por defecto del servicio.
func TTSCreateVoice(ctx context.Context, file multipart.File, header *multipart.FileHeader, slug, displayName, language, engine string) (map[string]interface{}, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("sample", header.Filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	fields := map[string]string{
		"slug":         slug,
		"display_name": displayName,
		"language":     language,
	}
	if engine != "" {
		fields["engine"] = engine
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := ttsRequest(ctx, http.MethodPost, "/api/voices", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := ttsClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts create voice request failed: %w", err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("tts service returned status %d: %s", resp.StatusCode, string(payload))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// TTSDeleteVoice elimina una voz y su muestra del volumen del servicio.
func TTSDeleteVoice(ctx context.Context, voiceID int64) error {
	req, err := ttsRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/voices/%d", voiceID), nil)
	if err != nil {
		return err
	}

	resp, err := ttsClient().Do(req)
	if err != nil {
		return fmt.Errorf("tts delete voice request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("tts service returned status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// TTSVoiceQuality devuelve el informe de calidad de una muestra de voz.
func TTSVoiceQuality(ctx context.Context, voiceID int64) (map[string]interface{}, error) {
	req, err := ttsRequest(ctx, http.MethodGet, fmt.Sprintf("/api/voices/%d/quality", voiceID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := ttsClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts voice quality request failed: %w", err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tts service returned status %d: %s", resp.StatusCode, string(payload))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// TTSSynthesizePreview genera un audio de forma sincrona. Se reserva para
// pruebas y uso interno: la sintesis de una noticia pasa por la cola.
func TTSSynthesizePreview(ctx context.Context, text, voice, engine string) (map[string]interface{}, error) {
	payload, err := json.Marshal(map[string]string{
		"text":   text,
		"voice":  voice,
		"engine": engine,
	})
	if err != nil {
		return nil, err
	}

	req, err := ttsRequest(ctx, http.MethodPost, "/api/synthesize", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ttsClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts synthesize request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tts service returned status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// TTSSpeaker es un hablante del modelo base (MeloTTS tiene 256 en español).
type TTSSpeaker struct {
	SpeakerID string `json:"speaker_id"`
	Language  string `json:"language"`
	Engine    string `json:"engine"`
}

// TTSVoicesResponse es el catalogo de voces del motor.
type TTSVoicesResponse struct {
	Engine   string       `json:"engine"`
	Speakers []TTSSpeaker `json:"speakers"`
}

// TTSModelSpeakers lista los hablantes disponibles en el modelo base.
func TTSModelSpeakers(ctx context.Context, engine string) (*TTSVoicesResponse, error) {
	path := "/api/speakers"
	if engine != "" {
		path += "?engine=" + engine
	}
	req, err := ttsRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := ttsClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts speakers request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tts service returned status %d: %s", resp.StatusCode, string(body))
	}

	var result TTSVoicesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TTSRegisterSpeaker registra un hablante del modelo como voz utilizable.
func TTSRegisterSpeaker(ctx context.Context, speakerID int, engine, displayName string) (map[string]interface{}, error) {
	payload := map[string]interface{}{"speaker_id": speakerID}
	if engine != "" {
		payload["engine"] = engine
	}
	if displayName != "" {
		payload["display_name"] = displayName
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := ttsRequest(ctx, http.MethodPost, "/api/speakers", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ttsClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts register speaker request failed: %w", err)
	}
	defer resp.Body.Close()

	payloadBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("tts service returned status %d: %s", resp.StatusCode, string(payloadBytes))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// TTSConfigured indica si hay un servicio de TTS configurado.
func TTSConfigured() bool {
	return cfg != nil && cfg.TTSURL != ""
}
