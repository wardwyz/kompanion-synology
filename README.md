# docker发布。请关注原作者
https://github.com/vanadium23/kompanion

1. 汉化
2. 增加删除书籍功能
3. 增加删除设备功能
4. 统计页面按照个人喜好做了更改
5. 书籍详情页增加书籍上传功能，方便连续上传多本书籍
6. 书籍详情页增加默认值，方便部分书籍不带元信息也可以直接保存

## 主要功能：
- 上传图书并查看书架
- 通过 OPDS 下载图书
- 提供 KOReader 阅读进度同步 API
- 通过 WebDAV 同步 KOReader 阅读统计

## KOReader 中配置：
1. 打开 `Cloud storage`
   1. 新增一个 WebDAV：
      - URL：`https://your-kompanion.org/webdav/`
      - 用户名：设备名
      - 密码：设备密码
2. 打开 `Statistics -> Settings -> Cloud sync`
   1. 即使列表是空的也没关系，直接点击 **Long press to choose current folder**。
3. 打开任意书籍后进入 `Tools -> Progress sync`
   1. 自定义同步服务器：`https://your-kompanion.org/`
   2. 登录信息：用户名为设备名，密码为设备密码
4. 配置 OPDS 目录：
   1. `Toolbar -> Search -> OPDS Catalog`
   2. 点击加号
   3. 目录地址：`https://your-kompanion.org/opds/`
   4. 用户名：设备名
   5. 密码：设备密码
## ios平台使用Readest类似配置
## docker-compose
XXX 更换成自己的用户名密码
```
version: '3.9'

services:
  # ---------- PostgreSQL 数据库 ----------
  postgres:
    image: postgres:16
    container_name: ko-postgres
    restart: unless-stopped
    volumes:
      - ./pgdata:/var/lib/postgresql/data
    environment:
      POSTGRES_USER: XXX
      POSTGRES_PASSWORD: XXX
      POSTGRES_DB: postgres

  # ---------- Kompanion 服务 ----------
  app:
    image: ghcr.io/wardwyz/kompanion:latest
    container_name: ko-web
    restart: unless-stopped
    ports:
      - "8322:8080"            # Web / OPDS / 进度同步
      - "8323:8080"            # 可选：给 KOReader Joplin 单独映射一个端口
    volumes:
      - ./data:/data           # 持久化数据
    environment:
      KOMPANION_PG_URL: "postgres://XXX:XXX@postgres:5432/postgres"
      KOMPANION_AUTH_USERNAME: XXX
      KOMPANION_AUTH_PASSWORD: XXX
      KOMPANION_JOPLIN_TOKEN: XXX # 手动设置，供 KOReader Joplin 插件使用
    depends_on:
      - postgres
```

说明：上面 `8322:8080` / `8323:8080` 的写法，**容器内监听端口都是 8080**，只是对外暴露了两个入口端口。
如果你需要“Joplin 在容器内也使用不同监听端口”，请看下面「双容器分离」方案。


## Joplin（KOReader 笔记同步）配置

新增了一个兼容 Joplin Clipper API 的接口，用于接收 KOReader 发送的 Markdown 笔记。

> 推荐（适配只支持 IP+端口+Token 的 KOReader）：
> 在 `docker-compose` 里给同一个容器额外映射一个“Joplin专用端口”（例如 `8323:8080`），
> 然后在 KOReader 的 Joplin 插件里只填写 `http://群晖IP:8323` + Token。

### （可选）双容器分离：Joplin 独立内部监听端口

如果你更希望从概念和配置上完全分开，可以再启动一个 `app-joplin` 容器，并设置不同的 `KOMPANION_HTTP_PORT`：

```yaml
services:
  app:
    image: ghcr.io/wardwyz/kompanion:latest
    container_name: ko-web
    environment:
      KOMPANION_HTTP_PORT: "8080"
      KOMPANION_PG_URL: "postgres://XXX:XXX@postgres:5432/postgres"
      KOMPANION_AUTH_USERNAME: XXX
      KOMPANION_AUTH_PASSWORD: XXX
      KOMPANION_JOPLIN_TOKEN: XXX
    ports:
      - "8322:8080"

  app-joplin:
    image: ghcr.io/wardwyz/kompanion:latest
    container_name: ko-web-joplin
    environment:
      KOMPANION_HTTP_PORT: "8081"
      KOMPANION_PG_URL: "postgres://XXX:XXX@postgres:5432/postgres"
      KOMPANION_AUTH_USERNAME: XXX
      KOMPANION_AUTH_PASSWORD: XXX
      KOMPANION_JOPLIN_TOKEN: XXX
    ports:
      - "8323:8081"
```

这样就是：
- Web/OPDS 走 `IP:8322`（容器内 8080）
- Joplin 走 `IP:8323`（容器内 8081）

1. 在容器中**手动设置** `KOMPANION_JOPLIN_TOKEN`（建议随机字符串，必填）。
2. 在 KOReader 的 Joplin 插件中配置：
   - Server/Base URL: `http://群晖IP:8323`（推荐，单独端口）
   - 或：`https://your-kompanion.org/joplin`（反代/域名模式）
   - Token: 你配置的 `KOMPANION_JOPLIN_TOKEN`
   - 如果 KOReader 版本只支持 `IP + Port`，无法填写路径，也可以直接填 `http://IP:PORT`（Kompanion 同时兼容 `/joplin/*` 和根路径 `/*` 的 Joplin API）。
3. 先在浏览器自检（把 TOKEN 换成你的值）：
   - `https://your-kompanion.org/joplin/ping?token=TOKEN` 应返回 `JoplinClipperServer`
   - `https://your-kompanion.org/joplin/folders?token=TOKEN` 应返回包含 `KOReader Notes` 的 JSON
   - 如果你在群晖通过 `IP:8322` 使用，可直接测：
     - `http://IP:8322/ping?token=TOKEN`
     - `http://IP:8322/folders?token=TOKEN`
4. 同步后的笔记可在：
   - 全部笔记：`/notes/`
   - 单本书详情页：`/books/:bookID`

接口示例（创建笔记）：

```bash
curl -X POST "http://127.0.0.1:8080/joplin/notes?token=<YOUR_TOKEN>" \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "KOReader Note",
    "body": "# Reading note\n\nSome markdown",
    "document_id": "<koreader_partial_md5>"
  }'
```

排障（KOReader 提示“导出失败”时建议按顺序检查）：

```bash
# 1) 直连 root path（IP + 端口模式）
curl -i "http://IP:8322/ping?token=<YOUR_TOKEN>"

# 2) 直连 /joplin path（带前缀模式）
curl -i "http://IP:8322/joplin/ping?token=<YOUR_TOKEN>"

# 3) 尝试创建 markdown 笔记（root path）
curl -i -X POST "http://IP:8322/notes?token=<YOUR_TOKEN>" \
  -H 'Content-Type: application/json' \
  -d '{"title":"KOReader Note","body":"# Reading note\n\nfrom synology"}'

# 4) 查看是否写入成功
curl -i "http://IP:8322/notes?token=<YOUR_TOKEN>"
```

如果你给 Joplin 单独映射了端口（示例 `8323`），把上面命令里的 `8322` 全部替换成 `8323` 即可。
