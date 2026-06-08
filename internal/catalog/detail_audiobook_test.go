package catalog

import (
	"testing"

	"github.com/Silo-Server/silo-server/internal/models"
)

func TestSortAudiobookMediaFilesPreservesPresentationPartOrder(t *testing.T) {
	files := []*models.MediaFile{
		{ID: 30, FilePath: "/books/book/03.mp3", PresentationPartIndex: 3},
		{ID: 10, FilePath: "/books/book/01.mp3", PresentationPartIndex: 1},
		{ID: 20, FilePath: "/books/book/02.mp3", PresentationPartIndex: 2},
	}

	sortAudiobookMediaFiles(files)

	got := []int{files[0].ID, files[1].ID, files[2].ID}
	want := []int{10, 20, 30}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted IDs = %v, want %v", got, want)
		}
	}
}

func TestSortAudiobookMediaFilesFallsBackToPath(t *testing.T) {
	files := []*models.MediaFile{
		{ID: 20, FilePath: "/books/book/02.mp3"},
		{ID: 10, FilePath: "/books/book/01.mp3"},
	}

	sortAudiobookMediaFiles(files)

	got := []int{files[0].ID, files[1].ID}
	want := []int{10, 20}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted IDs = %v, want %v", got, want)
		}
	}
}

func TestBuildEbookExtensionUsesAuthorsAndPublisherWithoutNarrators(t *testing.T) {
	crew := []CrewCredit{
		{Name: "Becky Chambers", Job: models.PersonKindAuthor.String(), PersonID: "10"},
		{Name: "Narrator Should Not Apply", Job: models.PersonKindNarrator.String(), PersonID: "20"},
	}
	item := &models.MediaItem{
		ContentID: "ebook-1",
		Type:      "ebook",
		Studios:   []string{"Tor"},
	}

	extension := (&DetailService{}).buildEbookExtension(nil, item, crew, AccessFilter{})

	if extension == nil {
		t.Fatal("buildEbookExtension returned nil")
	}
	if len(extension.Authors) != 1 || extension.Authors[0].Name != "Becky Chambers" {
		t.Fatalf("Authors = %+v, want Becky Chambers", extension.Authors)
	}
	if extension.Publisher != "Tor" {
		t.Fatalf("Publisher = %q, want Tor", extension.Publisher)
	}
}
