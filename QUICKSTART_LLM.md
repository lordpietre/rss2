# 🚀 Guía Rápida: Sistema LLM Categorizer

## ✅ Estado Actual

- ✓ Aplicación RSS2 levantada y funcionando correctamente
- ✓ Todos los contenedores están operativos
- ✓ Web accesible en http://localhost:8001
- ✓ Nuevo sistema LLM Categorizer creado y configurado

## 📋 Próximos Pasos

### 1. Descargar el Modelo LLM (REQUERIDO)

```bash
cd /home/x/rss2
./scripts/download_llm_model.sh
```

**Selecciona la opción 1** (Mistral-7B-Instruct-v0.2 GPTQ) - Recomendado para RTX 3060 12GB

⏱️ **Tiempo estimado**: 10-30 minutos según tu conexión  
💾 **Espacio necesario**: ~4.5 GB

### 2. Probar el Sistema (OPCIONAL pero recomendado)

```bash
# Instalar dependencias para prueba local
pip3 install exllamav2 torch

# Ejecutar prueba
python3 scripts/test_llm_categorizer.py
```

Esto te permite verificar que el modelo funciona ANTES de levantar el contenedor.

### 3. Levantar el Servicio LLM

```bash
# Construir y levantar el contenedor
docker compose up -d --build llm-categorizer

# Monitorear los logs
docker compose logs -f llm-categorizer
```

**Primera ejecución**: El contenedor tardará 2-5 minutos en cargar el modelo en GPU.

### 4. Verificar Funcionamiento

```bash
# Ver estado
docker compose ps llm-categorizer

# Ver últimas categorizaciones
docker exec -it rss2_db psql -U rss -d rss -c \
  "SELECT llm_categoria, COUNT(*) FROM noticias WHERE llm_processed = TRUE GROUP BY llm_categoria;"
```

---

## 🔧 Configuración

### Archivos Creados

```
/home/x/rss2/
├── workers/
│   └── llm_categorizer_worker.py      # Worker principal
├── Dockerfile.llm_worker               # Dockerfile específico
├── scripts/
│   ├── download_llm_model.sh          # Descarga del modelo
│   └── test_llm_categorizer.py        # Script de prueba
├── docs/
│   └── LLM_CATEGORIZER.md             # Documentación completa
└── docker-compose.yml                  # Actualizado con servicio llm-categorizer
```

### Servicio en docker-compose.yml

```yaml
llm-categorizer:
  build:
    context: .
    dockerfile: Dockerfile.llm_worker
  environment:
    LLM_BATCH_SIZE: 10          # Noticias por lote
    LLM_SLEEP_IDLE: 30          # Segundos entre lotes
    LLM_MODEL_PATH: /app/models/llm
  deploy:
    resources:
      limits:
        memory: 10G
      reservations:
        devices:
          - driver: nvidia
            count: 1
            capabilities: [ gpu ]
```

---

## 🎯 Cómo Funciona

1. **Recopilación**: El worker consulta la BD y obtiene 10 noticias sin categorizar
2. **Procesamiento**: Envía cada noticia al LLM local (Mistral-7B)
3. **Categorización**: El LLM determina la categoría más apropiada
4. **Actualización**: Guarda la categoría y confianza en la BD
5. **Loop**: Repite el proceso continuamente

### Categorías Disponibles

- Política
- Economía
- Tecnología
- Ciencia
- Salud
- Deportes
- Entretenimiento
- Internacional
- Nacional
- Sociedad
- Cultura
- Medio Ambiente
- Educación
- Seguridad
- Otros

---

## 📊 Rendimiento Esperado

### Con RTX 3060 12GB + Mistral-7B GPTQ

- **VRAM utilizada**: ~6-7 GB
- **Tiempo por noticia**: 2-5 segundos
- **Throughput**: ~120-300 noticias/hora
- **Precisión**: ~85-90% (depende del contenido)

---

## 🔍 Consultas SQL Útiles

### Ver distribución de categorías

```sql
SELECT llm_categoria, COUNT(*) as total, 
       AVG(llm_confianza) as confianza_media
FROM noticias 
WHERE llm_processed = TRUE 
GROUP BY llm_categoria
ORDER BY total DESC;
```

### Ver noticias de una categoría

```sql
SELECT titulo, llm_categoria, llm_confianza, fecha
FROM noticias
WHERE llm_categoria = 'Tecnología'
  AND llm_processed = TRUE
ORDER BY fecha DESC
LIMIT 20;
```

### Resetear procesamiento (para reprocesar)

```sql
-- Resetear últimas 100 noticias
UPDATE noticias 
SET llm_processed = FALSE 
WHERE id IN (
    SELECT id FROM noticias 
    ORDER BY fecha DESC 
    LIMIT 100
);
```

---

## 🐛 Troubleshooting Rápido

### ❌ "Out of memory"
```yaml
# Reducir batch size en docker-compose.yml
LLM_BATCH_SIZE: 5
LLM_CACHE_MODE: Q4
```

### ❌ "Model not found"
```bash
# Verificar descarga
ls -la models/llm/

# Re-descargar si necesario
./scripts/download_llm_model.sh
```

### ❌ No procesa noticias
```bash
# Verificar cuántas faltan
docker exec -it rss2_db psql -U rss -d rss -c \
  "SELECT COUNT(*) FROM noticias WHERE llm_processed = FALSE;"

# Resetear algunas para probar
docker exec -it rss2_db psql -U rss -d rss -c \
  "UPDATE noticias SET llm_processed = FALSE LIMIT 20;"
```

---

## 📚 Documentación Completa

Para más detalles, consulta:

```bash
cat docs/LLM_CATEGORIZER.md
```

O abre: `/home/x/rss2/docs/LLM_CATEGORIZER.md`

---

## 🎓 Comandos Útiles

```bash
# Ver todos los servicios
docker compose ps

# Reiniciar solo el LLM
docker compose restart llm-categorizer

# Ver uso de GPU
nvidia-smi

# Ver logs + seguir
docker compose logs -f llm-categorizer

# Detener el LLM
docker compose stop llm-categorizer

# Eliminar completamente (rebuild desde cero)
docker compose down llm-categorizer
docker compose up -d --build llm-categorizer
```

---

## ⚡ Optimizaciones Futuras

Si quieres mejorar el rendimiento:

1. **Usar EXL2 en lugar de GPTQ** (más rápido en ExLlamaV2)
2. **Aumentar batch size** si sobra VRAM
3. **Fine-tune el modelo** con tus propias categorizaciones
4. **Usar vLLM** para servidor de inferencia más eficiente

---

## 🤝 Soporte

Si encuentras problemas:

1. Revisa logs: `docker compose logs llm-categorizer`
2. Consulta documentación: `docs/LLM_CATEGORIZER.md`
3. Verifica GPU: `nvidia-smi`

---

**¡Listo!** El sistema está completamente configurado. Solo falta descargar el modelo y levantarlo. 🚀
