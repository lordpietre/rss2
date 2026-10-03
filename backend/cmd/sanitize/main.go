// Command sanitize aplica la limpieza de textclean a las noticias que ya están
// en la base de datos (backfill) y, opcionalmente, re-encola sus traducciones.
//
// Uso típico:
//
//	go run ./cmd/sanitize                  # dry-run: qué cambiaría (no escribe)
//	go run ./cmd/sanitize -apply           # aplica; guarda el original en *_raw
//	go run ./cmd/sanitize -apply -requeue  # además re-traduce lo que cambió
//	go run ./cmd/sanitize -restore         # deshace desde *_raw
//
// Propiedades:
//
//   - Idempotente: solo procesa filas con cleaned_at IS NULL y solo guarda
//     *_raw cuando el texto realmente cambia, así que una segunda pasada no
//     modifica nada (y nunca pisa el original guardado).
//   - Reversible: el texto original queda en noticias.titulo_raw/resumen_raw.
//   - Seguro con Ctrl+C: cada lote es una transacción; se puede reanudar.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rss2/backend/internal/textclean"
	"github.com/rss2/backend/internal/workers"
)

var (
	apply      = flag.Bool("apply", false, "aplica los cambios (por defecto solo dry-run)")
	requeue    = flag.Bool("requeue", false, "re-encola traducciones de las noticias cuyo texto cambió")
	restore    = flag.Bool("restore", false, "deshace la limpieza restaurando desde *_raw")
	batchSize  = flag.Int("batch", 1000, "filas por transacción")
	rowLimit   = flag.Int("limit", 0, "máximo de filas a procesar (0 = todas)")
	minDelta   = flag.Int("min-delta", 50, "diferencia mínima de caracteres para re-encolar la traducción")
	targetLang = flag.String("target-lang", "es", "lang_to de las traducciones a re-encolar")
	verbose    = flag.Bool("v", false, "imprime cada lote")
)

var (
	htmlLeftRe = regexp.MustCompile(`</?[a-zA-Z][^<>]{0,200}>`)
	codeRe     = regexp.MustCompile(`function\(|jQuery\(|document\.querySelector`)
)

type fuenteStat struct {
	n     int
	antes int
	desp  int
}

type stats struct {
	procesadas   int
	modificadas  int
	cambioGrande int
	vacias       int
	antes, desp  int
	htmlAntes    int
	htmlDesp     int
	codeAntes    int
	codeDesp     int
	requeueIDs   []string
	porFuente    map[string]*fuenteStat
	maxAntes     int
	maxDesp      int
}

type noticia struct {
	id      string
	titulo  string
	resumen string
	fuente  string
}

func main() {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := workers.LoadDBConfig()
	if err := workers.Connect(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error de conexión a la base de datos: %v\n", err)
		os.Exit(1)
	}
	pool := workers.GetPool()
	defer workers.Close()

	// El esquema se asegura aquí mismo: el binario puede ejecutarse antes que
	// cualquier despliegue nuevo, sin depender de aplicar migraciones a mano.
	if err := ensureSchema(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "error preparando el esquema: %v\n", err)
		os.Exit(1)
	}

	start := time.Now()
	s := &stats{porFuente: map[string]*fuenteStat{}}

	var err error
	mode := "limpieza"
	if *restore {
		mode = "restauración"
		err = runRestore(ctx, pool, s)
	} else {
		err = runClean(ctx, pool, s)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	s.print(mode, time.Since(start))
	if *requeue && !*apply {
		fmt.Println("\nNOTA: -requeue solo se ejecuta junto a -apply/-restore (el dry-run no escribe).")
	}
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	for _, q := range []string{
		`ALTER TABLE noticias ADD COLUMN IF NOT EXISTS resumen_raw TEXT`,
		`ALTER TABLE noticias ADD COLUMN IF NOT EXISTS titulo_raw TEXT`,
		`ALTER TABLE noticias ADD COLUMN IF NOT EXISTS cleaned_at TIMESTAMP`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

// runClean recorre las noticias con cleaned_at IS NULL (keyset por id) y
// calcula la versión limpia de título y resumen.
func runClean(ctx context.Context, pool *pgxpool.Pool, s *stats) error {
	lastID := ""
	for {
		if *rowLimit > 0 && s.procesadas >= *rowLimit {
			return nil
		}
		limit := *batchSize
		if *rowLimit > 0 && s.procesadas+limit > *rowLimit {
			limit = *rowLimit - s.procesadas
		}

		rows, err := pool.Query(ctx, `
			SELECT id, COALESCE(titulo,''), COALESCE(resumen,''), COALESCE(fuente_nombre,'')
			FROM noticias
			WHERE cleaned_at IS NULL AND id > $1
			ORDER BY id
			LIMIT $2`, lastID, limit)
		if err != nil {
			return err
		}

		batch := make([]noticia, 0, limit)
		for rows.Next() {
			var n noticia
			if err := rows.Scan(&n.id, &n.titulo, &n.resumen, &n.fuente); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, n)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		lastID = batch[len(batch)-1].id

		if err := processBatch(ctx, pool, batch, s); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// runRestore deshace la limpieza de las filas que tienen original guardado.
func runRestore(ctx context.Context, pool *pgxpool.Pool, s *stats) error {
	lastID := ""
	for {
		if *rowLimit > 0 && s.procesadas >= *rowLimit {
			return nil
		}
		limit := *batchSize
		if *rowLimit > 0 && s.procesadas+limit > *rowLimit {
			limit = *rowLimit - s.procesadas
		}

		rows, err := pool.Query(ctx, `
			SELECT id, COALESCE(titulo_raw,''), COALESCE(resumen_raw,''), COALESCE(fuente_nombre,'')
			FROM noticias
			WHERE (resumen_raw IS NOT NULL OR titulo_raw IS NOT NULL) AND id > $1
			ORDER BY id
			LIMIT $2`, lastID, limit)
		if err != nil {
			return err
		}

		batch := make([]noticia, 0, limit)
		for rows.Next() {
			var n noticia
			if err := rows.Scan(&n.id, &n.titulo, &n.resumen, &n.fuente); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, n)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		lastID = batch[len(batch)-1].id

		// En restore, "titulo/resumen" traen el ORIGINAL: lo deshacemos.
		b := &pgx.Batch{}
		for _, n := range batch {
			s.procesadas++
			s.modificadas++
			b.Queue(`
				UPDATE noticias
				SET titulo = $1, resumen = $2, titulo_raw = NULL, resumen_raw = NULL, cleaned_at = NULL
				WHERE id = $3`, n.titulo, n.resumen, n.id)
		}
		if *apply {
			br := pool.SendBatch(ctx, b)
			if err := br.Close(); err != nil {
				return err
			}
		}
		if *apply && *requeue {
			ids := make([]string, 0, len(batch))
			for _, n := range batch {
				ids = append(ids, n.id)
			}
			if err := requeueTranslations(ctx, pool, ids); err != nil {
				return err
			}
			s.requeueIDs = append(s.requeueIDs, ids...)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func processBatch(ctx context.Context, pool *pgxpool.Pool, batch []noticia, s *stats) error {
	b := &pgx.Batch{}
	requeueIDs := make([]string, 0)

	for _, n := range batch {
		nt := textclean.CleanTitle(n.titulo)
		nr := textclean.Clean(n.resumen)

		s.procesadas++
		s.antes += len(n.resumen)
		s.desp += len(nr)
		if len(n.resumen) > s.maxAntes {
			s.maxAntes = len(n.resumen)
		}
		if len(nr) > s.maxDesp {
			s.maxDesp = len(nr)
		}
		if htmlLeftRe.MatchString(n.resumen) || htmlLeftRe.MatchString(n.titulo) {
			s.htmlAntes++
		}
		if htmlLeftRe.MatchString(nr) || htmlLeftRe.MatchString(nt) {
			s.htmlDesp++
		}
		if codeRe.MatchString(n.resumen) {
			s.codeAntes++
		}
		if codeRe.MatchString(nr) {
			s.codeDesp++
		}

		f := s.porFuente[n.fuente]
		if f == nil {
			f = &fuenteStat{}
			s.porFuente[n.fuente] = f
		}
		f.n++
		f.antes += len(n.resumen)
		f.desp += len(nr)

		delta := absInt(len(n.resumen)-len(nr)) + absInt(len(n.titulo)-len(nt))
		if nt == n.titulo && nr == n.resumen {
			if *apply {
				b.Queue(`UPDATE noticias SET cleaned_at = NOW() WHERE id = $1`, n.id)
			}
			continue
		}

		s.modificadas++
		if len(nr) < 50 && len(n.resumen) >= 50 {
			s.vacias++
		}
		if delta >= *minDelta {
			s.cambioGrande++
			requeueIDs = append(requeueIDs, n.id)
		}
		if *apply {
			// COALESCE: si la fila ya tuviera original guardado, no se pisa.
			b.Queue(`
				UPDATE noticias
				SET titulo = $1, resumen = $2,
				    titulo_raw = COALESCE(titulo_raw, $3),
				    resumen_raw = COALESCE(resumen_raw, $4),
				    cleaned_at = NOW()
				WHERE id = $5`, nt, nr, n.titulo, n.resumen, n.id)
		}
	}

	if *apply && b.Len() > 0 {
		br := pool.SendBatch(ctx, b)
		if err := br.Close(); err != nil {
			return err
		}
	}
	if *apply && *requeue && len(requeueIDs) > 0 {
		if err := requeueTranslations(ctx, pool, requeueIDs); err != nil {
			return err
		}
	}
	s.requeueIDs = append(s.requeueIDs, requeueIDs...)

	if *verbose {
		fmt.Printf("  lote: %d filas (total %d)\n", len(batch), s.procesadas)
	}
	return nil
}

// requeueTranslations deja la traducción como pendiente y borra los derivados
// (embeddings y tags) para que se regeneren sobre el texto limpio. También pone
// lang=NULL para que langdetect vuelva a detectar el idioma del texto limpio.
func requeueTranslations(ctx context.Context, pool *pgxpool.Pool, ids []string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	lang := *targetLang

	if _, err := tx.Exec(ctx, `
		UPDATE traducciones
		SET titulo_trad = NULL, resumen_trad = NULL, contenido_trad = NULL, status = 'pending',
		    locked_at = NULL, lang_from = NULL, vectorized = FALSE
		WHERE noticia_id = ANY($1) AND lang_to = $2`, ids, lang); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM traduccion_embeddings
		WHERE traduccion_id IN (
			SELECT id FROM traducciones WHERE noticia_id = ANY($1) AND lang_to = $2)`,
		ids, lang); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM tags_noticia
		WHERE traduccion_id IN (
			SELECT id FROM traducciones WHERE noticia_id = ANY($1) AND lang_to = $2)`,
		ids, lang); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE noticias SET lang = NULL WHERE id = ANY($1)`, ids); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *stats) print(mode string, d time.Duration) {
	fmt.Printf("\n=== %s (%s) ===\n", mode, d.Round(time.Millisecond))
	fmt.Printf("filas procesadas : %d\n", s.procesadas)
	fmt.Printf("filas %s: %d (%.1f%%)\n", s.accion(), s.modificadas, pct(s.modificadas, s.procesadas))

	if mode == "restauración" {
		if !*apply {
			fmt.Println("modo DRY-RUN: no se ha escrito nada (usa -apply para aplicar)")
		}
		if *requeue {
			fmt.Printf("re-traducciones : %d\n", len(s.requeueIDs))
		}
		return
	}

	total := s.antes
	ahorro := 0
	if total > 0 {
		ahorro = 100 * (s.antes - s.desp) / total
	}
	fmt.Printf("reducción texto : %d -> %d car. (-%d%%)\n", s.antes, s.desp, ahorro)
	fmt.Printf("tamaño máx.     : %d -> %d car.\n", s.maxAntes, s.maxDesp)
	fmt.Printf("con HTML        : %d -> %d\n", s.htmlAntes, s.htmlDesp)
	fmt.Printf("con código JS   : %d -> %d\n", s.codeAntes, s.codeDesp)
	fmt.Printf("quedan <50 car. : %d (solo titular; no se traduce el cuerpo)\n", s.vacias)
	fmt.Printf("re-traducciones : %d (delta >= %d car.)\n", s.cambioGrande, *minDelta)
	if !*apply {
		fmt.Println("modo DRY-RUN: no se ha escrito nada (usa -apply para aplicar)")
	}

	// Top de fuentes por caracteres ahorrados
	type row struct {
		name   string
		ahorro int
		n      int
	}
	rows := make([]row, 0, len(s.porFuente))
	for name, f := range s.porFuente {
		if d := f.antes - f.desp; d > 0 {
			rows = append(rows, row{name, d, f.n})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ahorro > rows[j].ahorro })
	if len(rows) > 0 {
		fmt.Println("\nTop de fuentes por texto de ruido eliminado:")
		for i, r := range rows {
			if i >= 15 {
				break
			}
			fmt.Printf("  %8d car. (%4d noticias)  %s\n", r.ahorro, r.n, truncateName(r.name))
		}
	}
}

func truncateName(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 45 {
		return s[:45] + "…"
	}
	return s
}

func (s *stats) accion() string {
	switch {
	case *restore:
		return "restauradas"
	case *apply:
		return "cambiadas"
	default:
		return "cambiarían"
	}
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) * 100 / float64(b)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
