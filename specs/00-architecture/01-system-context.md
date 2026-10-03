# Architecture 01 — Contexto del Sistema

## Sistema: RSS2

Agregador multilingüe de noticias RSS que ingiere feeds, detecta idioma, traduce automáticamente al español usando NLLB, y proporciona búsqueda, alertas de picos de actividad y frontend React.

## Usuarios del sistema

| Actor | Rol | Necesidad |
|-------|-----|-----------|
| Usuario anónimo | Lectura de noticias | Ver noticias traducidas, buscar, filtrar |
| Usuario registrado | Guardar favoritos | Favoritos, alertas personalizadas |
| Administrador | Gestión del sistema | Estadísticas, gestión de fuentes, aliases |
| Sistema externo | Feeds RSS | Publicar contenido via RSS |

## Sistemas externos

| Sistema | Protocolo | Propósito |
|---------|-----------|-----------|
| feeds RSS | HTTP/XML | Fuente de noticias (6822 feeds configurados) |
| Wikipedia | HTTP API | Enriquecimiento con resúmenes de entidades |
| NLLB-200 | CTranslate2 local | Traducción automática multilingüe |
| sentence-transformers | MiniLM-L12-v2 local | Embeddings para búsqueda semántica |

## Fronteras de confianza

```
[Internet] --- RSS Feeds ---> [RSS2 Ingestor]
                                    |
                                    v
                              [Base de Datos]
                                    |
        +-------------+-------------+-------------+-------------+
        |             |             |             |             |
        v             v             v             v             v
   [Traductor]    [NER]       [Embeddings]    [Wiki]      [Topics]
        |             |             |             |             |
        +-------------+-------------+-------------+-------------+
                                    |
                                    v
                            [Backend API]
                                    |
                    +---------------+---------------+
                    |                               |
                    v                               v
              [Frontend]                      [Usuario Final]
```

## Decisiones arquitectónicas clave

1. **NLLB-200-distilled-600M** en CTranslate2 CPU para traducción (int8, 2GB RAM)
2. **Postgres 16** como fuente de verdad (no cola de mensajes)
3. **Redis** solo para caché reconstruible (no estado persistente)
4. **Docker** como unidad de despliegue (un servicio = un contenedor)
5. **Go** para backend API, workers y herramientas
6. **Python** para workers de ML (translator, embeddings, ner)

## Métricas observadas (2026-10-01)

- ~178,000 noticias en BD
- ~164,000 traducciones pending (procesando)
- ~700 traducciones completadas (nuevas con 1024 tokens)
- 6822 feeds RSS configurados
- 1460 feeds activos
- 20 workers de ingestión, polling cada 8 minutos
