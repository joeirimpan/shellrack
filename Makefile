build:
	@go build -o shellrack
.PHONY: build

backup: build
	@./shellrack --backup
.PHONY: backup

restore: build
	@./shellrack --restore
.PHONY: restore