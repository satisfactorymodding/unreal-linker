FROM golang:1.20-alpine

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY . ./

RUN go build -ldflags="-s -w" -v -o linker

EXPOSE 8080

CMD ["/app/linker"]
