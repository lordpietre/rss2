"""Subsistema de sintesis de voz (TTS) de RSS2.

Modulos:

- ``config``    : configuracion por variables de entorno.
- ``schema``    : DDL idempotente de las tablas ``tts_*``.
- ``script``    : normalizacion de guiones para locucion (cifras, fechas, ...).
- ``engine``    : interfaz de motor de voz + implementaciones MeloTTS/OpenVoice.
- ``voices``    : registro de voces y control de calidad de las muestras.
- ``db``        : acceso a Postgres (estado y metadatos).
- ``queue``     : cola asincrona (Redis con respaldo en Postgres).
- ``service``   : API HTTP interna + bucle del worker.

Ningun modulo depende de forma directa de torch o de los pesos del modelo:
importarlos es lazy, de modo que las pruebas unitarias no descargan nada.
"""

__all__ = ["config", "schema", "script", "engine", "voices", "db", "queue", "service"]
