package scanner

import "testing"

func TestNormalizeChapters(t *testing.T) {
	t.Run("uses embedded titles and keeps valid ranges", func(t *testing.T) {
		chapters := normalizeChapters([]ffprobeChapter{
			{
				StartTime: "0.0",
				EndTime:   "60.0",
				Tags:      map[string]string{"title": "Cold Open"},
			},
			{
				StartTime: "60.0",
				EndTime:   "120.0",
				Tags:      map[string]string{"TITLE": "Main Story"},
			},
		}, 180)

		if len(chapters) != 2 {
			t.Fatalf("len(chapters) = %d, want 2", len(chapters))
		}
		if chapters[0].Title != "Cold Open" || chapters[0].StartSeconds != 0 || chapters[0].EndSeconds != 60 {
			t.Fatalf("chapter[0] = %#v", chapters[0])
		}
		if chapters[1].Title != "Main Story" || chapters[1].StartSeconds != 60 || chapters[1].EndSeconds != 120 {
			t.Fatalf("chapter[1] = %#v", chapters[1])
		}
	})

	t.Run("fills in fallback titles after filtering", func(t *testing.T) {
		chapters := normalizeChapters([]ffprobeChapter{
			{StartTime: "0", EndTime: "10"},
			{StartTime: "10", EndTime: "20", Tags: map[string]string{"title": "Named"}},
			{StartTime: "20", EndTime: "20"},
			{StartTime: "20", EndTime: "25"},
		}, 25)

		if len(chapters) != 3 {
			t.Fatalf("len(chapters) = %d, want 3", len(chapters))
		}
		if chapters[0].Title != "Chapter 01" {
			t.Fatalf("chapter[0].Title = %q, want Chapter 01", chapters[0].Title)
		}
		if chapters[1].Title != "Named" {
			t.Fatalf("chapter[1].Title = %q, want Named", chapters[1].Title)
		}
		if chapters[2].Title != "Chapter 03" {
			t.Fatalf("chapter[2].Title = %q, want Chapter 03", chapters[2].Title)
		}
	})

	t.Run("sorts out of order chapters and trims overlaps", func(t *testing.T) {
		chapters := normalizeChapters([]ffprobeChapter{
			{StartTime: "90", EndTime: "140", Tags: map[string]string{"title": "Third"}},
			{StartTime: "0", EndTime: "70", Tags: map[string]string{"title": "First"}},
			{StartTime: "60", EndTime: "100", Tags: map[string]string{"title": "Second"}},
		}, 200)

		if len(chapters) != 3 {
			t.Fatalf("len(chapters) = %d, want 3", len(chapters))
		}
		if chapters[0].Title != "First" || chapters[0].StartSeconds != 0 || chapters[0].EndSeconds != 60 {
			t.Fatalf("chapter[0] = %#v", chapters[0])
		}
		if chapters[1].Title != "Second" || chapters[1].StartSeconds != 60 || chapters[1].EndSeconds != 90 {
			t.Fatalf("chapter[1] = %#v", chapters[1])
		}
		if chapters[2].Title != "Third" || chapters[2].StartSeconds != 90 || chapters[2].EndSeconds != 140 {
			t.Fatalf("chapter[2] = %#v", chapters[2])
		}
	})

	t.Run("clamps chapters to duration and drops zero length results", func(t *testing.T) {
		chapters := normalizeChapters([]ffprobeChapter{
			{StartTime: "-5", EndTime: "5"},
			{StartTime: "25", EndTime: "40"},
			{StartTime: "30", EndTime: "35"},
		}, 30)

		if len(chapters) != 2 {
			t.Fatalf("len(chapters) = %d, want 2", len(chapters))
		}
		if chapters[0].StartSeconds != 0 || chapters[0].EndSeconds != 5 {
			t.Fatalf("chapter[0] = %#v", chapters[0])
		}
		if chapters[1].StartSeconds != 25 || chapters[1].EndSeconds != 30 {
			t.Fatalf("chapter[1] = %#v", chapters[1])
		}
	})
}
