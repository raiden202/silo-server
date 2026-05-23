SELECT id, scope, library_id, position, section_type, title, featured, item_limit, enabled, config
FROM page_sections
ORDER BY scope, library_id NULLS FIRST, position, id;

SELECT section_type, count(*) AS section_count
FROM page_sections
GROUP BY section_type
ORDER BY section_type;
