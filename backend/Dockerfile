FROM golang:1.24.10-alpine AS builder

WORKDIR /app
ENV CGO_ENABLED=0

COPY . ./
RUN ls -al && \ 
    cd backend && \
     go mod download && \
     go build -o ./bin/server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/bin/server ./server

EXPOSE 8000

CMD ["./server"]
