CREATE INDEX IF NOT EXISTS idx_media_files_active_episode_folder
ON public.media_files USING btree (episode_id, media_folder_id)
WHERE ((episode_id IS NOT NULL) AND (missing_since IS NULL));
