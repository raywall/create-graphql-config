# create-schema

`create-schema` é uma aplicação de linha de comando em Go para gerar os arquivos `schema.json`, `connectors.json` e `service.json` usados pelo `go-graphql-connector`.

A ideia é partir de um JSON simples de exemplo, como `pagamentos.json`, e transformar o contrato de entrada/saída em uma configuração pronta para uso no conector GraphQL.

## O Que Ela Gera

- `schema.json`: tipos GraphQL inferidos a partir de `response.body`.
- `connectors.json`: configuração do adapter que buscará os dados.
- `service.json`: configuração principal para carregar schema e connectors locais.

## Adapters Suportados

A ferramenta cobre os adapters suportados pelo `go-graphql-connector`:

- `rest`
- `redis`
- `dynamodb`
- `s3`
- `rds`

Para REST, os métodos suportados são:

- `GET`
- `POST`
- `PUT`
- `PATCH`
- `DELETE`
- `HEAD`
- `OPTIONS`

## Formato de Entrada

Exemplo:

```json
{
  "baseurl": "http://localhost:8090",
  "request": {
    "method": "GET",
    "endpoint": "/api/estoque/{produto_id}",
    "headers": [],
    "body": {}
  },
  "response": {
    "status_code": 200,
    "body": {
      "produto_id": "PROD-123",
      "disponivel": true,
      "quantidade_atual": 15
    }
  }
}
```

O `response.body` é usado para inferir tipos GraphQL:

- string -> `String`
- boolean -> `Boolean`
- número inteiro -> `Int`
- número decimal -> `Float`
- objeto -> `Object`
- array -> `List`

Campos aninhados geram tipos GraphQL adicionais automaticamente.

## Uso Rápido

Gerar usando o arquivo `pagamentos.json`:

```bash
go run . -input pagamentos.json -out generated
```

Ou usando o Makefile:

```bash
make run
make example-rest
make example-dynamodb
```

Gerar com interface textual interativa:

```bash
go run . -input pagamentos.json -tui
```

Ou:

```bash
make tui
```

Arquivos criados:

```text
generated/
├── schema.json
├── connectors.json
└── service.json
```

## Exemplos

### REST

```bash
go run . \
  -input pagamentos.json \
  -out generated/rest \
  -adapter rest \
  -field estoque \
  -type Estoque \
  -base-url http://localhost:8090 \
  -endpoint '/api/estoque/{produto_id}' \
  -method GET \
  -key-pattern '/api/estoque/{produto_id}'
```

Isso gera um connector como:

```json
{
  "field": "estoque",
  "adapter": "rest",
  "adapterConfig": {
    "baseUrl": "http://localhost:8090",
    "endpoint": "/api/estoque/{produto_id}",
    "method": "GET",
    "headers": {}
  },
  "keyPattern": "/api/estoque/{produto_id}",
  "timeoutMs": 1000,
  "retries": 1
}
```

### Redis

```bash
go run . \
  -input pagamentos.json \
  -out generated/redis \
  -adapter redis \
  -field estoque \
  -type Estoque \
  -key-pattern 'ESTOQUE_{produto_id}' \
  -redis-endpoint localhost:6379
```

### DynamoDB

```bash
go run . \
  -input pagamentos.json \
  -out generated/dynamodb \
  -adapter dynamodb \
  -field estoque \
  -type Estoque \
  -key-pattern 'ESTOQUE_{produto_id}' \
  -aws-region us-east-1 \
  -dynamodb-table graphql-data
```

No adapter atual do `go-graphql-connector`, a busca no DynamoDB usa a chave `id` com o valor renderizado por `keyPattern`.

### S3

```bash
go run . \
  -input pagamentos.json \
  -out generated/s3 \
  -adapter s3 \
  -field estoque \
  -type Estoque \
  -key-pattern 'estoque/{produto_id}.json' \
  -aws-region us-east-1 \
  -s3-bucket graphql-data
```

### RDS

```bash
go run . \
  -input pagamentos.json \
  -out generated/rds \
  -adapter rds \
  -field estoque \
  -type Estoque \
  -rds-driver postgres \
  -rds-dsn 'postgres://user:pass@localhost:5432/app?sslmode=disable' \
  -rds-query "select produto_id, disponivel, quantidade_atual from estoque where produto_id = '{produto_id}'" \
  -rds-result-mode one
```

Para `resultMode=many`, o adapter retorna:

```json
{
  "items": []
}
```

Nesse caso, ajuste o JSON de exemplo para refletir esse envelope se quiser que o schema seja gerado com `items`.

## Interface TUI

O modo TUI permite revisar e alterar:

- diretório de saída;
- nome do field GraphQL;
- nome do tipo GraphQL;
- adapter;
- `keyPattern`;
- timeout;
- retries;
- optional;
- opções do `service.json`;
- configurações específicas de REST, Redis, DynamoDB, S3 ou RDS.

Execute:

```bash
go run . -tui
```

## Flags Principais

| Flag | Descrição |
|---|---|
| `-input` | Arquivo JSON de entrada. |
| `-out` | Diretório de saída. |
| `-field` | Nome do campo GraphQL e do connector. |
| `-type` | Nome do tipo GraphQL de resposta. |
| `-adapter` | `rest`, `redis`, `dynamodb`, `s3` ou `rds`. |
| `-key-pattern` | Template usado pelo connector para buscar dados. |
| `-timeout-ms` | Timeout por chamada do connector. |
| `-retries` | Quantidade de retentativas. |
| `-optional` | Marca o connector como opcional. |
| `-tui` | Abre a interface textual interativa. |

## Como Usar no go-graphql-connector

Depois de gerar os arquivos, aponte o `service.json` para eles:

```json
{
  "version": "1",
  "schema": "local:schema.json",
  "connectors": "local:connectors.json",
  "route": "/graphql",
  "pretty": true,
  "graphiql": true,
  "allow_partial": false
}
```

Exemplo de query gerada para um endpoint com `{produto_id}`:

```graphql
query {
  estoque(produto_id: "PROD-123") {
    produto_id
    disponivel
    quantidade_atual
  }
}
```

## Observações

- Templates usam `{nome_do_argumento}`.
- Os argumentos GraphQL são inferidos a partir de tokens no `keyPattern`.
- Se o `keyPattern` não tiver tokens, o campo GraphQL é gerado sem argumentos.
- Para DynamoDB, Redis e S3, o `keyPattern` normalmente representa a chave de busca.
- Para REST, o `keyPattern` normalmente é o endpoint.
- Para RDS, o `keyPattern` é a query SQL renderizada.
# create-graphql-config
