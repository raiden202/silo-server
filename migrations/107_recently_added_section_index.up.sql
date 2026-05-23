CREATE INDEX IF NOT EXISTS idx_item_libraries_folder_seen_content
ON public.media_item_libraries USING btree (media_folder_id, first_seen_at DESC, content_id ASC);
