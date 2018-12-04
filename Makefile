install:
	gx install
	gx-go rw
	go install
	gx-go uw

.PHONY: install
