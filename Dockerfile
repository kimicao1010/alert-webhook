# ---------- 构建阶段 ----------
FROM golang:1.26 AS build

COPY / /src

RUN cd /src \
  && CGO_ENABLED=0 make build

# ---------- 运行阶段 ----------
FROM alpine:3.20

# adapter 需要向钉钉/飞书/企微等 HTTPS API 发请求，alpine 基础镜像必须补 CA 证书
RUN apk add --no-cache ca-certificates

COPY --from=build /src/_output/alertmanager-webhook-adapter /alertmanager-webhook-adapter

ENTRYPOINT [ "/alertmanager-webhook-adapter" ]
