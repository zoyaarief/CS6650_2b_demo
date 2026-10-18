FROM golang:1.25-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY product-service.go ./
RUN go build -o product-service product-service.go

EXPOSE 8080

CMD ["./product-service"]