# RSS2 - Agente de Mejora de Software

## Visión General del Proyecto

Aplicación de agregación y procesamiento de noticias RSS con capacidades de IA.

### Stack Tecnológico

| Capa | Tecnología |
|------|-------------|
| Frontend | React 18 + TypeScript + Vite + TailwindCSS + React Query + React Router |
| Backend API | Go 1.25 + Gin |
| Workers Backend | Go 1.25, Python 3.x |
| Base de datos | PostgreSQL 18 |
| Caché | Redis 7 |
| Búsqueda vectorial | Qdrant |
| IA/LLM | Ollama, HuggingFace (NLLB, BERT, sentence-transformers) |
| Orquestación | Docker Compose |
| Monitoreo | Prometheus + Grafana + cAdvisor |

### Arquitectura de Workers

- **rss-ingestor-go**: Ingestión de feeds RSS
- **scraper**: Extracción de artículos de URLs
- **discovery**: Descubrimiento de nuevos feeds
- **topics**: Matching de temas y países
- **related**: Noticias relacionadas
- **langdetect**: Detección de idioma
- **translator**: Traducción con NLLB (CPU/GPU)
- **embeddings**: Generación de vectores semánticos
- **ner**: Extracción de entidades nombradas
- **cluster**: Agrupación de noticias
- **llm-categorizer**: Categorización con Ollama
- **wiki-worker**: Wikipedia info y thumbnails

---

## Recomendaciones de Mejora

### 1. Infraestructura y DevOps

- **Añadir CI/CD**: Pipeline con GitHub Actions para testing y deploy automático
- **Health checks**: Implementar health checks en todos los workers (no solo en db/redis)
- **Logging centralizado**: Integrar Loki o ELK stack para agregación de logs
- **Variables de entorno**: Mover configuración hardcodeada (ej: `translator_type: cpu`) a variables与环境

### 2. Backend Go

- **Estructura**: El proyecto no tiene estructura clara de packages (cmd/ internal/)
- **Testing**: No se observan tests unitarios en el backend
- **Config**: Usar Viper o library dedicada para configuración
- **Graceful shutdown**: Implementar en todos los workers
- **Migraciones**: Usar herramientas como golang-migrate en lugar de crear tablas en initDB()

### 3. Frontend

- **E2E Testing**: Añadir Playwright o Cypress
- **State Management**: Considerar Zustand o Jotai para estado global
- **Componentes**: Crear library de componentes reutilizables
- **i18n**: Preparar para multiidioma
- **PWA**: Añadir Service Worker para offline

### 4. Base de Datos

- **ORM/Query Builder**: Considerar GORM o sqlc para mejor mantenimiento
- **Índices**: Verificar que todos los campos de búsqueda tengan índices
- **Migrations**: Sistema formal de migraciones
- **Connection Pooling**: Configurar apropiadamente

### 5. Workers y Procesamiento

- **Colas**: Implementar RabbitMQ o Redis Streams para jobs asíncronos
- **Retry Logic**: Sistema robusto de reintentos con backoff exponencial
- **Dead Letter Queue**: Manejo de trabajos fallidos
- **Métricas por worker**: Prometheus metrics específicas por worker
- **Rate Limiting**: Protecciones contra rate limits de APIs externas

### 6. Seguridad

- **Rate limiting API**: Implementar en endpoints sensibles
- **Input Validation**: Usar biblioteca como go-playground/validator
- **CSRF Protection**: Añadir tokens CSRF
- **Security Headers**: Implementar todos los headers de seguridad
- **Audit Logs**: Logging de acciones administrativas

### 7. Rendimiento

- **Caching**: Cachear más endpoints con Redis (ej: estadísticas)
- **Pagination**: Implementar cursor-based pagination para grandes datasets
- **Database Connection Pooling**: Configurar límites apropiados
- **Image Optimization**: Comprimir imágenes antes de servir

### 8. Documentación

- **API Docs**: OpenAPI/Swagger para la REST API
- **Arquitecture Decision Records (ADRs)**: Documentar decisiones técnicas
- **Runbooks**: Documentación de operaciones

### 9. Monitorización y Alertas

- **Alertas**: Configurar alertas en Grafana para workers caídos
- **Dashboards**: Dashboard específico por worker
- **Tracing**: Añadir OpenTelemetry para distributed tracing

### 10. User Experience

- **Dark Mode**: Soporte completo
- **Keyboard Shortcuts**: Mejorar navegación
- **Infinite Scroll**: Para feeds grandes
- **Real-time Updates**: WebSockets para nuevas noticias
