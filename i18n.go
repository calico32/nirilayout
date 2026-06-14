package nirilayout

import (
	"embed"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/leonelquinteros/gotext"
)

// Compiled message catalogs, one per language, laid out in the standard
// gettext directory structure: locales/<lang>/LC_MESSAGES/nirilayout.mo.
// English is the source language (the msgids themselves), so it has no
// catalog and is never loaded here.
//
//go:embed locales
var localesFS embed.FS

const i18nDomain = "nirilayout"

var (
	// catalog maps source (English) msgids to their translation in the active
	// language. It is nil when the active language is English or no catalog
	// could be loaded, in which case the source msgids are used verbatim.
	catalog map[string]string
	// lowercaseEnabled forces all translated interface text to lowercase.
	lowercaseEnabled bool
)

// InitI18n selects the interface language and lowercase preference. It must be
// called after flag.Parse. Language selection follows this precedence:
//
//  1. the -lang flag, when set, overrides everything;
//  2. otherwise the operating system locale, read from LC_ALL, then
//     LC_MESSAGES, then LANG;
//  3. otherwise English.
//
// If the selected language has no compiled catalog, the source (English)
// msgids are used as a fallback.
func InitI18n() {
	lang := *langFlag
	lowercase := *lowercaseFlag

	// The flags may not be parsed yet — InitI18n is also called before
	// flag.Parse so that flag.Usage (triggered by -h during parsing) is
	// already localized. In that case, fall back to scanning the raw
	// arguments so that "-lang"/"-lowercase" still take effect.
	if lang == "" || !lowercase {
		argLang, argLower := scanI18nArgs(os.Args[1:])
		if lang == "" {
			lang = argLang
		}
		lowercase = lowercase || argLower
	}

	lowercaseEnabled = lowercase

	if lang == "" {
		lang = detectSystemLang()
	}
	lang = normalizeLang(lang)

	if lang == "en" {
		catalog = nil
		return
	}

	data, err := localesFS.ReadFile(path.Join("locales", lang, "LC_MESSAGES", i18nDomain+".mo"))
	if err != nil {
		catalog = nil
		return
	}

	catalog = parseCatalog(data)
}

// parseCatalog parses a compiled gettext .mo file and returns a map from source
// msgid to its translation. Empty or identity translations are omitted so that
// lookups fall back to the source msgid.
func parseCatalog(data []byte) map[string]string {
	mo := gotext.NewMo()
	mo.Parse(data)

	m := make(map[string]string)
	for id, tr := range mo.GetDomain().GetTranslations() {
		if s := tr.Get(); s != "" && s != id {
			m[id] = s
		}
	}
	return m
}

// lookup returns the translation for msgid, or msgid itself when there is no
// active catalog or no entry for it.
func lookup(msgid string) string {
	if catalog != nil {
		if s, ok := catalog[msgid]; ok {
			return s
		}
	}
	return msgid
}

// scanI18nArgs extracts the -lang value and the -lowercase presence directly
// from the raw command-line arguments. It is used to localize flag.Usage,
// which the flag package may invoke (on -h) before the flags are parsed.
func scanI18nArgs(args []string) (lang string, lowercase bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return lang, lowercase
		case a == "-lang" || a == "--lang":
			if i+1 < len(args) {
				lang = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "-lang="):
			lang = strings.TrimPrefix(a, "-lang=")
		case strings.HasPrefix(a, "--lang="):
			lang = strings.TrimPrefix(a, "--lang=")
		case a == "-lowercase" || a == "--lowercase",
			a == "-lowercase=true" || a == "--lowercase=true":
			lowercase = true
		}
	}
	return lang, lowercase
}

// detectSystemLang reads the OS locale from the standard POSIX environment
// variables, in order of precedence. Returns "" if none are set.
func detectSystemLang() string {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return ""
}

// normalizeLang reduces a raw locale string such as "it_IT.UTF-8@euro" to its
// base language code ("it"). The "C" and "POSIX" locales, as well as an empty
// value, map to English.
func normalizeLang(raw string) string {
	s := raw
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexAny(s, "_-"); i >= 0 {
		s = s[:i]
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "c" || s == "posix" {
		return "en"
	}
	return s
}

// N marks a string for translation extraction without translating it. It is a
// no-op at runtime, returning its argument unchanged. Use it where the string
// must be a plain literal at the point of definition (e.g. flag usage strings
// registered before InitI18n runs) but should still be picked up by xgettext.
// The recorded msgid can then be translated later with T.
func N(s string) string { return s }

// T translates msgid into the active language. When -lowercase is active the
// result is lowercased. Untranslated msgids fall back to English. The msgid may
// be a non-constant string (e.g. a flag usage string looked up at runtime); for
// translating a format string with arguments, use Tf.
func T(msgid string) string {
	s := lookup(msgid)
	if lowercaseEnabled {
		s = strings.ToLower(s)
	}
	return s
}

// Tf translates format into the active language and then formats it with the
// given arguments using fmt.Sprintf semantics. When -lowercase is active the
// final (formatted) string is lowercased. Untranslated formats fall back to
// English.
func Tf(format string, args ...any) string {
	s := fmt.Sprintf(lookup(format), args...)
	if lowercaseEnabled {
		s = strings.ToLower(s)
	}
	return s
}
