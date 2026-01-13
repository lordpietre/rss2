# 📖 PROCESO COMPLETO: Descubrimiento y Gestión de Feeds RSS

## 🎯 Problema Resuelto

**Pregunta:** ¿Cómo asigno país y categoría a los feeds descubiertos automáticamente?

**Respuesta:** El sistema ahora usa un flujo inteligente de 3 niveles:

1. **Auto-aprobación** (feeds con categoría/país)
2. **Revisión manual** (feeds sin metadata completa)
3. **Análisis automático** (sugerencias inteligentes)

---

## 🔄 FLUJO COMPLETO DEL SISTEMA

### Paso 1: Añadir URL Fuente

Tienes 2 opciones para añadir URLs:

#### Opción A: Con Categoría y País (AUTO-APROBACIÓN)
```sql
INSERT INTO fuentes_url (nombre, url, categoria_id, pais_id, idioma, active)
VALUES ('El País', 'https://elpais.com', 1, 44, 'es', TRUE);
--                                        ^     ^
--                             categoria_id    pais_id
```

✅ **Resultado**: Feeds se crean **AUTOMÁTICAMENTE** y se activan
- Worker descubre feeds
- Hereda categoría (1) y país (44) del padre
- Crea feeds en tabla `feeds` directam ente
- Ingestor empieza a descargar noticias

#### Opción B: Sin Categoría o País (REQUIERE REVISIÓN)
```sql
INSERT INTO fuentes_url (nombre, url, active)
VALUES ('BBC News', 'https://www.bbc.com/news', TRUE);
--      Sin categoria_id ni pais_id
```

⚠️ **Resultado**: Feeds van a **REVISIÓN MANUAL**
- Worker descubre feeds
- Analiza automáticamente:
  - Detecta país desde dominio (.com → Reino Unido)
  - Detecta idioma (en)
  - Sugiere categoría ("Internacional")
- Crea feeds en tabla `feeds_pending`
- **ESPERA APROBACIÓN MANUAL** antes de activar

---

### Paso 2: Worker Descubre Feeds (cada 15 min)

El worker `url_discovery_worker` ejecuta automaticamente:

```
1. Lee fuentes_url activas
2. Para cada URL:
   a. Descubre todos los feeds RSS
   b. Valida cada feed
   c. Analiza metadata:
      - Idioma del feed
      - País (desde dominio: .es, .uk, .fr, etc.)
      - Categoría sugerida (keywords en título/descripción)
   
   d. DECIDE EL FLUJO:
      
      ┌─────────────────────────────────────┐
      │ ¿Parent tiene categoria_id Y pais_id? │
      └──────────┬──────────────────────────┘
                 │
        ┌────────┴────────┐
        │ SÍ              │ NO
        ▼                 ▼
  ┌──────────────┐   ┌─────────────────┐
  │ AUTO-APROBAR │   │ REQUIERE REVISIÓN│
  └───────┬──────┘   └─────────┬───────┘
          │                    │
          ▼                    ▼
    tabla: feeds        tabla: feeds_pending
    activo: TRUE        reviewed: FALSE
    ✅ Listo para       ⏳ Espera aprobación
       ingestor
```

---

### Paso 3A: Feeds AUTO-APROBADOS

Si la URL padre tiene `categoria_id` y `pais_id`:

```sql
-- Ejemplo: URL con metadata completa
fuentes_url: 
  id=1, url='https://elpais.com', 
  categoria_id=1 (Noticias), 
  pais_id=44 (España)

↓ Worker descubre 3 feeds:
  - https://elpais.com/rss/portada.xml
  - https://elpais.com/rss/internacional.xml
  - https://elpais.com/rss/deportes.xml

↓ Se crean DIRECTAMENTE en tabla feeds:
INSERT INTO feeds (nombre, url, categoria_id, pais_id, activo)
VALUES 
  ('El País - Portada', 'https://elpais.com/rss/portada.xml', 1, 44, TRUE),
  ('El País - Internacional', 'https://elpais.com/rss/internacional.xml', 1, 44, TRUE),
  ('El País - Deportes', 'https://elpais.com/rss/deportes.xml', 1, 44, TRUE);

✅ Feeds están ACTIVOS inmediatamente
✅ Ingestor Go los procesa en siguiente ciclo (15 min)
✅ Noticias empiezan a llegar
```

---

### Paso 3B: Feeds PENDIENTES (requieren revisión)

Si la URL padre NO tiene `categoria_id` o `pais_id`:

```sql
-- Ejemplo: URL sin metadata
fuentes_url: 
  id=2, url='https://www.bbc.com/news', 
  categoria_id=NULL, 
  pais_id=NULL

↓ Worker descubre 2 feeds y ANALIZA automáticamente:
  
  Feed 1: https://www.bbc.com/news/world/rss.xml
  - Título: "BBC News - World"
  - Idioma detectado: 'en'
  - País detectado: 'Reino Unido' (desde .com + idioma inglés)
  - Categoría sugerida: 'Internacional' (keyword "world")

  Feed 2: https://www.bbc.com/sport/rss.xml
  - Título: "BBC Sport"
  - Idioma detectado: 'en'
  - País detectado: 'Reino Unido'
  - Categoría sugerida: 'Deportes' (keyword "sport")

↓ Se crean en tabla feeds_pending:
INSERT INTO feeds_pending (
    fuente_url_id, feed_url, feed_title, 
    feed_language, detected_country_id, suggested_categoria_id,
    reviewed, approved, notes
) VALUES (
    2, 
    'https://www.bbc.com/news/world/rss.xml',
    'BBC News - World',
    'en',
    74,  -- Reino Unido (ID detectado)
    2,   -- Internacional (ID sugerido)
    FALSE, FALSE,
    'Country from domain: Reino Un ido | Suggested category: Internacional (confidence: 85%)'
);

⏳ Feeds están PENDIENTES
⏳ NO están activos aún
⏳ Requieren revisión manual en /feeds/pending
```

---

## 📊 Tabla Comparativa

| Aspecto | Auto-Aprobación | Revisión Manual |
|---------|----------------|-----------------|
| **Requisito** | URL padre con `categoria_id` Y `pais_id` | URL padre sin uno o ambos |
| **Tabla destino** | `feeds` (directa) | `feeds_pending` (temporal) |
| **Estado inicial** | `activo = TRUE` | `reviewed = FALSE, approved = FALSE` |
| **Análisis automático** | Hereda valores del padre | Detecta país, sugiere categoría |
| **Intervención manual** | ❌ No necesaria | ✅ Requerida |
| **Tiempo hasta activación** | Inmediato | Después de aprobación |
| **Ingestor procesa** | Sí (próximo ciclo) | No (hasta aprobar) |

---

## 🛠️ Interfaces de Gestión

### 1. Añadir URL con Metadata (Auto-aprobación)

**Ruta:** `/urls/add_source`

```
Formulario:
┌─────────────────────────────────────┐
│ Nombre:     El País                 │
│ URL:        https://elpais.com      │
│ Categoría:  [Noticias ▼]  ← IMPORTANTE
│ País:       [España ▼]    ← IMPORTANTE
│ Idioma:     es                      │
│                                     │
│ [Añadir Fuente]                     │
└─────────────────────────────────────┘

Resultado: Feeds se crearán AUTOMÁTICAMENTE
```

### 2. Revisar Feeds Pendientes (Nueva interfaz)

**Ruta:** `/feeds/pending` (próximamente)

```
Feeds Pendientes de Revisión
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Feed: BBC News - World
URL: https://www.bbc.com/news/world/rss.xml
Fuente: BBC News (https://www.bbc.com/news)

Análisis Automático:
├─ Idioma: English (en)
├─ País detectado: Reino Unido (.com domain + language)
└─ Categoría sugerida: Internacional (85% confianza)
   Keywords: "world", "international", "global"

┌─────────────────────────────────────┐
│ Categoría:  [Internacional ▼]      │  ← Pre-seleccionada
│ País:       [Reino Unido ▼]        │  ← Pre-seleccionado
│ Idioma:     [en]                    │  ← Auto-detectado
│                                     │
│ [✓ Aprobar Feed] [✗ Rechazar]      │
└─────────────────────────────────────┘
```

### 3. Descubrir Feeds Manualmente

**Ruta:** `/feeds/discover`

```
Perfecto para cuando quieres control total:
1. Ingresar URL
2. Ver todos los feeds encontrados
3. Seleccionar cuáles añadir
4. Asignar categoría/país globalmente
5. Feeds se crean directamente (no van a pending)
```

---

## 💡 RECOMENDACIONES DE USO

### Estrategia 1: Auto-aprobación Total
**Para fuentes conocidas y confiables:**

```sql
-- Añadir fuentes con metadata completa
INSERT INTO fuentes_url (nombre, url, categoria_id, pais_id, idioma) VALUES
('El País', 'https://elpais.com', 1, 44, 'es'),
('Le Monde', 'https://lemonde.fr', 1, 60, 'fr'),
('The Guardian', 'https://theguardian.com', 1, 74, 'en');

-- Worker creará feeds automáticamente
-- Sin intervención manual necesaria
```

### Estrategia 2: Revisión Manual
**Para fuentes nuevas o desconocidas:**

```sql
-- Añadir sin metadata
INSERT INTO fuentes_url (nombre, url) VALUES
('Sitio Desconocido', 'https://ejemplo.com');

-- Worker crea feeds en feeds_pending
-- Revisar en /feeds/pending
-- Aprobar/rechazar manualmente
```

### Estrategia 3: Híbrida (Recomendada)
**Combinar ambas:**

- URLs conocidas → Con categoría/país
- URLs nuevas → Sin metadata (revisión)
- Usar análisis automático como guía
- Ajustar manualmente si es necesario

---

## 🔍 Análisis Automático Explicado

### Detección de País

```python
# 1. Desde dominio (TLD)
.es → España
.uk, .co.uk → Reino Unido
.fr → Francia
.de → Alemania
.mx → México
.ar → Argentina

# 2. Desde idioma (si no hay dominio claro)
es → España (país principal)
en → Reino Unido
fr → Francia
pt → Portugal

# 3. Desde subdominios
es.example.com → España
uk.example.com → Reino Unido
```

### Sugerencia de Categoría

```python
# Análisis de keywords en título + descripción

Keywords encontrados → Categoría sugerida (% confianza)

"política", "gobierno", "elecciones" → Política (75%)
"economía", "bolsa", "mercado" → Economía (82%)
"tecnología", "software", "digital" → Tecnología (90%)
"deportes", "fútbol", "liga" → Deportes (95%)
"internacional", "mundo", "global" → Internacional (70%)
```

---

## 📝 Ejemplos Completos

### Ejemplo 1: Periódico Español (Auto-aprobación)

```sql
-- 1. Añadir fuente con metadata
INSERT INTO fuentes_url (nombre, url, categoria_id, pais_id, idioma)
VALUES ('El Mundo', 'https://elmundo.es', 1, 44, 'es');

-- 2. Worker ejecuta (15 min después):
--    - Descubre: elmundo.es/rss/portada.xml
--    - Descubre: elmundo.es/rss/deportes.xml
--    - Hereda: categoria_id=1, pais_id=44
--    - Crea en feeds directamente

-- 3. Resultado en tabla feeds:
SELECT id, nombre, url, categoria_id, pais_id, activo
FROM feeds
WHERE fuente_nombre LIKE '%El Mundo%';

-- id | nombre              | url                          | cat | pais | activo
-- 1  | El Mundo - Portada  | elmundo.es/rss/portada.xml  | 1   | 44   | TRUE
-- 2  | El Mundo - Deportes | elmundo.es/rss/deportes.xml | 1   | 44   | TRUE

-- ✅ Feeds activos, ingestor procesando
```

### Ejemplo 2: Sitio Internacional (Revisión Manual)

```sql
-- 1. Añadir fuente SIN metadata
INSERT INTO fuentes_url (nombre, url)
VALUES ('Reuters', 'https://www.reuters.com');

-- 2. Worker ejecuta (15 min después):
--    - Descubre: reuters.com/rssfeed/worldNews
--    - Analiza: idioma=en, país=Reino Unido (dominio+idioma)
--    - Sugiere: categoría=Internacional (keyword "world")
--    - Crea en feeds_pending

-- 3. Resultado en tabla feeds_pending:
SELECT feed_title, detected_country_id, suggested_categoria_id, notes
FROM feeds_pending
WHERE fuente_url_id = 3;

-- feed_title         | detected_country_id | suggested_cat | notes
-- Reuters World News | 74 (Reino Unido)    | 2 (Int.)      | "Country from domain..."

-- ⏳ Requiere aprobación en /feeds/pending
```

---

## ✅ CHECKLIST: Añadir Nueva Fuente

**Para auto-aprobación (recomendado si sabes país/categoría):**

- [ ] Ir a `/urls/add_source`
- [ ] Ingresar nombre descriptivo
- [ ] Ingresar URL del sitio (NO del feed RSS)
- [ ] **IMPORTANTE:** Seleccionar categoría
- [ ] **IMPORTANTE:** Seleccionar país
- [ ] Ingresar idioma (opcional, se detecta)
- [ ] Guardar
- [ ] Esperar 15 minutos (máximo)
- [ ] Ver feeds en `/feeds/` (activos automáticamente)

**Para revisión manual (si no estás seguro):**

- [ ] Ir a `/urls/add_source`
- [ ] Ingresar nombre y URL
- [ ] Dejar categoría/país vacíos
- [ ] Guardar
- [ ] Esperar 15 minutos
- [ ] Ir a `/feeds/pending`
- [ ] Revisar sugerencias automáticas
- [ ] Ajustar categoría/país si necesario
- [ ] Aprobar feeds
- [ ] Feeds se activan inmediatamente

---

## 🎓 Resumen Ejecutivo

**3 Niveles de Automatización:**

| Nivel | Descripción | Cuándo Usar |
|-------|-------------|-------------|
| **Nivel 1: Totalmente Manual** | Descubrir en `/feeds/discover` | Control total, pocas URLs |
| **Nivel 2: Auto-aprobación** | URL con cat/país → feeds activos | URLs confiables, muchas fuentes |
| **Nivel 3: Revisión Asistida** | URL sin cat/país → análisis → aprobar | URLs nuevas, verificación |

**Flujo Recomendado:**
1. Añade URL con categoría/país si la conoces
2. Si no, déjalo vacío y revisa sugerencias automáticas
3. Worker descubre y analiza todo automáticamente
4. Tú solo apruebas/ajustas lo necesario

**Resultado:** Gestión eficiente de cientos de fuentes RSS con mínima intervención manual.

---

**📅 Fecha de última actualización:** 2026-01-07
**📌 Versión del sistema:** 2.0 - Análisis Inteligente de Feeds
