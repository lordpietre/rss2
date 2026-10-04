# Cache Redis - RSS2

## Configuración

Redis se usa para caching de datos semi-estáticos y reconstruibles.

## TTL Constants

| Constante | Valor | Uso |
|-----------|-------|-----|
| `TTLShort` | 5 min | Datos que cambian frecuentemente (búsquedas) |
| `TTLMedium` | 30 min | Datos semi-estáticos (stats, feeds) |
| `TTLLong` | 2 horas | Datos estables |
| `TTLNoExpire` | 0 | Sin expiración (invalidación manual) |

## Key Prefixes

| Prefix | Entidad |
|--------|---------|
| `search:*` | Resultados de búsqueda |
| `news:*` | Detalle de noticias |
| `feed:*` | Feed individual |
| `feedlist:*` | Lista de feeds |
| `categories` | Categorías (sin sufijo) |
| `countries` | Países (sin sufijo) |
| `stats` | Estadísticas generales |
| `statstop:*` | Top categories/countries |
| `entities:*` | Entidades |

## Formato de Keys

```
search:{query}:{lang}:{page}:{perPage}
news:{id}:{lang}
feed:{id}
feedlist:{filters}:{page}:{perPage}
categories                    # string, no suffix
countries                     # string, no suffix
stats                         # string, no suffix
statstop:{type}              # type = categories | countries
entities:{type}:{query}:{page}:{perPage}
```

## Invalidation

El cache se invalida cuando los datos subyacentes cambian:

| Operación | Keys invalidadas |
|-----------|-----------------|
| `InvalidateSearch()` | `search:*` |
| `InvalidateNews(id)` | `news:{id}:*` |
| `InvalidateFeed(id)` | `feed:{id}` |
| `InvalidateFeedList()` | `feedlist:*` |
| `InvalidateCategories()` | `categories` |
| `InvalidateCountries()` | `countries` |
| `InvalidateStats()` | `stats`, `statstop:categories`, `statstop:countries` |
| `InvalidateEntities(type)` | `entities:{type}:*` |

## Notas de Diseño

1. **Cache reconstruible**: Todos los datos cacheados se pueden reconstruir desde la BD.
2. **Serialización JSON**: `cache.Set` serializa structs a JSON.
3. **Best-effort**: Si Redis falla, el sistema sigue funcionando (cache opcional).
4. **Translation cache**: El translator worker usa cache Redis con formato `tr:{lang_from}:{lang_to}:{md5}`, TTL 30 días (ver spec 03).
