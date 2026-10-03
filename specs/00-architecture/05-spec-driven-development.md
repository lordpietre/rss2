# Spec-Driven Development (SDD) — Plan para RSS2

## Principios

1. **Specs antes que código**: toda funcionalidad nueva requiere spec primero
2. **Verificación antes de cerrar**: tests y evidencia antes de marcar hecho
3. **Diff mínimo**: solo cambiar lo necesario
4. **Documentar decisiones**: el "por qué" junto al "qué"

## Flujo de desarrollo

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                 CICLO SDD                                            │
└─────────────────────────────────────────────────────────────────────────────────────┘

  IDEA ──▶ SPEC ──▶ CODE ──▶ TEST ──▶ REVIEW ──▶ DEPLOY ──▶ DOCUMENT
             │                                │            │
             │                                │            ▼
             │                                │        specs/
             │                                │        actual-
             │                                │        izado
             │                                │
             └────────────────────────────────┘
                    (si falla test/review)
```

## Estructura de specs

```
specs/
├── 00-architecture/     # Documentación arquitectónica
│   ├── 01-system-context.md
│   ├── 02-container-view.md
│   ├── 03-data-flow.md
│   └── 04-component-inventory.md
├── 01-system-overview/  # Visión general del sistema
├── 02-ingestion/         # Pipeline de ingestión RSS
├── 03-translation/      # Traducción NLLB
├── 04-enrichment/       # NER, embeddings, wiki, topics
├── 05-api-backend/      # Contratos HTTP
├── 06-frontend/          # UI/UX
├── 07-pending-workers/   # Workers no implementados
├── 08-operations/        # Deploy, backup, monitoring
├── constitution.md        # Reglas obligatorias
├── plan.md               # Roadmap general
└── tasks.md              # Backlog de tareas
```

## Template para nueva spec

```markdown
# Spec NN — [Título]

## Objetivo
[Qué problema resuelve]

## Decisiones de diseño
[Opciones consideradas y justificación]

## Contrato
[API, schema, comportamiento]

## Métricas de éxito
[Criterios medibles]

## Tareas de implementación
- [ ] Subtarea 1
- [ ] Subtarea 2

## Estado
- [ ] Propuesta
- [ ] Aprobada
- [ ] Implementada
- [ ] Verificada
```

## Checklist de cambio

Para cada cambio, verificar:

### Antes de codificar
- [ ] Spec creada y aprobada
- [ ] Esquema BD actualizado si aplica
- [ ] Contratos HTTP definidos si aplica
- [ ] Métricas de éxito establecidas

### Antes de commit
- [ ] Compila localmente
- [ ] `go vet ./...` limpio (Go)
- [ ] `tsc --noEmit` limpio (TypeScript)
- [ ] Tests unitarios passing
- [ ] Datos de prueba准备好了

### Antes de desplegar
- [ ] Build en Docker exitoso
- [ ] Healthchecks definidos
- [ ] Variables de entorno documentadas
- [ ] Rollback planificado

### Después de desplegar
- [ ] Verificado contra entorno real
- [ ] Logs sin errores
- [ ] Métricas dentro de esperados
- [ ] Spec actualizada si cambió comportamiento

## Categorías de tareas

### Crítico (P0)
- Bugs que rompen funcionalidad principal
- Problemas de seguridad
- Pérdida de datos

### Importante (P1)
- Bugs que afectan flujo principal
- Mejoras de rendimiento >20%
- Features prometidos a usuarios

### Normal (P2)
- Bugs menores
- Mejoras de UX
- Documentación

### Técnico (P3)
- Refactoring
- Deuda técnica
- Optimizaciones

## Ritmo de desarrollo

| Fase | Frecuencia | Entregable |
|------|------------|------------|
| Daily | diaria | estado en tasks.md |
| Weekly | semanal | spec nueva o update |
| Sprint | 2 semanas | features completados |
| Retro | mensual | lecciones aprendidas |

## Revisión de specs

### Quién aprueba

| Tipo | Aprobador |
|------|-----------|
| Arquitetura | Owner del proyecto |
| API/backend | Lead backend |
| Frontend | Lead frontend |
| Datos | Lead datos |
| Seguridad | Owner |

### Criterios de aprobación

- [ ] Objetivo claro y medible
- [ ] Alternativas consideradas
- [ ] Riesgos identificados
- [ ] Impacto en existentes evaluado
- [ ] Tests de integración definidos

## Mejora continua

### Retrospectiva técnica
- Revisar specs que resultó incorrectas
- Identificar gaps en verificación
- Actualizar constitution.md si necesario

### Metrics a trackear
- Tiempo de spec → deploy
- Bugs escapados a producción
- Cobertura de tests
- Deuda técnica (tickets abiertos)

## Fuentes de verdad (constitution.md)

> 1. **Esquema BD**: `init-db/*.sql`
> 2. **Despliegue**: `docker-compose.yml`
> 3. **Contratos HTTP**: `backend/cmd/server/main.go` + types
> 4. **Specs**: `specs/`

Orden de prioridad ante contradicciones.
