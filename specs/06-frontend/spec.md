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

## Tasks

- [x] Todos los fixes listados + verificación (`tsc` en Docker, 200s).
- [x] Gap AdminAliases cerrado por backend (2026-09-14): `AdminAliases.tsx`
  funciona contra `GET/PUT/DELETE /admin/aliases/:id` reales.
- [x] `ErrorBoundary` (`components/ErrorBoundary.tsx`) envolviendo `Routes`
  en `App.tsx` (2026-09-14).
