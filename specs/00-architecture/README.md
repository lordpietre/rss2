# Architecture Documentation

Documentación arquitectónica de RSS2. Esta sección describe la estructura
y diseño del sistema de forma as-built (verificado contra código en ejecución).

## Índice

| Documento | Descripción |
|-----------|-------------|
| [01-system-context.md](01-system-context.md) | Contexto, usuarios, sistemas externos |
| [02-container-view.md](02-container-view.md) | Topología de contenedores Docker |
| [03-data-flow.md](03-data-flow.md) | Pipeline de datos completo |
| [04-component-inventory.md](04-component-inventory.md) | Componentes, APIs, esquemas |
| [05-spec-driven-development.md](05-spec-driven-development.md) | Metodología de desarrollo |
| [06-roadmap.md](06-roadmap.md) | Estado actual y próximo desarrollo |

## Quick reference

### Flujo de datos

```
RSS Feeds → ingestor → noticias → langdetect → translation-scheduler 
    → translator (NLLB) → ner → embeddings → related/wiki/topics
```

### Servicios core

| Servicio | Tecnología | Propósito |
|----------|------------|-----------|
| backend | Go + Gin | API REST |
| translator | Python + CTranslate2 | NLLB-200 translation |
| ner | Python + spaCy | Named entity recognition |
| embeddings | Python + MiniLM | Semantic vectors |
| ingestor | Go | RSS ingestion |

### Bases de datos

| Store | Motor | Uso |
|-------|-------|-----|
| Primary | Postgres 16 | Fuente de verdad |
| Cache | Redis 7 | Caché reconstruible |

## Mantenimiento

Para actualizar esta documentación:
1. Crear/editar archivo en `specs/00-architecture/`
2. Verificar contra código real (`docker exec`, `psql`, logs)
3. Incluir evidencia (logs, queries, métricas)
4. Actualizar `06-roadmap.md` si cambia estado
