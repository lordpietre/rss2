# Sistema de Categorización Automática con LLM

## Descripción

Este sistema utiliza **ExLlamaV2** con un modelo de lenguaje local (LLM) para categorizar automáticamente las noticias del feed RSS. 

### ¿Qué hace?

1. **Recopila 10 noticias** sin categorizar de la base de datos
2. **Envía al LLM local** con un prompt especializado
3. **El LLM discrimina/categoriza** cada noticia en una de las categorías predefinidas
4. **Actualiza la base de datos** con las categorías asignadas

### Ventajas

- ✅ **100% Local**: No envía datos a APIs externas
- ✅ **Optimizado para RTX 3060 12GB**: Modelos cuantizados eficientes
- ✅ **Categorización inteligente**: Entiende contexto, no solo keywords
- ✅ **Escalable**: Procesa lotes de 10 noticias automáticamente
- ✅ **Integrado**: Se ejecuta como un worker más del sistema

---

## Instalación

### Paso 1: Descargar el Modelo

El sistema necesita un modelo LLM compatible. Recomendamos **Mistral-7B-Instruct GPTQ** para RTX 3060 12GB.

```bash
# Ejecutar el script de descarga
./scripts/download_llm_model.sh
```

El script te mostrará opciones:
1. **Mistral-7B-Instruct-v0.2 (GPTQ)** - RECOMENDADO
2. Mistral-7B-Instruct-v0.2 (EXL2)
3. OpenHermes-2.5-Mistral-7B (GPTQ)
4. Neural-Chat-7B (GPTQ)

**Tiempo estimado de descarga**: 10-30 minutos (según conexión)

**Espacio en disco**: ~4.5 GB

### Paso 2: Verificar la instalación

```bash
# Verificar que el modelo se descargó correctamente
ls -lh models/llm/

# Deberías ver archivos como:
# - model.safetensors o *.safetensors
# - config.json
# - tokenizer.json
# - etc.
```

### Paso 3: Probar el sistema (opcional)

Antes de levantar el contenedor, puedes probar que funciona:

```bash
# Instalar dependencias localmente (solo para prueba)
pip3 install exllamav2 torch

# Ejecutar script de prueba
python3 scripts/test_llm_categorizer.py
```

---

## Uso

### Iniciar el servicio

```bash
# Construir y levantar el contenedor
docker compose up -d llm-categorizer

# Ver logs en tiempo real
docker compose logs -f llm-categorizer
```

### Verificar funcionamiento

```bash
# Ver estado del contenedor
docker compose ps llm-categorizer

# Ver últimas 50 líneas de log
docker compose logs --tail=50 llm-categorizer

# Ver categorías asignadas en la base de datos
docker exec -it rss2_db psql -U rss -d rss -c \
  "SELECT llm_categoria, COUNT(*) FROM noticias WHERE llm_processed = TRUE GROUP BY llm_categoria;"
```

### Detener el servicio

```bash
docker compose stop llm-categorizer
```

---

## Configuración

### Variables de Entorno

Puedes ajustar el comportamiento editando `docker-compose.yml`:

```yaml
environment:
  # Número de noticias a procesar por lote (default: 10)
  LLM_BATCH_SIZE: 10
  
  # Tiempo de espera cuando no hay noticias (segundos, default: 30)
  LLM_SLEEP_IDLE: 30
  
  # Longitud máxima de contexto (default: 4096)
  LLM_MAX_SEQ_LEN: 4096
  
  # Modo de caché: FP16 o Q4 (default: FP16)
  # Q4 usa menos VRAM pero puede ser más lento
  LLM_CACHE_MODE: FP16
  
  # Distribución de GPU: "auto" para single GPU
  LLM_GPU_SPLIT: auto
```

### Categorías

Las categorías están definidas en `workers/llm_categorizer_worker.py`:

```python
CATEGORIES = [
    "Política",
    "Economía",
    "Tecnología",
    "Ciencia",
    "Salud",
    "Deportes",
    "Entretenimiento",
    "Internacional",
    "Nacional",
    "Sociedad",
    "Cultura",
    "Medio Ambiente",
    "Educación",
    "Seguridad",
    "Otros"
]
```

Para modificarlas, edita el archivo y reconstruye el contenedor:

```bash
docker compose up -d --build llm-categorizer
```

---

## Base de Datos

### Nuevas columnas en `noticias`

El worker añade automáticamente estas columnas:

- `llm_categoria` (VARCHAR): Categoría asignada
- `llm_confianza` (FLOAT): Nivel de confianza (0.0 - 1.0)
- `llm_processed` (BOOLEAN): Si ya fue procesada
- `llm_processed_at` (TIMESTAMP): Fecha de procesamiento

### Consultas útiles

```sql
-- Ver distribución de categorías
SELECT llm_categoria, COUNT(*) as total, AVG(llm_confianza) as confianza_media
FROM noticias 
WHERE llm_processed = TRUE 
GROUP BY llm_categoria
ORDER BY total DESC;

-- Ver noticias de una categoría específica
SELECT id, titulo, llm_categoria, llm_confianza, fecha
FROM noticias
WHERE llm_categoria = 'Tecnología'
  AND llm_processed = TRUE
ORDER BY fecha DESC
LIMIT 20;

-- Ver noticias con baja confianza (revisar manualmente)
SELECT id, titulo, llm_categoria, llm_confianza
FROM noticias
WHERE llm_processed = TRUE
  AND llm_confianza < 0.6
ORDER BY llm_confianza ASC
LIMIT 20;

-- Resetear procesamiento (para reprocesar)
UPDATE noticias SET llm_processed = FALSE WHERE llm_categoria = 'Otros';
```

---

## Monitorización

### Prometheus/Grafana

El worker está integrado con el stack de monitorización. Puedes ver:

- Uso de GPU (VRAM)
- Tiempo de procesamiento por lote
- Tasa de categorización

Accede a Grafana: http://localhost:3001

### Logs

```bash
# Ver logs en tiempo real
docker compose logs -f llm-categorizer

# Buscar errores
docker compose logs llm-categorizer | grep ERROR

# Ver estadísticas de categorización
docker compose logs llm-categorizer | grep "Distribución"
```

---

## Troubleshooting

### Error: "Out of memory"

**Causa**: El modelo es demasiado grande para tu GPU.

**Solución**:
1. Usa un modelo más pequeño (ej: EXL2 con menor bpw)
2. Reduce el batch size: `LLM_BATCH_SIZE: 5`
3. Usa cache Q4 en lugar de FP16: `LLM_CACHE_MODE: Q4`

```yaml
environment:
  LLM_BATCH_SIZE: 5
  LLM_CACHE_MODE: Q4
```

### Error: "Model not found"

**Causa**: El modelo no se descargó correctamente.

**Solución**:
```bash
# Verificar directorio
ls -la models/llm/

# Debería contener config.json y archivos .safetensors
# Si está vacío, ejecutar de nuevo:
./scripts/download_llm_model.sh
```

### El worker no procesa noticias

**Causa**: Posiblemente ya están todas procesadas.

**Solución**:
```bash
# Verificar cuántas noticias faltan
docker exec -it rss2_db psql -U rss -d rss -c \
  "SELECT COUNT(*) FROM noticias WHERE llm_processed = FALSE;"

# Si es 0, resetear algunas para probar
docker exec -it rss2_db psql -U rss -d rss -c \
  "UPDATE noticias SET llm_processed = FALSE WHERE id IN (SELECT id FROM noticias ORDER BY fecha DESC LIMIT 20);"
```

### Categorización incorrecta

**Causa**: El prompt puede necesitar ajustes o el modelo no es adecuado.

**Soluciones**:
1. Ajustar el prompt en `workers/llm_categorizer_worker.py` (método `_build_prompt`)
2. Probar un modelo diferente (ej: OpenHermes es mejor generalista)
3. Ajustar la temperatura (más baja = más determinista):

```python
self.settings.temperature = 0.05  # Muy determinista
```

---

## Rendimiento

### RTX 3060 12GB

- **Modelo recomendado**: Mistral-7B-Instruct GPTQ 4-bit
- **VRAM utilizada**: ~6-7 GB
- **Tiempo por noticia**: ~2-5 segundos
- **Throughput**: ~120-300 noticias/hora

### Optimizaciones

Para mejorar el rendimiento:

1. **Aumentar batch size** (si sobra VRAM):
   ```yaml
   LLM_BATCH_SIZE: 20
   ```

2. **Cache Q4** (menos VRAM, ligeramente más lento):
   ```yaml
   LLM_CACHE_MODE: Q4
   ```

3. **Modelo EXL2 optimizado**:
   - Usar Mistral EXL2 4.0bpw
   - Es más rápido que GPTQ en ExLlamaV2

---

## Integración con la Web

Para mostrar las categorías en la interfaz web, modifica `routers/search.py` o crea una nueva vista:

```python
# Ejemplo de endpoint para estadísticas
@app.route('/api/categories/stats')
def category_stats():
    query = """
        SELECT llm_categoria, COUNT(*) as total
        FROM noticias
        WHERE llm_processed = TRUE
        GROUP BY llm_categoria
        ORDER BY total DESC
    """
    # ... ejecutar query y devolver JSON
```

---

## Roadmap

Posibles mejoras futuras:

- [ ] Subcategorías automáticas
- [ ] Detección de temas trending
- [ ] Resúmenes automáticos por categoría
- [ ] Alertas personalizadas por categoría
- [ ] API REST para categorización bajo demanda
- [ ] Fine-tuning del modelo con feedback de usuario

---

## Soporte

Para problemas o preguntas:

1. Revisar logs: `docker compose logs llm-categorizer`
2. Verificar GPU: `nvidia-smi`
3. Consultar documentación de ExLlamaV2: https://github.com/turboderp/exllamav2

---

## Licencia

Este componente se distribuye bajo la misma licencia que el proyecto principal RSS2.

Los modelos LLM tienen sus propias licencias (generalmente Apache 2.0 o MIT para los recomendados).
