# Exemplos de APIs

Arquivos JSON derivados de `/Users/raysouz/Workspace/estudos/workflows/SAMPLE.md`.

Os arquivos de API seguem o mesmo formato de `pagamentos.json` e podem ser usados como entrada no `create-graphql-config`.

## Arquivos

- `estoque-consulta.json`: `GET /api/estoque/{produto_id}`
- `pagamentos-geracao-cobranca.json`: `POST /api/pagamentos`
- `pedidos-criacao.json`: `POST /api/pedidos`
- `estoque-baixa-produto.json`: `PUT /api/estoque/baixar`
- `expedicao-ordem-envio.json`: `POST /api/expedicao`
- `evento-pagamento-aprovado.json`: evento assíncrono publicado em `pagamentos.eventos`

## Exemplos de uso

```bash
go run . -input examples/apis/estoque-consulta.json -out generated/examples/estoque-consulta -field estoque -type Estoque -key-pattern '/api/estoque/{produto_id}'
```

```bash
go run . -input examples/apis/pagamentos-geracao-cobranca.json -out generated/examples/pagamentos -field pagamento -type Pagamento -key-pattern '/api/pagamentos'
```

```bash
go run . -input examples/apis/expedicao-ordem-envio.json -out generated/examples/expedicao -field expedicao -type Expedicao -key-pattern '/api/expedicao'
```

O arquivo `evento-pagamento-aprovado.json` não segue o contrato request/response, pois representa uma mensagem assíncrona. Ele serve como massa de teste para fluxo/event sourcing ou para adaptar uma configuração baseada em fila/tópico.
