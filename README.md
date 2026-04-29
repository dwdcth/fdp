# dockerpull

基于 Go + aria2c 的 Docker/OCI 镜像并发下载工具，支持 mirror 加速、断点续传、缓存复用。

## 功能特性

- 通过 aria2c 并发下载镜像 config 与 layer
- 多 mirror 轮转 + 自动回退，无凭证 mirror 自动跳过
- `-m` 参数与 `DOCKER_MIRRORS` 环境变量合并使用
- 支持导出 Docker 归档（默认）和 OCI 归档
- 已下载的 blob 自动缓存，重复拉取跳过
- SQLite 状态追踪，断点续传
- 纯 Go 编译，无需 CGO

## 安装依赖

```bash
# macOS
brew install aria2

# Ubuntu / Debian
apt install aria2
```

## 构建

```bash
go build ./cmd/dockerpull
```

## 用法

### 基本用法（通过 mirror 拉取）

```bash
./dockerpull nginx:latest \
  -m https://docker.1panel.live \
  -m https://hub.rat.dev \
  -m https://docker.xuanyuan.me \
  -m https://docker.hlmirror.com \
  -o /tmp/nginx.tar.gz
```

### 使用环境变量配置 mirror

```bash
export DOCKER_MIRRORS="https://docker.1panel.live,https://hub.rat.dev"
./dockerpull nginx:latest -o /tmp/nginx.tar.gz
```

`-m` 参数与 `DOCKER_MIRRORS` 合并生效，`-m` 优先排前面。

### 拉取指定平台

```bash
./dockerpull nginx:latest --platform linux/arm64 -o /tmp/nginx-arm64.tar.gz
```

### 导出 OCI 格式

```bash
./dockerpull ghcr.io/org/app:v1 --oci -o /tmp/app.oci.tar.gz
```

### 拉取第三方仓库镜像

```bash
./dockerpull ghcr.io/nginx/nginx:latest -o /tmp/nginx.tar.gz
./dockerpull quay.io/prometheus/prometheus:v3.0 -o /tmp/prometheus.tar.gz
```

### 调整并发参数

```bash
./dockerpull nginx:latest \
  -t 8 \    # 8 个 layer 并发下载
  -x 32 \   # aria2c 每个 server 32 连接
  -s 32 \   # aria2c 分片数 32
  -o /tmp/nginx.tar.gz
```

### 指定 aria2c 路径和缓存目录

```bash
./dockerpull nginx:latest \
  --aria2c /usr/local/bin/aria2c \
  --cache-dir ~/.dockerpull-cache \
  -o /tmp/nginx.tar.gz
```

## 参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `IMAGE[:TAG]` | - | 镜像引用，如 `nginx:latest`、`ghcr.io/org/app:v1` |
| `-m URL` | - | Registry mirror，可多次传入 |
| `-o PATH` | **必填** | 输出归档路径 |
| `-t N` | `4` | layer 并发下载 worker 数 |
| `-x N` | `16` | aria2c `--max-connection-per-server` |
| `-s N` | `16` | aria2c `--split`（分片数） |
| `--platform` | `linux/amd64` | 目标平台，格式 `os/arch[/variant]` |
| `--docker` | 默认 | 导出 Docker 归档 |
| `--oci` | - | 导出 OCI 归档（与 `--docker` 互斥） |
| `--aria2c` | `aria2c` | aria2c 可执行文件路径 |
| `--cache-dir` | `./cache` | 缓存目录 |
| `--state-db` | `<cache-dir>/state/images.db` | SQLite 状态库路径 |

## Mirror 策略

```
用户传入 mirror1, mirror2, mirror3
           ↓
    按轮转选起始 mirror
           ↓
  ┌─ mirror2 ─→ 成功 → 下载
  │     ↓ 失败
  ├─ mirror3 ─→ 成功 → 下载
  │     ↓ 失败
  ├─ mirror1 ─→ 成功 → 下载
  │     ↓ 失败
  └─ registry 源站（如 Docker Hub）
```

- 每个 blob 按 mirror 列表轮转选择起始点，不同 blob 可能走不同 mirror，实现负载均衡
- 当前 mirror 返回 403/401（需要凭证）时自动跳过，尝试下一个
- 所有 mirror 失败后回退 registry 源站
- manifest 和 blob 各自独立走 mirror fallback

## 缓存与断点续传

缓存目录结构：

```
cache/
  blobs/sha256/          # 已下载的 blob 文件
  manifests/             # 已保存的 manifest
  state/images.db        # SQLite 状态库（blob 校验状态、镜像摘要）
```

- 已下载且 digest 校验通过的 blob 不会重复下载
- aria2c 启用 `--continue=true`，支持断点续传
- 中断后重新运行同一命令即可继续

## 加载下载的镜像

```bash
# Docker 归档
docker load < /tmp/nginx.tar.gz

# 或
docker load -i /tmp/nginx.tar.gz
```

## 常见问题

### 直连 Docker Hub 失败

在某些网络环境下直连 `registry-1.docker.io` 可能超时或被重置，建议使用 mirror：

```bash
./dockerpull nginx:latest -m https://docker.1panel.live -o /tmp/nginx.tar.gz
```

### aria2c not found

确保 aria2c 已安装并在 PATH 中，或通过 `--aria2c` 指定路径：

```bash
./dockerpull nginx:latest --aria2c /opt/homebrew/bin/aria2c -o /tmp/nginx.tar.gz
```

## 技术栈

- Go 1.24+
- [modernc.org/sqlite](https://modernc.org/sqlite) — 纯 Go SQLite，无需 CGO
- [aria2c](https://aria2.github.io/) — 高性能并发下载引擎
