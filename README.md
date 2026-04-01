# docker发布。请关注原作者
https://github.com/vanadium23/kompanion

1. 汉化
2. 增加删除书籍功能
3. 增加删除设备功能
4. 统计页面按照个人喜好做了更改
5. 书籍详情页增加书籍上传功能，方便连续上传多本书籍
6. 书籍详情页增加默认值，方便部分书籍不带元信息也可以直接保存
7. 增加笔记导出与展示。

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
5. joplin配置：
   1.笔记服务商joplin
   2.IP+8322+token
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
      - "8322:8080"            # Web / OPDS / 进度同步 /joplin
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


