package migrations

import (
	"strings"
	"testing"
)

// normalizeSQL collapses every run of whitespace (including newlines) into a
// single space so substring assertions are insensitive to indentation and
// formatting tweaks in the .sql files.
func normalizeSQL(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func TestTemplateBundleManagementMigrationContract(t *testing.T) {
	upBytes, err := FS.ReadFile("133_library_collection_template_bundle_management.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	downBytes, err := FS.ReadFile("133_library_collection_template_bundle_management.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	up := normalizeSQL(string(upBytes))
	down := normalizeSQL(string(downBytes))

	for _, want := range []string{
		"'template_bundle'",
		"idx_library_collections_managed_management_key",
		"WHERE management_mode <> 'manual' AND management_key <> ''",
	} {
		if !strings.Contains(up, normalizeSQL(want)) {
			t.Fatalf("up migration missing %q", want)
		}
	}
	for _, want := range []string{
		"CHECK (management_mode = ANY (ARRAY['manual', 'section']))",
		"idx_library_collections_section_management_key",
		"WHERE management_mode = 'section' AND management_key <> ''",
	} {
		if !strings.Contains(down, normalizeSQL(want)) {
			t.Fatalf("down migration missing %q", want)
		}
	}
}
