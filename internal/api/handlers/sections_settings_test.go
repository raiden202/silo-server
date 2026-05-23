package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetSectionsSettingDefaultsToFalse(t *testing.T) {
	h := &SectionSettingsHandler{} // no Settings repo wired
	req := httptest.NewRequest(http.MethodGet, "/api/admin/settings/sections", nil)
	rec := httptest.NewRecorder()

	h.HandleGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"allow_profile_custom_sections":false`) {
		t.Fatalf("default not false: %s", rec.Body.String())
	}
}

func TestGetProfileFlagDefaultsToFalse(t *testing.T) {
	h := &SectionSettingsHandler{} // no Settings repo wired
	req := httptest.NewRequest(http.MethodGet, "/api/profile/sections/flags", nil)
	rec := httptest.NewRecorder()

	h.HandleGetProfileFlag(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"allow_profile_custom_sections":false`) {
		t.Fatalf("default not false: %s", rec.Body.String())
	}
}
