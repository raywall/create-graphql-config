.PHONY: run tui test fmt example-rest example-dynamodb

GOENV := GOCACHE=$(CURDIR)/.gocache

run:
	$(GOENV) go run . -input pagamentos.json -out generated/rest

tui:
	$(GOENV) go run . -input pagamentos.json -tui

test:
	$(GOENV) go test ./...

fmt:
	$(GOENV) go fmt ./...

example-rest:
	$(GOENV) go run . -input pagamentos.json -out generated/rest -adapter rest -field estoque -type Estoque -key-pattern '/api/estoque/{produto_id}'

example-dynamodb:
	$(GOENV) go run . -input pagamentos.json -out generated/dynamodb -adapter dynamodb -field estoque -type Estoque -key-pattern 'ESTOQUE_{produto_id}' -dynamodb-table graphql-data
