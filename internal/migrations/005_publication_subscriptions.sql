ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS plan_duration_months SMALLINT,
    ADD COLUMN IF NOT EXISTS subscription_expires_at TIMESTAMPTZ;

UPDATE orders
SET plan_duration_months = CASE WHEN status = 'completed' THEN 12 ELSE 1 END
WHERE order_type = 'publication_plan' AND plan_duration_months IS NULL;

UPDATE orders
SET subscription_expires_at = updated_at + INTERVAL '12 months'
WHERE order_type = 'publication_plan'
  AND status = 'completed'
  AND subscription_expires_at IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'orders_plan_duration_months_check'
          AND conrelid = 'orders'::regclass
    ) THEN
        ALTER TABLE orders
            ADD CONSTRAINT orders_plan_duration_months_check
                CHECK (plan_duration_months IS NULL OR plan_duration_months IN (1, 12));
    END IF;
END;
$$;
