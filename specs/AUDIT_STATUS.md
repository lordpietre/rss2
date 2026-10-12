# RSS2 Intelligence Center - Estado de Auditoría

## Fecha: 2026-10-12 (Actualizado)

## Estado del Sistema ✅

### Servicios - Todos Healthy
```
rss2_backend          Up ~15 min (healthy)
rss2_db               Up ~8 hours (healthy)
rss2_frontend         Up ~30 min (healthy)
rss2_nginx            Up ~30 min (healthy)
rss2_cooccurrences    Up ~5 min (healthy) - nuevo worker
(+ 13 workers adicionales - todos healthy)
```

### Datos - Verificados ✅
| Métrica | Valor | Fuente |
|---------|-------|--------|
| Total News | 1,284,617 | `/api/stats` |
| News Today | 18,440 | `/api/stats` |
| Total Feeds | 28,704 | `/api/stats` |
| Countries | 195 | `/api/countries` |
| Categories | 15 | `/api/categories` |
| Translations | 194,324 | `/api/stats` |
| Entity Cooccurrences | 375,120 | `entity_cooccurrences` table |

### Top Countries (por volumen de noticias)
```
1. Estados Unidos: 618,935
2. Reino Unido: 100,782
3. Brasil: 47,506
4. Rusia: 27,731
5. Alemania: 26,977
```

## Funcionalidades Implementadas

### ✅ Páginas Principales
| Ruta | Página | Estado |
|------|--------|--------|
| `/overview` | Intelligence Overview | ✅ Dashboard con métricas reales |
| `/map` | Global Monitor | ✅ Mapa SVG + distribución |
| `/search` | News Intelligence | ✅ Búsqueda + filtros |
| `/stats` | Trends Radar | ✅ Entidades trending |
| `/alertas` | Alert Center | ✅ Con auth |
| `/feeds` | Sources Management | ✅ CRUD completo |
| `/populares` | Entity Network | ✅ Grafo con co-occurrences |
| `/analisis` | News Evolution | ✅ Análisis temporal |
| `/admin/workers` | System Status | ✅ Observabilidad |

### ✅ Componentes Visuales
| Componente | Estado | Notas |
|------------|--------|-------|
| IntelligenceLayout | ✅ | Sidebar + Topbar + Reloj |
| Dark Theme | ✅ | CSS variables + paleta fosforito |
| EntityGraph | ✅ | SVG force layout, 371 líneas |
| WorldMap | ✅ | SVG interactivo |
| LineChart | ✅ | Gráfico temporal |
| MetricCard | ✅ | Métricas con trends |
| AlertPanel | ✅ | Con badge de nuevas |

### ✅ Backend API
| Endpoint | Estado | Datos |
|----------|--------|-------|
| `GET /api/stats` | ✅ | Métricas globales |
| `GET /api/news` | ✅ | Lista paginada |
| `GET /api/entities/graph` | ✅ | Nodes + Links |
| `GET /api/countries` | ✅ | 195 países |
| `GET /api/categories` | ✅ | 15 categorías |
| `GET /api/entities/news` | ✅ | Noticias por entidad |
| `GET /api/entities/mentions` | ✅ | Serie temporal |

## Arquitectura de Entidades

```
┌─────────────────────────────────────────────────────────────┐
│                    Entity Network Architecture               │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐    ┌──────────────────┐    ┌─────────────┐ │
│  │   tags      │───▶│  tags_noticia  │◀───│  noticias   │ │
│  │  (1.7M)     │    │   (2.4M)       │    │  (1.28M)   │ │
│  └─────────────┘    └──────────────────┘    └─────────────┘ │
│                            │                                │
│                            ▼                                │
│                   ┌─────────────────────┐                   │
│                   │ entity_cooccurrences│                   │
│                   │    (375,120)        │                   │
│                   │ Pre-calculated links│                   │
│                   └─────────────────────┘                   │
│                            │                                │
│                            ▼                                │
│                   ┌─────────────────────┐                   │
│                   │  /api/entities/    │                   │
│                   │     graph          │                   │
│                   │ Returns nodes +    │                   │
│                   │ links for SVG viz  │                   │
│                   └─────────────────────┘                   │
└─────────────────────────────────────────────────────────────┘
```

## Worker de Co-occurrences

```yaml
cooccurrences:
  image: rss2-backend
  entrypoint: ["/cooccurrences"]
  interval: 24h  # Por defecto
  # También soporta --once para ejecución manual
```

El worker:
1. Trunca la tabla `entity_cooccurrences`
2. Recalcula co-occurrences de últimos 30 días
3. Requiere >= 3 co-apariciones para incluir relación

## Limitaciones Conocidas

### 1. Entity Graph por Tipo
Cuando se filtra por `tipo=persona`, puede devolver 0 nodos porque:
- Las personas tienen menos co-occurrences que lugares/temas
- El ranking se hace por co-count total, no por tipo

**Solución temporal**: Usar `limit=50` o mayor para obtener más nodos.

### 2. Co-occurrences Limitadas a 100
El endpoint devuelve máximo 100 links para evitar sobrecarga.

### 3. Top Co-occurring Son Mayormente Lugares/Temas
```
Top 5 entities por co-occurrences:
1. semana (tema)
2. Estados Unidos (lugar)
3. elecciones (tema)
4. partido (tema)
5. Rusia (lugar)
```
Esto es esperado - eventos como "elecciones en Estados Unidos" generan muchas co-occurrences.

## Commits (Sesión Actual)

```
5c16f10 feat: add cooccurrences worker for entity graph updates
5005b7f docs: update AUDIT_STATUS with current implementation state
18bc2fc feat: entity co-occurrences table and pre-computed links
c2c11e2 fix: eliminate infinite loop in EntityGraph force simulation
7118795 feat: add EntityGraph component for network visualization
```

## Pruebas Realizadas

```bash
# API Tests
curl "http://localhost:8888/api/stats"                    # ✅ 1.28M news
curl "http://localhost:8888/api/entities/graph?limit=5"    # ✅ nodes + links
curl "http://localhost:8888/api/countries"                # ✅ 195 countries
curl "http://localhost:8888/api/categories"                # ✅ 15 categories

# Frontend
curl "http://localhost:8888/"                             # ✅ SPA
curl "http://localhost:8888/feeds"                          # ✅ SPA

# Database
docker exec rss2_db psql -U rss -d rss -c "SELECT COUNT(*) FROM entity_cooccurrences"
# Resultado: 375,120 registros
```

## Métricas de Rendimiento

| Operación | Tiempo |
|-----------|--------|
| `GET /api/stats` | ~50ms |
| `GET /api/entities/graph?limit=10` | ~200ms |
| `refresh_all_entity_cooccurrences()` | ~5-10 min |
| Frontend build | ~20s |

## Próximos Pasos (Opcionales)

1. **Optimizar filtrado por tipo** en EntityGraph para priorizar entidades del tipo solicitado
2. **Reducir ventana de co-occurrences** de 30 a 14 días para mayor实时
3. **Añadir paginación** a endpoint de noticias
4. **Testing de responsive** en tablet/móvil
5. **Optimizar queries** de tendencias si hay lentitud
