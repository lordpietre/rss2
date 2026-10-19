-- Añadir campos de suscripción y plan a usuarios
-- Para soportar monetización via Google Play Billing

ALTER TABLE users ADD COLUMN IF NOT EXISTS plan VARCHAR(20) DEFAULT 'free';
ALTER TABLE users ADD COLUMN IF NOT EXISTS subscription_status VARCHAR(20) DEFAULT 'none';
ALTER TABLE users ADD COLUMN IF NOT EXISTS subscription_end TIMESTAMP;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_play_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_play_product_id TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS limits_json JSONB DEFAULT '{"feeds": 5, "translations_per_day": 0, "search_enabled": false, "alerts_limit": 0}'::jsonb;

-- Índice para buscar por plan y status
CREATE INDEX IF NOT EXISTS idx_users_plan ON users(plan);
CREATE INDEX IF NOT EXISTS idx_users_subscription_status ON users(subscription_status);

COMMENT ON COLUMN users.plan IS 'Plan del usuario: free, pro, premium';
COMMENT ON COLUMN users.subscription_status IS 'Estado de suscripción: none, active, cancelled, expired';
COMMENT ON COLUMN users.subscription_end IS 'Fecha de fin de suscripción';
COMMENT ON COLUMN users.google_play_token IS 'Token de receipt de Google Play';
COMMENT ON COLUMN users.google_play_product_id IS 'Product ID de Google Play (pro_monthly, premium_monthly)';
COMMENT ON COLUMN users.limits_json IS 'Límites aplicados según el plan en formato JSON';
