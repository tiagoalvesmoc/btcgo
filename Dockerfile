# Etapa 1: Construir o binário
FROM golang:1.20 AS build

# Defina o diretório de trabalho dentro do contêiner
WORKDIR /app

# Copie o go.mod e o go.sum e baixe as dependências
COPY go.mod go.sum ./
RUN go mod download

# Copie o restante dos arquivos da aplicação
COPY . .

# Compile a aplicação
RUN go build -o /app/btc-go-up

# Etapa 2: Criar a imagem final
FROM debian:bullseye-slim

# Defina o diretório de trabalho dentro do contêiner
WORKDIR /app

# Copie o binário da etapa de construção
COPY --from=build /app/btc-go /app/btc-go

# Copie quaisquer outros arquivos necessários, como o ranges.json e wallets.json
COPY --from=build /app/data /app/data

# Defina a porta que será exposta (se necessário)
EXPOSE 8080

# Comando para executar a aplicação
CMD ["./btc-go"]
