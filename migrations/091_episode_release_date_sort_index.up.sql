CREATE INDEX IF NOT EXISTS idx_episodes_air_date_content
ON public.episodes USING btree (air_date DESC, content_id)
WHERE (air_date IS NOT NULL);
