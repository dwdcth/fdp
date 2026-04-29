# dockerpull

`dockerpull` 是一个基于 Go 和 `aria2c` 的 Docker/OCI 镜像并发下载器。

## 功能

- 从 Docker Registry HTTP API V2 拉取 manifest 和 blobs
- 通过 `aria2c` 并发下载 config 与 layers
- 支持多个 mirror 轮转起始与失败回退
- 支持 `-m` 与 `DOCKER_MIRRORS` 合并，CLI 优先
- 默认导出 Docker 归档，支持 `--oci`
- 使用 `modernc.org/sqlite`，无需 CGO

## 依赖

- Go 1.24+
- `aria2c`

## 构建

```bash
go build ./cmd/dockerpull
```

## 用法

```bash
./dockerpull nginx:latest \
  -m https://mirror1.example.com \
  -m https://mirror2.example.com \
  -t 4 \
  -x 16 \
  -s 16 \
  -o /tmp/nginx.tar.gz
```

环境变量 mirror：

```bash
DOCKER_MIRRORS="https://m1.example.com,https://m2.example.com" ./dockerpull nginx:latest -o /tmp/nginx.tar.gz
```

导出 OCI：

```bash
./dockerpull ghcr.io/org/app:v1 --oci -o /tmp/app.oci.tar.gz
```

指定平台：

```bash
./dockerpull nginx:latest --platform linux/arm64 -o /tmp/nginx-arm64.tar.gz
```

## 参数

- `-m`：镜像 mirror，可重复传入
- `-t`：layer 并发 worker 数
- `-x`：透传 aria2c `--max-connection-per-server`
- `-s`：透传 aria2c `--split`
- `-o`：输出归档路径
- `--platform`：目标平台，默认 `linux/amd64`
- `--docker`：导出 Docker 归档
- `--oci`：导出 OCI 归档
- `--aria2c`：aria2c 可执行文件路径
- `--cache-dir`：缓存目录，默认 `./cache`
- `--state-db`：SQLite 状态库路径

## Mirror 策略

- 每个任务按 mirror 列表轮转选择起始 mirror
- 当前 mirror 失败时自动回退剩余 mirror
- 所有 mirror 失败后回退 registry 源站

## 缓存目录

```text
cache/
  blobs/
  manifests/
  state/images.db
```

## 说明

当前版本优先打通下载、缓存和导出主链路。Docker 归档与 OCI 归档都按单平台镜像生成。
