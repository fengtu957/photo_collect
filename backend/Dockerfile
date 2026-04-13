FROM registry.cn-chengdu.aliyuncs.com/fengtu/go:1.24.10-alpine AS builder

WORKDIR /app
ENV CGO_ENABLED=0

COPY . ./
RUN ls -al && \ 
    mkdir ./bin && \
    go env -w GOPROXY=https://goproxy.cn,direct && \
    go mod download && \
    go build -o /server ./cmd/server

FROM registry.cn-chengdu.aliyuncs.com/fengtu/alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /server ./server

EXPOSE 8000

CMD ["./server"]
