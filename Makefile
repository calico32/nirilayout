MAIN := ./cmd/nirilayout
SRCS := $(wildcard *.go cmd/nirilayout/*.go) go.mod go.sum style.css

# Localization (gettext). Source files to scan for translatable T("...") calls.
I18N_SRCS := $(wildcard *.go cmd/nirilayout/*.go)
POT := locales/nirilayout.pot
POFILES := $(wildcard locales/*/LC_MESSAGES/*.po)
MOFILES := $(POFILES:.po=.mo)

nirilayout: $(SRCS) $(MOFILES)
	go build -o $@ $(MAIN)

nirilayout-profile: $(SRCS) $(MOFILES)
	go build -o $@ -tags profile $(MAIN)

install: $(SRCS) $(MOFILES)
	go install $(MAIN)

# Regenerate the .pot template from the Go sources.
pot: $(POT)
$(POT): $(I18N_SRCS)
	xgettext --language=C --from-code=UTF-8 --keyword=T:1 --keyword=Tf:1 --keyword=N:1 \
		--package-name=nirilayout \
		--msgid-bugs-address="https://github.com/calico32/nirilayout/issues" \
		--copyright-holder="nirilayout contributors" \
		-o $@ $(I18N_SRCS)

# Merge the latest .pot into every existing .po, preserving translations.
update-po: $(POT)
	@for po in $(POFILES); do \
		echo "msgmerge $$po"; \
		msgmerge --update --backup=none $$po $(POT); \
	done

# Compile .po files to .mo. Build targets depend on these.
%.mo: %.po
	msgfmt --check -o $@ $<

# Regenerate translations: refresh .po files from sources, then recompile .mo.
i18n: update-po $(MOFILES)

clean:
	rm -f nirilayout nirilayout-profile

.PHONY: clean pot update-po i18n
