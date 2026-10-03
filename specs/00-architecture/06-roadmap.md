# Architecture 06 — Roadmap y Próximos Pasos

## Estado actual del sistema (2026-10-01)

| Componente | Estado | Notas |
|------------|--------|-------|
| Ingesta RSS | ✅ Operativo | 6822 feeds, 1460 activos |
| langdetect | ✅ Operativo | Detecta idioma automáticamente |
| translation-scheduler | ✅ Operativo | Crea jobs cada 30s |
| translator (NLLB) | ✅ Operativo | 2 réplicas CPU, 1024 tokens |
| ner | ✅ Operativo | spaCy es_core_news_lg |
| embeddings | ✅ Operativo | MiniLM-L12-v2 |
| related | ✅ Operativo | Coseno SQL sobre embeddings |
| wiki | ✅ Operativo | Wikipedia API |
| topics | ✅ Operativo | Clasificación automática |
| scraper | ✅ Operativo | Enriquece resúmenes |
| backend API | ✅ Operativo | Go + Gin |
| frontend | ✅ Operativo | React SPA |

## Métricas actuales

```
noticias:          ~178,000
traducciones done: ~700 (batch nuevo 1024 tokens)
traducciones pending: ~164,000
feeds activos:      1,460 / 6,822
tags:              ~31,000
```

## Roadmap 2026 Q4

### Fase 1: Estabilización (2 semanas)

#### 1.1 Limpiar backlog de traducciones
- Estado: **En progreso**
- ~164,000 pending con límite 1024 tokens
- Estimado: 30-40 horas (100/hora × 2 réplicas)
- Métrica: traducciones done > 150,000

#### 1.2 Re-procesar chrome residual
- Estado: **Pendiente**
- 930 noticias con `window._taboola` u otro JS residual
- Comando: `docker exec rss2_backend sanitize -apply -requeue`
- Requiere: rebuild de translators con código de limpieza

#### 1.3 Verificar calidad de traducción
- Estado: **Pendiente**
- Sample de 100 traducciones con ratio <0.5
- Validar si es comportamiento del modelo o bug
- Ajustar MAX_TOKENS si es necesario

### Fase 2: Mejoras de rendimiento (2 semanas)

#### 2.1 Aumentar réplicas de translator
- Estado: **Propuesto**
- Actualmente: 2 réplicas CPU
- Propuesta: 4 réplicas (requires more RAM)
- Impacto estimado: 2x throughput

#### 2.2 Optimizar chunking
- Estado: **Propuesto**
- Analizar distribución de ratios por idioma
- Idiomas problemáticos: tr (0.23-0.60), da (0.43), ru (0.43)
- Ajustar regex de chunking por idioma

#### 2.3 Cache de traducciones
- Estado: **Propuesto**
- Redis caching para títulos y cuerpos
- TTL: 24 horas
- Impacto estimado: 30-50% menos llamadas al modelo

### Fase 3: Nuevas features (4 semanas)

#### 3.1 Alertas mejoradas
- Estado: **En spec** (specs/04-enrichment)
- Nuevo algoritmo con umbrales ajustados
- Filtro por blocklist de entidades
- Métrica: reducir 169 alertas/día → ~20 alertas/día

#### 3.2 Búsqueda semántica
- Estado: **Deprecado**
- Originalmente con Qdrant
- Eliminado (no se usaba)
- Alternativa: embeddings + coseno SQL ya existe

#### 3.3 Mejoras UI
- Estado: **Pendiente**
- ErrorBoundary en frontend
- Lazy loading de imágenes
- Infinite scroll en noticias

### Fase 4: Operaciones (continuo)

#### 4.1 Backup automático
- Estado: **Manual** (backup.sh existe)
- Automatizar con cron
- Verificación de integridad

#### 4.2 Monitoring
- Estado: **Parcial** (prometheus/grafana)
- Dashboards de翻译 backlog
- Alertas de salud de workers

#### 4.3 Rotación de credenciales
- Estado: **Documentado** (specs/08-operations)
- Automatizar procedimiento

## Tickets Prioritarios

| ID | Descripción | Prioridad | Estimación |
|----|-------------|-----------|-------------|
| ARCH-001 | Re-procesar 930 noticias con chrome | P1 | 1h |
| ARCH-002 | Verificar calidad traducción sample | P1 | 2h |
| ARCH-003 | Implementar cache Redis para traducciones | P2 | 4h |
| ARCH-004 | Dashboard Grafana de backlog | P2 | 2h |
| ARCH-005 | Ajuste de alertas (algoritmo) | P2 | 8h |

## Decisiones pendientes

1. **¿4 réplicas de translator?**
   - Requiere: 8GB RAM adicional
   - Impacto: 2x throughput
   - Decisión: 2026-10-15

2. **¿Aumentar MAX_TOKENS a 2048?**
   - Tradeoff: más lentitud vs mejor calidad
   - Decisión: después de verificar calidad actual

3. **¿Eliminar feeds inactivos?**
   - 5182 feeds sin actividad
   - Limpieza: `UPDATE fuentes_url SET active=false WHERE last_check < NOW() - INTERVAL '30 days'`
   - Decisión: pendiente análisis

## Métricas objetivo Q4

| Métrica | Actual | Objetivo Q4 |
|---------|--------|-------------|
| Traducciones completadas | ~700 | >150,000 |
| Throughput traducción | ~100/h | ~200/h |
| Alertas/día | 169 | <25 |
| Chrome residual | 930 | 0 |
| Feeds activos | 1,460 | >2,000 |
