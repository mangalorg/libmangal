#!/usr/bin/env just --justfile

go-mod := `go list`

test:
    go test ./...

generate:
	go generate ./...

update:
	go get -u
	go mod tidy -v

publish tag:
    GOPROXY=proxy.golang.org go list -m {{go-mod}}@{{tag}}

# Rename go.mod name
rename new-go-mod:
    find . -type f -not -path './.git/*' -exec sed -i '' -e "s|{{go-mod}}|{{new-go-mod}}|g" {} \;
