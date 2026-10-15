# go-lab

Laboratório pessoal de estudos em Go: pequenos exercícios com testes, um por
Pull Request.

## Como funciona

- Cada exercício vive em `exercises/NNN-nome/` como um pacote independente.
- Todo exercício vem com testes (`*_test.go`) em formato table-driven.
- O fluxo é sempre o mesmo: branch → Pull Request → merge.

## Rodando

```sh
go test ./...        # todos os exercícios
go vet ./...         # análise estática
go test ./exercises/001-reverse-string/   # um exercício
```

## Requisitos

- Go 1.24 ou superior

## Licença

[MIT](LICENSE)
