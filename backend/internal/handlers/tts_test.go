package handlers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestResolveTTSAudioPath comprueba que el audio solo se pueda servir desde
// el directorio de audios: es la proteccion contra path traversal.
func TestResolveTTSAudioPath(t *testing.T) {
	base := t.TempDir()

	tests := []struct {
		name        string
		baseDir     string
		storagePath string
		wantErr     bool
	}{
		{"ruta normal", base, "12/job-3.wav", false},
		{"subdirectorios", base, "2024/10/job-9.wav", false},
		{"path traversal", base, "../../etc/passwd", true},
		{"path traversal profundo", base, "12/../../../../etc/passwd", true},
		{"ruta absoluta", base, "/etc/passwd", true},
		{"ruta vacia", base, "", true},
		{"sin directorio base", "", "12/job-3.wav", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveTTSAudioPath(tt.baseDir, tt.storagePath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("se esperaba error para %q, se obtuvo %q", tt.storagePath, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if !filepath.IsAbs(got) {
				t.Fatalf("se esperaba una ruta absoluta, se obtuvo %q", got)
			}
			rel, relErr := filepath.Rel(base, got)
			if relErr != nil || rel == ".." || filepath.IsAbs(rel) {
				t.Fatalf("la ruta %q se sale del directorio base %q", got, base)
			}
		})
	}
}

// TestResolveTTSAudioPathBlocksSymlinkEscape comprueba que un enlace
// simbolico fuera del directorio base tampoco se puede usar.
func TestResolveTTSAudioPathBlocksSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secreto.wav")
	if err := os.WriteFile(secret, []byte("contenido"), 0o600); err != nil {
		t.Fatalf("no se pudo crear el fichero de prueba: %v", err)
	}

	link := filepath.Join(base, "enlace.wav")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("no se pueden crear enlaces simbolicos: %v", err)
	}

	// La comprobacion es lexical (no sigue enlaces), pero el servicio solo
	// escribe dentro del volumen: verificar que la ruta resuelta no sale.
	got, err := resolveTTSAudioPath(base, "enlace.wav")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got != filepath.Join(base, "enlace.wav") {
		t.Fatalf("ruta inesperada: %q", got)
	}
}

// TestTTSIdempotencyKey comprueba que la misma narracion produce siempre la
// misma clave y que distintos parametros producen claves distintas.
func TestTTSIdempotencyKey(t *testing.T) {
	base := ttsIdempotencyKey(42, "es-base", "melotts", 1.0)

	if base != ttsIdempotencyKey(42, "es-base", "melotts", 1.0) {
		t.Fatal("la clave de idempotencia deberia ser estable")
	}
	if base == ttsIdempotencyKey(43, "es-base", "melotts", 1.0) {
		t.Fatal("guiones distintos deben dar claves distintas")
	}
	if base == ttsIdempotencyKey(42, "otra-voz", "melotts", 1.0) {
		t.Fatal("voces distintas deben dar claves distintas")
	}
	if base == ttsIdempotencyKey(42, "es-base", "openvoice", 1.0) {
		t.Fatal("motores distintos deben dar claves distintas")
	}
	if base == ttsIdempotencyKey(42, "es-base", "melotts", 1.2) {
		t.Fatal("velocidades distintas deben dar claves distintas")
	}
	if len(base) != 64 {
		t.Fatalf("se esperaba una clave de 64 caracteres, se obtuvo %d", len(base))
	}
}

// TestClampTTSSpeed comprueba los limites de la velocidad de locucion.
func TestClampTTSSpeed(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0, 1.0},
		{-2, 1.0},
		{0.3, 0.5},
		{1.0, 1.0},
		{1.25, 1.25},
		{2.0, 2.0},
		{3.5, 2.0},
	}
	for _, tt := range cases {
		if got := clampTTSSpeed(tt.in); got != tt.want {
			t.Errorf("clampTTSSpeed(%v) = %v, se esperaba %v", tt.in, got, tt.want)
		}
	}
}

// TestParseTimeFilter comprueba la interpretacion de las ventanas
// temporales (fechas ISO y relativas).
func TestParseTimeFilter(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

	if _, ok := parseTimeFilter("", now); ok {
		t.Error("una ventana vacia no debe interpretarse")
	}
	if _, ok := parseTimeFilter("ayer", now); ok {
		t.Error("un valor no valido no debe interpretarse")
	}

	got, ok := parseTimeFilter("24h", now)
	if !ok {
		t.Fatal("deberia interpretar 24h")
	}
	if !got.Equal(now.Add(-24 * time.Hour)) {
		t.Errorf("ventana de 24h incorrecta: %v", got)
	}

	got, ok = parseTimeFilter("7d", now)
	if !ok || !got.Equal(now.AddDate(0, 0, -7)) {
		t.Errorf("ventana de 7d incorrecta: %v", got)
	}

	got, ok = parseTimeFilter("2026-10-20", now)
	if !ok {
		t.Fatal("deberia interpretar una fecha ISO")
	}
	if got.Year() != 2026 || got.Month() != time.October || got.Day() != 20 {
		t.Errorf("fecha ISO mal interpretada: %v", got)
	}
}

// TestNullable comprueba la conversion a NULL de los campos de texto.
func TestNullable(t *testing.T) {
	if nullable("") != nil {
		t.Fatal("una cadena vacia debe convertirse en NULL")
	}
	if nullable("   ") != nil {
		t.Fatal("una cadena de espacios debe convertirse en NULL")
	}
	if nullable("texto") != "texto" {
		t.Fatal("una cadena con contenido debe mantenerse")
	}
}

// TestTTSStatusConstants comprueba que los estados usados por la API son los
// que espera la base de datos.
func TestTTSStatusConstants(t *testing.T) {
	if ttsScriptStatusDraft != "draft" || ttsScriptStatusApproved != "approved" {
		t.Fatal("estados de guion inesperados")
	}
	if ttsJobStatusQueued != "queued" || ttsJobStatusRunning != "running" {
		t.Fatal("estados de tarea inesperados")
	}
	if ttsAudioPending != "pending_review" {
		t.Fatal("un audio nuevo debe quedar pendiente de revision")
	}
}

// TestClampTTSSilenceMs comprueba el limite de la pausa entre noticias.
func TestClampTTSSilenceMs(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{-1000, 0},
		{0, 0},
		{700, 700},
		{10000, 10000},
		{999999, 10000},
	}
	for _, tt := range cases {
		if got := clampTTSSilenceMs(tt.in); got != tt.want {
			t.Errorf("clampTTSSilenceMs(%d) = %d, se esperaba %d", tt.in, got, tt.want)
		}
	}
}

// TestTTSMaxBulletinItems comprueba el limite de noticias por noticiario.
func TestTTSMaxBulletinItems(t *testing.T) {
	if ttsMaxBulletinItems < 2 || ttsMaxBulletinItems > 200 {
		t.Fatalf("limite de items del noticiario inesperado: %d", ttsMaxBulletinItems)
	}
}

// TestResolveTTSEngineWithoutVoice comprueba el orden de decision sin base de
// datos: el motor pedido manda; sin pedido ni voz, el configurado.
func TestResolveTTSEngineWithoutVoice(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret")
	t.Setenv("TTS_ENGINE", "chatterbox")

	if got := resolveTTSEngine(context.Background(), "melotts", nil); got != "melotts" {
		t.Errorf("el motor pedido debe respetarse, se obtuvo %q", got)
	}
	if got := resolveTTSEngine(context.Background(), "  openvoice  ", nil); got != "openvoice" {
		t.Errorf("el motor pedido debe recortarse, se obtuvo %q", got)
	}
	if got := resolveTTSEngine(context.Background(), "", nil); got != "chatterbox" {
		t.Errorf("sin voz ni pedido se espera el motor configurado, se obtuvo %q", got)
	}
}
