# Spec 06 — Frontend (React + Vite + nginx)

## Alcance

SPA servida por `frontend` (build `tsc && vite build`, nginx sirve `dist` y
proxya `/api → backend:8080`). Gateway `:8888` → frontend `/` y backend
`/api/`. `VITE_API_URL=/api`. Cliente `apiService` (24 métodos) + `api`
directo para admin/workers.

## Páginas ↔ endpoints (verificado)

Home(news,categories,countries,suggestions,searchlog) · News(news/:id) ·
Feeds(feeds CRUD+import/export) · Search · Populares(entities,entities/news,
admin retype/aliases/backup) · Stats · Analisis(entities,mentions) ·
Alertas(alerts+read,entities/news) · Favorites/Account(local) ·
Admin*(users,workers+remote,settings,aliases) · Login/Register · Wizard.

## Reglas verificadas (fixes aplicados)

1. **Subidas**: interceptor que borra `Content-Type` con `FormData` (el
   default `application/json` rompía el multipart → `No file provided`).
   Afecta a import feeds, restore BD, import aliases (×2).
2. **nginx**: upstreams al nombre de servicio (`backend`, no `backend-go`).
3. **Login**: duplicado → mensaje en español que manda a iniciar sesión.
4. **Duplicados por merge**: no duplicar bloques JSX con estado inexistente
   (caso AdminSettings: sección export CSV con `Rss/paisFilter/paises`).
5. **Set<string> en React**: no mutar (`clear/add/toggle` in-place); crear
   `new Set(prev)` (caso Populares).
6. Tras fixes: rebuild de imagen + Ctrl+Shift+R en navegador.

## Parámetros de Búsqueda

### translated_only

Cuando `translated_only=true`, la búsqueda filtra solo noticias que tienen traducción
al idioma destino (normalmente español). El backend hace un `JOIN` con la tabla
`traducciones` verificando que existe una traducción con `status='done'`.

```
GET /api/news?translated_only=true
GET /api/search?q=...&translated_only=true
```

**Nota**: `translated_only` no significa "solo mostrar la traducción", sino
"solo mostrar noticias que tienen traducción". La traducción real se muestra
en los campos `title_translated`, `summary_translated`, `content_translated`.

## Estados de Traducción en UI

### Campos de traducción

| Campo | Descripción |
|-------|-------------|
| `title_translated` | Título traducido al idioma destino |
| `summary_translated` | Resumen traducido |
| `content_translated` | Contenido completo traducido |
| `lang_translated` | Código ISO del idioma al que se tradujo (ej: "es") |

### Comportamiento de Display

La UI usa **fallback automático**: si existe traducción, la muestra; si no, muestra el texto original.

```tsx
// Ejemplo de NewsCard.tsx
{news.title_translated || news.titulo}
{news.summary_translated || news.resumen}
```

### Estados posibles

1. **Sin traducir**: `title_translated` es `null/undefined`
   - Se muestra `titulo` original
   - Badge de idioma original (ej: "EN")

2. **Traducido**: `title_translated` existe
   - Se muestra traducción
   - Badge de idioma traducido (ej: "ES")
   - El campo `lang_translated` indica el idioma destino

3. **Error en traducción**: No hay campo para esto en la UI
   - La traducción simplemente no aparece
   - El item sigue apareciendo con texto original

### Notas de Implementación

- No hay indicador visual de "traducción en progreso"
- No hay retry manual de traducción desde la UI
- La traducción es transparente para el usuario (ve solo el resultado)

## Tasks

- [x] Todos los fixes listados + verificación (`tsc` en Docker, 200s).
- [x] Gap AdminAliases cerrado por backend (2026-09-14): `AdminAliases.tsx`
  funciona contra `GET/PUT/DELETE /admin/aliases/:id` reales.
- [x] `ErrorBoundary` (`components/ErrorBoundary.tsx`) envolviendo `Routes`
  en `App.tsx` (2026-09-14).
