#!/usr/bin/env just --justfile

go-mod := `go list`
flags := '-trimpath -ldflags="-s -w"'

install:
	go install ./cmd/lmangal

update:
	go get -u
	go mod tidy -v

generate:
	go generate ./...

# Rename go.mod name
rename new-go-mod:
    find . -type f -not -path './.git/*' -exec sed -i '' -e "s|{{go-mod}}|{{new-go-mod}}|g" {} \;
