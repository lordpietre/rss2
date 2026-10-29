-- News dated in the future: feeds whose clock is off.
--
-- Some feeds publish their LOCAL time but label it as UTC: CNN Turk says
-- '12:06 GMT' when it is really 14:06 local (UTC+3), and the Philippine
-- Inquirer says '+0000' when it is really 17:20 local (UTC+8). The result
-- is news dated 3 or 8 hours in the future, which /api/news (ORDER BY fecha
-- DESC) puts at the head of the home page for hours on end.
--
-- The ingestor already clamps them (sanitizeDate, 2 hour margin). This
-- migration fixes the ones that were already ingested.
--
-- Only recent news is touched: for old news, created_at is the date it was
-- ingested (today) and not the date it was published, so it cannot be used
-- as an upper bound.

ALTER TABLE noticias DISABLE TRIGGER update_noticias_updated_at;

UPDATE noticias
   SET fecha = created_at
 WHERE fecha > created_at + INTERVAL '2 hours'
   AND created_at > NOW() - INTERVAL '30 days';

-- The news that are only slightly ahead (a few minutes, because the
-- publisher's clock runs fast) keep their date: they are not an error.
--
-- With this there is no news in the database with a date in the future.

ALTER TABLE noticias ENABLE TRIGGER update_noticias_updated_at;
