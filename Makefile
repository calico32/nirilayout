nirilayout: $(wildcard *.go) go.mod go.sum style.css
	go build -o $@

nirilayout-profile: $(wildcard *.go) go.mod go.sum style.css
	go build -o $@ -tags profile

clean:
	rm -f nirilayout nirilayout-profile

.PHONY: clean
