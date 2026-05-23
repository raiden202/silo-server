CREATE INDEX IF NOT EXISTS idx_operational_logs_component_level
    ON public.operational_logs (component, level, "timestamp" DESC);
