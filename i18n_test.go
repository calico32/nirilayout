package nirilayout

import "testing"

func TestNormalizeLang(t *testing.T) {
	cases := map[string]string{
		"it":             "it",
		"it_IT":          "it",
		"it_IT.UTF-8":    "it",
		"it_IT.UTF-8@eu": "it",
		"en_US.UTF-8":    "en",
		"pt-BR":          "pt",
		"C":              "en",
		"POSIX":          "en",
		"":               "en",
		"  IT  ":         "it",
	}
	for in, want := range cases {
		if got := normalizeLang(in); got != want {
			t.Errorf("normalizeLang(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTFallbackToMsgid(t *testing.T) {
	defer restoreI18n(catalog, lowercaseEnabled)
	catalog = nil
	lowercaseEnabled = false

	if got := T("Esc to quit"); got != "Esc to quit" {
		t.Errorf("T fallback = %q, want %q", got, "Esc to quit")
	}
	if got := Tf("Error loading layouts: %v", "boom"); got != "Error loading layouts: boom" {
		t.Errorf("Tf fallback with args = %q, want %q", got, "Error loading layouts: boom")
	}
}

func TestTLowercase(t *testing.T) {
	defer restoreI18n(catalog, lowercaseEnabled)
	catalog = nil
	lowercaseEnabled = true

	if got := T("Name or shortcut…"); got != "name or shortcut…" {
		t.Errorf("T lowercase = %q, want %q", got, "name or shortcut…")
	}
	if got := Tf("Error loading layouts: %v", "BOOM"); got != "error loading layouts: boom" {
		t.Errorf("Tf lowercase with args = %q, want %q", got, "error loading layouts: boom")
	}
}

func TestTItalianCatalog(t *testing.T) {
	defer restoreI18n(catalog, lowercaseEnabled)
	lowercaseEnabled = false

	data, err := localesFS.ReadFile("locales/it/LC_MESSAGES/nirilayout.mo")
	if err != nil {
		t.Fatalf("embedded Italian catalog missing: %v", err)
	}
	catalog = parseCatalog(data)

	if got := T("Esc to quit"); got != "Esc per uscire" {
		t.Errorf("T(it) = %q, want %q", got, "Esc per uscire")
	}
	// An unknown msgid must fall back to the source string. Use a variable so
	// xgettext does not extract this test-only string into the catalog.
	unknown := "Totally untranslated string"
	if got := T(unknown); got != unknown {
		t.Errorf("T(it) unknown = %q, want fallback to source", got)
	}
}

func restoreI18n(c map[string]string, lower bool) {
	catalog = c
	lowercaseEnabled = lower
}
