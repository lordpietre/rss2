// Command entityscan recorre TODOS los valores de `tags` (todos los países y
// tipos) y los clasifica con `entitycheck` para detectar falsos positivos del
// NER.
//
// Uso:
//
//	go run ./cmd/entityscan                # dry-run: informe, no escribe
//	go run ./cmd/entityscan -apply         # siembra entity_blocklist
//	go run ./cmd/entityscan -apply -tipo persona -min-mentions 2
//	go run ./cmd/entityscan -unblock "Nombre Raro" -tipo persona
//	go run ./cmd/entityscan -alertas       # poda alertas heredadas (dry-run)
//	go run ./cmd/entityscan -alertas -apply
//
// La blocklist es la única fuente de verdad del descarte: la leen
// `handlers.GetEntities`/`GetEntityNews` (Populares deja de mostrarlos) y
// `workers/ner_worker.py` (ya no se vuelven a insertar). Los FP débiles solo
// se informan: decidirlos es cosa de una persona.
//
// Propiedades: idempotente (UNIQUE (tipo, valor_lower)), seguro con Ctrl+C y
// reversible con -unblock.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rss2/backend/internal/entitycheck"
	"github.com/rss2/backend/internal/workers"
)

var (
	apply       = flag.Bool("apply", false, "escribe en entity_blocklist (por defecto solo informe)")
	tipo        = flag.String("tipo", "", "restringe a un tipo (persona|organizacion|lugar|tema)")
	minMentions = flag.Int("min-mentions", 1, "solo bloquea valores con al menos N menciones")
	unblock     = flag.String("unblock", "", "quita un valor de la blocklist (junto a -tipo)")
	pruneAlerts = flag.Bool("alertas", false, "revisa `alertas` y marca como descartada las que no cumplen los umbrales vigentes")
	verbose     = flag.Bool("v", false, "imprime cada valor bloqueado")
)

type cuenta struct {
	tipo      string
	valor     string
	menciones int
}

type motivoStat struct {
	valores   int
	menciones int
	ejemplos  []string
}

type stats struct {
	leidos     int
	fuertes    int
	debiles    int
	bloqueados int
	podadas    int // alertas marcadas como descartada (-alertas)
	activas    int // alertas que siguen válidas (-alertas)
	porTipo    map[string]int
	porMotivo  map[string]*motivoStat
	muestra    []cuenta // top por menciones, para revisión humana
}

func main() {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := workers.Connect(workers.LoadDBConfig()); err != nil {
		fmt.Fprintf(os.Stderr, "error de conexión a la base de datos: %v\n", err)
		os.Exit(1)
	}
	pool := workers.GetPool()
	defer workers.Close()

	if err := ensureSchema(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "error preparando el esquema: %v\n", err)
		os.Exit(1)
	}

	start := time.Now()
	s := &stats{porTipo: map[string]int{}, porMotivo: map[string]*motivoStat{}}

	var err error
	mode := "informe de falsos positivos"
	switch {
	case *unblock != "":
		err = runUnblock(ctx, pool, s)
		mode = "desbloqueo"
	case *pruneAlerts:
		err = runAlertPrune(ctx, pool, s)
		mode = "podado de alertas"
	default:
		err = runScan(ctx, pool, s)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	s.print(mode, time.Since(start))
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS entity_blocklist (
			id           BIGSERIAL PRIMARY KEY,
			tipo         TEXT        NOT NULL,
			valor        TEXT        NOT NULL,
			valor_lower  TEXT        NOT NULL,
			motivo       TEXT        NOT NULL,
			menciones    INT         NOT NULL DEFAULT 0,
			origen       TEXT        NOT NULL DEFAULT 'entityscan',
			created_at   TIMESTAMP   NOT NULL DEFAULT NOW(),
			UNIQUE (tipo, valor_lower)
		)`)
	return err
}

func runScan(ctx context.Context, pool *pgxpool.Pool, s *stats) error {
	q := `
		SELECT t.tipo, t.valor, COUNT(*)::int AS menciones
		FROM tags_noticia tn
		JOIN tags t ON t.id = tn.tag_id
		GROUP BY 1, 2`
	args := []interface{}{}
	if *tipo != "" {
		q += ` HAVING t.tipo = $1`
		args = append(args, *tipo)
	}
	q += ` ORDER BY 3 DESC`

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Dry-run y apply comparten el mismo recorrido: la única diferencia es la
	// escritura al final de cada candidato.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // inofensivo si ya se hizo Commit
	commit := func() error {
		if !*apply {
			return nil
		}
		return tx.Commit(ctx)
	}

	for rows.Next() {
		var c cuenta
		if err := rows.Scan(&c.tipo, &c.valor, &c.menciones); err != nil {
			return err
		}
		s.leidos++

		sev, motivo := entitycheck.Classify(c.tipo, c.valor)
		switch sev {
		case entitycheck.SeverityStrong:
			s.fuertes++
			if c.menciones < *minMentions {
				continue
			}
			if !*apply {
				s.record(c, motivo)
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO entity_blocklist (tipo, valor, valor_lower, motivo, menciones)
				VALUES ($1, $2, LOWER($2), $3, $4)
				ON CONFLICT (tipo, valor_lower)
				DO UPDATE SET motivo = EXCLUDED.motivo, menciones = EXCLUDED.menciones`,
				c.tipo, c.valor, motivo, c.menciones); err != nil {
				return err
			}
			s.record(c, motivo)
		case entitycheck.SeverityWeak:
			s.debiles++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := commit(); err != nil {
		return err
	}
	return nil
}

// runAlertPrune revisa las alertas existentes contra los umbrales vigentes
// (entitycheck.AlertaBase/AlertaTema), la blocklist y las reglas FP fuertes.
// Poda las alertas heredadas de antes de racionalizar el algoritmo: no se
// borran, se marcan `status='descartada'` (reversible a mano).
func runAlertPrune(ctx context.Context, pool *pgxpool.Pool, s *stats) error {
	enBlocklist := map[string]bool{}
	rows, err := pool.Query(ctx, `SELECT tipo, valor_lower FROM entity_blocklist`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var tipo, valor string
		if err := rows.Scan(&tipo, &valor); err != nil {
			rows.Close()
			return err
		}
		enBlocklist[strings.ToLower(tipo)+"\x00"+valor] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	alertas, err := pool.Query(ctx, `
		SELECT id, valor, tipo, hits, baseline, ratio
		FROM alertas WHERE status <> 'descartada' ORDER BY id`)
	if err != nil {
		return err
	}
	defer alertas.Close()

	var ids []int64
	for alertas.Next() {
		var id int64
		var valor, tipo string
		var hits int
		var baseline, ratio float64
		if err := alertas.Scan(&id, &valor, &tipo, &hits, &baseline, &ratio); err != nil {
			return err
		}
		s.leidos++

		motivo := ""
		switch {
		case enBlocklist[strings.ToLower(tipo)+"\x00"+strings.ToLower(valor)]:
			motivo = "blocklist"
		default:
			// Igual que el scan: una alerta exige un valor limpio (los FP
			// débiles se podan aquí aunque sigan visibles en Populares).
			if sev, m := entitycheck.Classify(tipo, valor); sev != entitycheck.SeverityNone {
				motivo = "fp: " + m
			} else if ok, m := entitycheck.AlertaValida(tipo, hits, baseline, ratio, 0); !ok {
				motivo = m
			}
		}

		if motivo == "" {
			s.activas++
			if sev, m := entitycheck.Classify(tipo, valor); sev == entitycheck.SeverityWeak {
				s.debiles++
				mm := s.porMotivo["débil: "+m]
				if mm == nil {
					mm = &motivoStat{}
					s.porMotivo["débil: "+m] = mm
				}
				mm.valores++
				mm.menciones += hits
				if len(mm.ejemplos) < 3 {
					mm.ejemplos = append(mm.ejemplos, valor)
				}
			}
			continue
		}
		s.podadas++
		ids = append(ids, id)
		m := s.porMotivo[motivo]
		if m == nil {
			m = &motivoStat{}
			s.porMotivo[motivo] = m
		}
		m.valores++
		m.menciones += hits
		if len(m.ejemplos) < 3 {
			m.ejemplos = append(m.ejemplos, valor)
		}
	}
	if err := alertas.Err(); err != nil {
		return err
	}

	if *apply && len(ids) > 0 {
		if _, err := pool.Exec(ctx,
			`UPDATE alertas SET status = 'descartada' WHERE id = ANY($1)`, ids); err != nil {
			return err
		}
	}
	return nil
}

func runUnblock(ctx context.Context, pool *pgxpool.Pool, s *stats) error {
	if *tipo == "" {
		return fmt.Errorf("-unblock requiere -tipo")
	}
	tag, err := pool.Exec(ctx,
		`DELETE FROM entity_blocklist WHERE tipo = $1 AND valor_lower = LOWER($2)`,
		*tipo, *unblock)
	if err != nil {
		return err
	}
	s.bloqueados = int(tag.RowsAffected())
	return nil
}

func (s *stats) record(c cuenta, motivo string) {
	s.bloqueados++
	s.porTipo[c.tipo]++
	m := s.porMotivo[motivo]
	if m == nil {
		m = &motivoStat{}
		s.porMotivo[motivo] = m
	}
	m.valores++
	m.menciones += c.menciones
	if len(m.ejemplos) < 3 {
		m.ejemplos = append(m.ejemplos, c.valor)
	}
	if *verbose {
		fmt.Printf("  bloqueado [%s] %-10s %5d menc.  %s\n", motivo, c.tipo, c.menciones, trunc(c.valor, 70))
	}
	if len(s.muestra) < 15 {
		s.muestra = append(s.muestra, c)
	}
	sort.Slice(s.muestra, func(i, j int) bool { return s.muestra[i].menciones > s.muestra[j].menciones })
	if len(s.muestra) > 15 {
		s.muestra = s.muestra[:15]
	}
}

func (s *stats) print(mode string, d time.Duration) {
	fmt.Printf("\n=== %s (%.0fs) ===\n", mode, d.Seconds())
	if *pruneAlerts {
		fmt.Printf("alertas revisadas: %d\n", s.leidos)
		fmt.Printf("siguen válidas : %d (%d con FP débil: revisión humana)\n", s.activas, s.debiles)
		fmt.Printf("podadas        : %d %s\n", s.podadas, estado(*apply))
		if len(s.porMotivo) > 0 {
			motivos := make([]string, 0, len(s.porMotivo))
			for k := range s.porMotivo {
				motivos = append(motivos, k)
			}
			sort.Slice(motivos, func(i, j int) bool {
				return s.porMotivo[motivos[i]].valores > s.porMotivo[motivos[j]].valores
			})
			fmt.Println("Por motivo:")
			for _, k := range motivos {
				m := s.porMotivo[k]
				fmt.Printf("  %-28s %5d alertas  ej: %s\n",
					k, m.valores, trunc(strings.Join(m.ejemplos, " | "), 90))
			}
		}
		if !*apply {
			fmt.Println("\nmodo DRY-RUN: no se ha escrito nada (usa -apply para podar).")
		}
		return
	}
	fmt.Printf("valores leídos : %d\n", s.leidos)
	if *unblock != "" {
		fmt.Printf("desbloqueados  : %d\n", s.bloqueados)
		return
	}
	fmt.Printf("FP fuertes     : %d (%.1f%%) — %s\n", s.fuertes,
		pct(s.fuertes, s.leidos), estado(*apply))
	fmt.Printf("FP débiles     : %d (%.1f%%) — solo informe, no se bloquean\n",
		s.debiles, pct(s.debiles, s.leidos))
	if *apply {
		fmt.Printf("en blocklist   : %d\n", s.bloqueados)
	} else {
		fmt.Printf("candidatos     : %d (se bloquearían)\n", s.bloqueados)
	}

	if len(s.porTipo) > 0 {
		tipos := make([]string, 0, len(s.porTipo))
		for k := range s.porTipo {
			tipos = append(tipos, k)
		}
		sort.Strings(tipos)
		fmt.Println("\nPor tipo:")
		for _, k := range tipos {
			fmt.Printf("  %-12s %d\n", k, s.porTipo[k])
		}
	}

	if len(s.porMotivo) > 0 {
		motivos := make([]string, 0, len(s.porMotivo))
		for k := range s.porMotivo {
			motivos = append(motivos, k)
		}
		sort.Slice(motivos, func(i, j int) bool {
			return s.porMotivo[motivos[i]].valores > s.porMotivo[motivos[j]].valores
		})
		fmt.Println("\nPor motivo:")
		for _, k := range motivos {
			m := s.porMotivo[k]
			fmt.Printf("  %-28s %5d valores %6d menciones  ej: %s\n",
				k, m.valores, m.menciones, trunc(strings.Join(m.ejemplos, " | "), 90))
		}
	}

	if len(s.muestra) > 0 && *apply {
		fmt.Println("\nTop por menciones (revisar a mano):")
		for _, c := range s.muestra {
			fmt.Printf("  %5d  %-11s %s\n", c.menciones, c.tipo, trunc(c.valor, 70))
		}
	}

	if !*apply {
		fmt.Println("\nmodo DRY-RUN: no se ha escrito nada (usa -apply para bloquear).")
	}
}

func estado(apply bool) string {
	if apply {
		return "bloqueados"
	}
	return "se bloquearían"
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
