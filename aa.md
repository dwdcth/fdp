可以做。核心不是“多线程下载 Docker 镜像”，而是：

**先通过 Docker Registry HTTP API V2 解析镜像 manifest，再把每个 layer blob 的下载地址交给 aria2c 并发下载。**

Docker Registry V2 拉 manifest 的接口是：

```http
GET /v2/<name>/manifests/<reference>
```

拉 layer blob 的接口是：

```http
GET /v2/<name>/blobs/<digest>
```

layer blob 通常支持重定向、缓存和 Range 请求，所以适合交给 aria2c 做断点续传和多连接下载。([Docker Documentation][1])

---

# 一、整体架构

```text
┌────────────────────┐
│ images.yaml         │
│ 配置镜像列表/轮询间隔 │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│ Poller              │
│ 定时轮询镜像 tag/digest │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│ Registry Client     │
│ 获取 token / manifest │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│ Manifest Parser     │
│ 解析平台/层/layers    │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│ Download Scheduler  │
│ 去重/缓存/任务队列     │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│ aria2c Downloader   │
│ 多连接/断点/限速下载   │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│ Local Cache         │
│ blobs/manifest/index │
└────────────────────┘
```

---

# 二、配置文件设计

例如 `images.yaml`：

```yaml
poll_interval_seconds: 300

download:
  dir: ./cache
  aria2c_path: aria2c
  max_concurrent_layers: 4
  split: 16
  min_split_size: 1M
  continue: true
  check_integrity: true
  max_connection_per_server: 16

registries:
  docker.io:
    username: ""
    password: ""

images:
  - name: nginx
    registry: docker.io
    repository: library/nginx
    reference: latest
    platform:
      os: linux
      architecture: amd64

  - name: redis
    registry: docker.io
    repository: library/redis
    reference: "7"
    platform:
      os: linux
      architecture: amd64
```

---

# 三、关键流程

## 1. 解析镜像名

支持几种形式：

```text
nginx:latest
ubuntu:22.04
docker.io/library/nginx:latest
ghcr.io/owner/image:tag
registry.example.com/ns/app:v1
```

规则：

```text
nginx:latest
=> registry = registry-1.docker.io
=> repository = library/nginx
=> reference = latest
```

Docker Hub 官方镜像需要自动补 `library/`。

---

## 2. 获取认证 Token

访问 manifest 时，很多 Registry 会返回：

```http
401 Unauthorized
WWW-Authenticate: Bearer realm="...",service="...",scope="repository:library/nginx:pull"
```

然后请求：

```http
GET <realm>?service=<service>&scope=repository:<repo>:pull
```

得到：

```json
{
  "token": "xxx"
}
```

后续请求带：

```http
Authorization: Bearer xxx
```

---

## 3. 获取 manifest

请求头要带多个 `Accept`，因为镜像可能返回 manifest list，也可能直接返回 image manifest。

```http
GET /v2/library/nginx/manifests/latest
Accept: application/vnd.oci.image.index.v1+json
Accept: application/vnd.docker.distribution.manifest.list.v2+json
Accept: application/vnd.oci.image.manifest.v1+json
Accept: application/vnd.docker.distribution.manifest.v2+json
```

如果返回的是 **manifest list / image index**，需要按平台选择真正的 manifest：

```json
{
  "manifests": [
    {
      "digest": "sha256:xxx",
      "platform": {
        "os": "linux",
        "architecture": "amd64"
      }
    }
  ]
}
```

然后再请求：

```http
GET /v2/library/nginx/manifests/sha256:xxx
```

如果返回的是 image manifest，就能拿到 layers：

```json
{
  "schemaVersion": 2,
  "config": {
    "digest": "sha256:..."
  },
  "layers": [
    {
      "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
      "digest": "sha256:...",
      "size": 123456
    }
  ]
}
```

OCI/Docker 镜像 manifest 都是通过 layer descriptors 引用 layer blob，descriptor 里包含 digest、mediaType、size 等信息。([GitHub][2])

---

## 4. 下载 layers

每个 layer 的下载 URL：

```text
https://registry-1.docker.io/v2/library/nginx/blobs/sha256:xxxx
```

但是要注意：这个接口经常返回 `307 Temporary Redirect` 到 CDN 地址。

aria2c 可以直接处理重定向，但 Authorization 头不一定会被安全地转发到重定向后的域名。稳妥做法有两个：

## 方案 A：直接让 aria2c 请求 registry blob URL

```bash
aria2c \
  -x 16 \
  -s 16 \
  -k 1M \
  -c \
  --header="Authorization: Bearer <token>" \
  -d ./cache/blobs/sha256 \
  -o <digest>.tar.gz \
  "https://registry-1.docker.io/v2/library/nginx/blobs/sha256:xxxx"
```

优点：简单。

缺点：部分 Registry 的重定向认证可能有坑。

## 方案 B：程序先 HEAD/GET 拿到最终重定向 URL，再交给 aria2c

```text
程序请求 blob URL
    ↓
拿到 307 Location
    ↓
aria2c 下载 Location
```

优点：更稳定。

缺点：CDN URL 可能有时效，不能长期缓存 URL。

推荐用 **方案 B**。

---

# 四、目录结构

建议保存成 OCI 风格缓存：

```text
cache/
  manifests/
    docker.io/
      library/
        nginx/
          latest.json
          sha256_xxx.json

  blobs/
    sha256/
      abcd....
      efgh....

  state/
    images.db
```

`images.db` 可以用 SQLite：

```sql
CREATE TABLE image_state (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    registry TEXT NOT NULL,
    repository TEXT NOT NULL,
    reference TEXT NOT NULL,
    platform_os TEXT NOT NULL,
    platform_arch TEXT NOT NULL,
    manifest_digest TEXT,
    last_checked_at INTEGER,
    last_changed_at INTEGER,
    UNIQUE(registry, repository, reference, platform_os, platform_arch)
);

CREATE TABLE blob_state (
    digest TEXT PRIMARY KEY,
    size INTEGER,
    media_type TEXT,
    local_path TEXT,
    downloaded INTEGER DEFAULT 0,
    verified INTEGER DEFAULT 0,
    created_at INTEGER,
    updated_at INTEGER
);
```

---

# 五、轮询逻辑

伪代码：

```go
for {
    images := loadConfig()

    for _, img := range images {
        go syncImage(img)
    }

    sleep(pollInterval)
}
```

`syncImage`：

```go
func syncImage(img ImageConfig) error {
    token := registry.GetToken(img.Registry, img.Repository)

    rawManifest := registry.GetManifest(
        img.Registry,
        img.Repository,
        img.Reference,
        token,
    )

    manifestDigest := sha256(rawManifest)

    oldDigest := db.GetManifestDigest(img)

    if oldDigest == manifestDigest {
        log.Println("image unchanged:", img.Repository, img.Reference)
        return nil
    }

    finalManifest := resolvePlatformManifest(rawManifest, img.Platform)

    layers := parseLayers(finalManifest)

    for _, layer := range layers {
        if cache.ExistsAndVerified(layer.Digest, layer.Size) {
            continue
        }

        scheduler.Add(layer)
    }

    scheduler.Wait()

    db.UpdateImageManifestDigest(img, manifestDigest)

    return nil
}
```

---

# 六、aria2c 下载器设计

不要为每个 layer 都启动一个 aria2c 进程也可以，但实现简单的话可以先这样。

## 单 layer 下载命令

```bash
aria2c \
  --continue=true \
  --allow-overwrite=true \
  --auto-file-renaming=false \
  --max-connection-per-server=16 \
  --split=16 \
  --min-split-size=1M \
  --dir=./cache/blobs/sha256 \
  --out=abcdef... \
  "https://cdn-registry-url/xxx"
```

## 多 layer 并发

Go 里面用 worker pool：

```go
type LayerTask struct {
    Digest string
    Size   int64
    URL    string
    Path   string
}

func RunDownloadPool(tasks []LayerTask, workers int) error {
    ch := make(chan LayerTask)
    errCh := make(chan error, len(tasks))

    var wg sync.WaitGroup

    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            for task := range ch {
                if err := DownloadWithAria2c(task); err != nil {
                    errCh <- err
                }
            }
        }()
    }

    for _, task := range tasks {
        ch <- task
    }

    close(ch)
    wg.Wait()
    close(errCh)

    for err := range errCh {
        if err != nil {
            return err
        }
    }

    return nil
}
```

---

# 七、Go 关键代码骨架

## 1. manifest 结构

```go
type Descriptor struct {
    MediaType string `json:"mediaType"`
    Digest    string `json:"digest"`
    Size      int64  `json:"size"`
    Platform  *struct {
        OS           string `json:"os"`
        Architecture string `json:"architecture"`
        Variant      string `json:"variant,omitempty"`
    } `json:"platform,omitempty"`
}

type ManifestList struct {
    SchemaVersion int          `json:"schemaVersion"`
    MediaType     string       `json:"mediaType"`
    Manifests     []Descriptor `json:"manifests"`
}

type ImageManifest struct {
    SchemaVersion int          `json:"schemaVersion"`
    MediaType     string       `json:"mediaType"`
    Config        Descriptor   `json:"config"`
    Layers        []Descriptor `json:"layers"`
}
```

---

## 2. 拉 manifest

```go
func GetManifest(registry, repo, reference, token string) ([]byte, string, error) {
    url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repo, reference)

    req, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return nil, "", err
    }

    req.Header.Set("Accept", strings.Join([]string{
        "application/vnd.oci.image.index.v1+json",
        "application/vnd.docker.distribution.manifest.list.v2+json",
        "application/vnd.oci.image.manifest.v1+json",
        "application/vnd.docker.distribution.manifest.v2+json",
    }, ", "))

    if token != "" {
        req.Header.Set("Authorization", "Bearer "+token)
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, "", fmt.Errorf("get manifest failed: %s, body=%s", resp.Status, string(body))
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, "", err
    }

    digest := resp.Header.Get("Docker-Content-Digest")
    return body, digest, nil
}
```

---

## 3. 判断 manifest 类型

```go
func IsManifestList(mediaType string) bool {
    return mediaType == "application/vnd.oci.image.index.v1+json" ||
        mediaType == "application/vnd.docker.distribution.manifest.list.v2+json"
}

func IsImageManifest(mediaType string) bool {
    return mediaType == "application/vnd.oci.image.manifest.v1+json" ||
        mediaType == "application/vnd.docker.distribution.manifest.v2+json"
}
```

---

## 4. 选择平台 manifest

```go
func SelectPlatformManifest(raw []byte, osName, arch string) (string, error) {
    var list ManifestList
    if err := json.Unmarshal(raw, &list); err != nil {
        return "", err
    }

    for _, m := range list.Manifests {
        if m.Platform == nil {
            continue
        }

        if m.Platform.OS == osName && m.Platform.Architecture == arch {
            return m.Digest, nil
        }
    }

    return "", fmt.Errorf("platform not found: %s/%s", osName, arch)
}
```

---

## 5. 获取 blob 重定向地址

```go
func GetBlobRedirectURL(registry, repo, digest, token string) (string, error) {
    url := fmt.Sprintf("https://%s/v2/%s/blobs/%s", registry, repo, digest)

    client := &http.Client{
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse
        },
    }

    req, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return "", err
    }

    if token != "" {
        req.Header.Set("Authorization", "Bearer "+token)
    }

    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusTemporaryRedirect ||
        resp.StatusCode == http.StatusFound ||
        resp.StatusCode == http.StatusSeeOther {
        loc := resp.Header.Get("Location")
        if loc == "" {
            return "", fmt.Errorf("redirect without location")
        }
        return loc, nil
    }

    if resp.StatusCode == http.StatusOK {
        return url, nil
    }

    body, _ := io.ReadAll(resp.Body)
    return "", fmt.Errorf("get blob url failed: %s, body=%s", resp.Status, string(body))
}
```

---

## 6. 调 aria2c 下载

```go
func DownloadWithAria2c(aria2cPath string, task LayerTask, cfg DownloadConfig) error {
    args := []string{
        "--continue=true",
        "--allow-overwrite=true",
        "--auto-file-renaming=false",
        fmt.Sprintf("--max-connection-per-server=%d", cfg.MaxConnectionPerServer),
        fmt.Sprintf("--split=%d", cfg.Split),
        fmt.Sprintf("--min-split-size=%s", cfg.MinSplitSize),
        "--dir=" + filepath.Dir(task.Path),
        "--out=" + filepath.Base(task.Path),
        task.URL,
    }

    cmd := exec.Command(aria2cPath, args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    return cmd.Run()
}
```

---

# 八、完整任务流程伪代码

```go
func SyncImage(img ImageConfig, cfg Config) error {
    token, err := GetRegistryToken(img.Registry, img.Repository)
    if err != nil {
        return err
    }

    raw, manifestDigest, err := GetManifest(
        img.Registry,
        img.Repository,
        img.Reference,
        token,
    )
    if err != nil {
        return err
    }

    mediaType := DetectMediaType(raw)

    if IsManifestList(mediaType) {
        selectedDigest, err := SelectPlatformManifest(
            raw,
            img.Platform.OS,
            img.Platform.Architecture,
        )
        if err != nil {
            return err
        }

        raw, manifestDigest, err = GetManifest(
            img.Registry,
            img.Repository,
            selectedDigest,
            token,
        )
        if err != nil {
            return err
        }
    }

    var manifest ImageManifest
    if err := json.Unmarshal(raw, &manifest); err != nil {
        return err
    }

    var tasks []LayerTask

    for _, layer := range manifest.Layers {
        localPath := BlobPath(cfg.Download.Dir, layer.Digest)

        if BlobExistsAndValid(localPath, layer.Digest, layer.Size) {
            continue
        }

        url, err := GetBlobRedirectURL(
            img.Registry,
            img.Repository,
            layer.Digest,
            token,
        )
        if err != nil {
            return err
        }

        tasks = append(tasks, LayerTask{
            Digest: layer.Digest,
            Size:   layer.Size,
            URL:    url,
            Path:   localPath,
        })
    }

    if err := RunDownloadPool(tasks, cfg.Download.MaxConcurrentLayers); err != nil {
        return err
    }

    for _, layer := range manifest.Layers {
        localPath := BlobPath(cfg.Download.Dir, layer.Digest)
        if err := VerifyDigest(localPath, layer.Digest); err != nil {
            return err
        }
    }

    SaveManifest(img, manifestDigest, raw)

    return nil
}
```

---

# 九、digest 校验

layer digest 是内容地址，必须校验：

```go
func VerifyDigest(path string, digest string) error {
    parts := strings.SplitN(digest, ":", 2)
    if len(parts) != 2 {
        return fmt.Errorf("invalid digest: %s", digest)
    }

    algo := parts[0]
    expect := parts[1]

    if algo != "sha256" {
        return fmt.Errorf("unsupported digest algo: %s", algo)
    }

    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return err
    }

    actual := hex.EncodeToString(h.Sum(nil))

    if actual != expect {
        return fmt.Errorf("digest mismatch: %s != %s", actual, expect)
    }

    return nil
}
```

---

# 十、是否需要组装成 Docker 镜像？

如果只是下载和缓存 layer，不需要。

如果你想最终生成可导入的文件：

```bash
docker load -i image.tar
```

那你需要组装成 Docker save 格式或 OCI layout。

更推荐生成 **OCI layout**：

```text
oci-image/
  oci-layout
  index.json
  blobs/
    sha256/
      manifest
      config
      layer1
      layer2
```

`oci-layout`：

```json
{
  "imageLayoutVersion": "1.0.0"
}
```

`index.json`：

```json
{
  "schemaVersion": 2,
  "manifests": [
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "digest": "sha256:...",
      "size": 1234,
      "annotations": {
        "org.opencontainers.image.ref.name": "latest"
      }
    }
  ]
}
```

---

# 十一、推荐 MVP 版本

第一版不要做太复杂，建议只做：

1. 读取 `images.yaml`
2. 支持 Docker Hub 公共镜像
3. 获取 token
4. 获取 manifest list
5. 选择 `linux/amd64`
6. 解析 layers
7. 用 aria2c 下载 layers
8. sha256 校验
9. SQLite 记录状态
10. 定时轮询 tag 是否变化

第一版目录：

```text
docker-aria2-puller/
  cmd/
    puller/
      main.go
  internal/
    config/
    registry/
    manifest/
    downloader/
    scheduler/
    cache/
    state/
  configs/
    images.yaml
  go.mod
```

---

# 十二、给 AI 编程助手的实现提示词

你可以直接把下面这段丢给 AI 编程助手：

```text
请用 Go 实现一个基于 aria2c 的 Docker/OCI 镜像多线程下载器。

功能要求：

1. 读取 images.yaml 配置。
2. 配置中包含：
   - poll_interval_seconds
   - download.dir
   - download.aria2c_path
   - download.max_concurrent_layers
   - download.split
   - download.min_split_size
   - download.max_connection_per_server
   - images 列表
3. 每个 image 包含：
   - registry
   - repository
   - reference
   - platform.os
   - platform.architecture
4. 支持 Docker Registry HTTP API V2。
5. 支持 Bearer Token 鉴权。
6. 支持 Docker Hub 公共镜像，例如 library/nginx:latest。
7. 获取 manifest 时 Accept 需要支持：
   - application/vnd.oci.image.index.v1+json
   - application/vnd.docker.distribution.manifest.list.v2+json
   - application/vnd.oci.image.manifest.v1+json
   - application/vnd.docker.distribution.manifest.v2+json
8. 如果返回 manifest list / image index，则根据 platform 选择具体 manifest digest。
9. 解析 image manifest 中的 config 和 layers。
10. 每个 layer 根据 digest 下载：
    GET /v2/<repo>/blobs/<digest>
11. 下载前先获取重定向 Location，如果有 Location，就把最终 URL 交给 aria2c。
12. aria2c 使用参数：
    - --continue=true
    - --allow-overwrite=true
    - --auto-file-renaming=false
    - --split=<config>
    - --max-connection-per-server=<config>
    - --min-split-size=<config>
13. layer 下载到：
    cache/blobs/sha256/<hash>
14. 下载后使用 sha256 校验 digest。
15. 使用 SQLite 记录：
    - 镜像当前 manifest digest
    - blob 下载状态
    - blob 校验状态
16. 轮询时如果 manifest digest 没变化，则跳过。
17. 如果 manifest digest 变化，则只下载本地缺失的 layers。
18. 代码结构清晰，分为：
    - internal/config
    - internal/registry
    - internal/manifest
    - internal/downloader
    - internal/cache
    - internal/state
    - internal/scheduler
19. 给出可运行 main.go。
20. 给出 README，包含运行方式和 images.yaml 示例。
```

---

# 十三、几个坑要提前处理

1. **latest 不稳定**
   轮询 `latest` 时 manifest digest 可能变化，应该以 `Docker-Content-Digest` 或 manifest 内容 sha256 作为变化判断。

2. **manifest list digest 和 image manifest digest 不同**
   一个是多平台索引 digest，一个是具体平台 manifest digest。建议两个都记录。

3. **aria2c 多连接不等于 layer 多线程**
   `-x/-s` 是单个文件内部多连接，`max_concurrent_layers` 是多个 layer 并发，两者都要控制。

4. **私有仓库认证更复杂**
   MVP 先支持匿名 token 和 basic credentials，后面再扩展 Harbor、GHCR、阿里云、腾讯云等。

5. **不要只看文件存在**
   必须校验 size 和 sha256，否则断点残留文件可能误判为成功。

6. **blob 去重很重要**
   不同镜像可能共享同一个 layer digest，本地以 digest 存储即可天然去重。

---

结论：这个方案完全可行。建议你第一版先实现 **“轮询 + 解析 manifest + aria2c 下载 layers + digest 校验”**，不要一开始就做 `docker load` 兼容包。等下载缓存稳定后，再加 OCI layout 导出。

[1]: https://docs.docker.com/reference/api/registry/latest/?utm_source=chatgpt.com "Supported registry API for Docker Hub"
[2]: https://github.com/opencontainers/distribution-spec/blob/main/spec.md?utm_source=chatgpt.com "distribution-spec/spec.md at main"
