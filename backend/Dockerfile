FROM registry.cn-chengdu.aliyuncs.com/fengtu/go:1.24.10-alpine AS builder

WORKDIR /app
ENV CGO_ENABLED=0

COPY . ./
RUN ls -al && \
    mkdir -p /out && \
    go env -w GOPROXY=https://goproxy.cn,direct && \
    go mod download && \
    go build -o /out/server ./cmd/server && \
    go build -o /out/vip-admin ./cmd/vip-admin && \
    go build -o /out/migrate-qiniu-to-aliyun ./cmd/migrate-qiniu-to-aliyun

FROM registry.cn-chengdu.aliyuncs.com/fengtu/alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /out/server ./server
COPY --from=builder /out/vip-admin ./vip-admin
COPY --from=builder /out/migrate-qiniu-to-aliyun ./migrate-qiniu-to-aliyun

EXPOSE 8000

CMD ["./server"]
