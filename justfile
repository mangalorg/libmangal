install:
	go install ./cmd/lmangal

update:
	go get -u
	go mod tidy -v

generate:
	go generate ./...