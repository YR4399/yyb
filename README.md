# YYB Go

应用宝协议模拟服务，提供微信扫码登录、账号管理和 wxapp 接口调用。本仓库在原有扫码登录流程上，重点新增了可选的品赞代理功能。

## 主要 API 一览

服务启动后，所有接口均为 `http://<host>:<port>` 下的 HTTP JSON 接口，统一响应包为 `{ "code": 0, "msg": "success", "data": ... }`。完整路径、参数与可复制的 `curl` 示例见 **[API.md](./API.md)**；交互式文档在 `/docs`，机器可读描述在 `/openapi.json`。

| 分组 | 方法 & 路径 | 说明 |
|------|-------------|------|
| 健康检查 | `GET /health` | 服务存活检查 |
| 品赞代理 | `GET /pinzan/regions` | 返回全国 / 省 / 市三级代理地区编码 |
| 扫码登录 | `POST /qr` | 创建扫码会话（可指定品赞代理与地区） |
| 扫码登录 | `GET /qr/{session_id}/image` | 获取二维码图片 |
| 扫码登录 | `GET /qr/{session_id}/poll` | 轮询扫码状态 |
| 扫码登录 | `POST /qr/{session_id}/confirm` | 确认授权并保存账号 |
| 账号管理 | `GET /accounts` | 账号列表 |
| 账号管理 | `DELETE /accounts?ref=` | 删除指定账号 |
| 账号管理 | `POST /accounts/refresh` | 刷新账号存活状态 |
| 账号管理 | `POST /accounts/resync` | 重新同步账号资料 |
| 账号管理 | `GET /accounts/avatar?ref=` | 获取账号头像 |
| wxapp | `POST /wxapp/getCode` | 获取小程序 code |
| wxapp | `POST /wxapp/getPhoneNumber` | 获取手机号 |
| wxapp | `POST /wxapp/operateWxData` | 调用小程序云函数 / 业务数据接口 |

页面入口：`GET /`（控制台）、`GET /scan`（扫码页，支持按次开关品赞代理）。

## WCS 兼容接口（`/api/*` 与 `/wx/*`）

在原有的 `/wxapp/*` 与 `/accounts` 之上，本仓库额外提供了一套**与 WCS（微信协议服务器）路径风格一致的兼容接口**，便于原本对接 WCS 的客户端平滑迁移到 yyb：

| 分组 | 方法 & 路径 | 说明 |
|------|-------------|------|
| 账号 | `GET /api/accounts` | 列出全部账号（等同 `/accounts`） |
| 账号 | `POST /api/accounts/add` | 用 `login_buffer` 导入账号 |
| 账号 | `POST /api/accounts/delete` | 删除账号 |
| 账号 | `POST /api/accounts/disable` | 启用 / 禁用账号 |
| 账号 | `POST /api/accounts/remark` | 设置备注 / 别名 |
| 账号 | `POST /api/accounts/rescan` | 重新校验登录态（刷新） |
| 账号 | `GET /api/accounts/status` | 查询单个账号状态 |
| 扫码 | `POST /api/qr/start` | 创建扫码会话（镜像 `/qr`） |
| 扫码 | `GET /api/qr/status` | 轮询扫码状态（镜像 `/qr/{id}/poll`） |
| 鉴权 | `POST /api/auth/validate` | 校验调用方令牌 |
| 代理 | `GET /api/proxies` | 列出全部代理 |
| 代理 | `POST /api/proxies/add` | 新增代理 |
| 代理 | `POST /api/proxies/delete` | 删除代理 |
| 代理 | `POST /api/proxies/test` | 测试代理连通性 |
| wx | `POST /wx/code` | 获取小程序 code（镜像 `/wxapp/getCode`） |
| wx | `POST /wx/getphonenumber` | 获取手机号 |
| wx | `POST /wx/operateWxData` | 小程序云函数 |
| wx | `POST /wx/getuserinfo` | 获取已存账号资料 |
| wx | `POST /wx/getsession` | 获取协议会话状态 |
| wx | `POST /wx/refresh` | 刷新并保存登录态 |

代理可在 `POST /wx/*` 调用时通过 `proxy`（地址）或 `proxy_id`（代理库 id）参数选用，真正生效于微信协议链路。WCS 的其余 `/wx/*` 操作（`oauth`、`qrcodeauth`、`cloud`、`gateway`、`translatelink`、`encryptkey`、`appmsgext`、`appmsglike` 等）yyb 核心协议层尚未实现，调用时返回 `501` 并列出已支持的接口，避免伪造行为。

## 本项目的主要修改：二维码代理功能

> 核心改动：用户获取二维码时，可以自行选择是否通过品赞代理完成本次扫码登录。

- **代理按次开启**：每次获取新二维码时独立选择使用代理或保持原有直连模式，不会修改全局网络设置。
- **支持省市选择**：对接品赞优质池白名单模式，可按 `地区表.txt` 中的官方编码选择全国、省份或城市代理。
- **自动检测可用性**：代理提取后先连接二维码实际依赖的微信开放平台，检查是否联通并计算延迟毫秒数。
- **不可用自动更换**：健康检查失败后自动重新提取新代理，最多尝试 3 个；全部不可用时明确报错，不会静默切换成直连。
- **同一会话全程使用代理**：OAuth 初始化、二维码图片下载、扫码状态轮询和 OAuth 回调均绑定到同一个已验证代理。
- **一分钟有效期**：默认提取 1 分钟代理，页面显示二维码剩余时间；到期后停止轮询并提示用户重新获取。
- **配置不会暴露到网页**：品赞套餐编号和提取密钥通过环境变量配置，不返回前端，也不写入业务日志。

## Linux 单文件部署

按下文构建得到的无后缀 `yyb-go` 适用于 Linux x86_64/amd64 服务器，运行时不需要安装 Go。程序已经嵌入网页模板和默认品赞地区表，只需上传这一个文件：

```bash
chmod +x yyb-go
export YYB_PINZAN_NO="你的套餐购买编号"
export YYB_PINZAN_SECRET="你的套餐提取密钥"
./yyb-go
```

程序默认监听 `127.0.0.1:8000`。首次运行会在当前目录自动创建 `resource/db`、`resource/qr` 等运行数据目录；部署时不需要另外上传 `resource` 或 `地区表.txt`。如需覆盖内置地区表，仍可通过 `-pinzan-regions` 指定外部文件。

## 服务器构建

源码包也可在安装了 Go 1.23 或更高版本的 Linux/Windows 服务器上自行构建。

解压源码包并进入项目目录：

```bash
go test ./...
go build -trimpath -ldflags="-s -w" -o yyb-go ./cmd/yyb-go
```

直接模式无需品赞配置：

```bash
./yyb-go
```

服务默认监听 `127.0.0.1:8000`。建议保留本机监听并通过带访问控制的反向代理提供服务；这个控制台包含账号登录能力，不应直接裸露在公网。

## 品赞优质池白名单代理

先在品赞后台把服务器公网出口 IP 加入对应套餐白名单，再设置套餐购买编号和提取密钥：

```bash
export YYB_PINZAN_NO="你的套餐购买编号"
export YYB_PINZAN_SECRET="你的套餐提取密钥"
./yyb-go
```

扫码页面可以为每次新二维码单独开启或关闭代理。省市选项来自项目根目录的 `地区表.txt`：全国发送 `area=all`，省市发送表中的六位编码，例如深圳市发送 `area=440300`。

代理提取固定使用以下参数：

```text
mode=whitelist
pool=quality
protocol=1
format=json
num=1
```

代理会先访问二维码实际依赖的微信开放平台进行健康检查，确认代理能够连接目标站点，并记录往返延迟毫秒数。检查失败会自动丢弃并重新提取，最多尝试 3 个新代理；全部失败才会显示错误，且不会自动改为直连。检查通过后，页面会显示代理延迟，再将同一代理用于二维码会话的 OAuth 初始化、二维码图片下载、扫码轮询和 OAuth 回调。这里不再把第三方公网 IP 查询站点作为硬性检查，避免代理能访问微信却因为检测站点返回 502 或超时而被误判。

代理提取默认发送 `minute=1`。代理二维码按实际剩余时间显示最多 60 秒倒计时；到期后停止轮询并显示过期，必须由用户手动点击重新生成。需要其他套餐时长时可以指定：

```bash
./yyb-go -pinzan-minute 3
```

允许的值为 `1`、`3`、`5`、`10`、`15`、`30`。程序不会自动增删品赞白名单，也不会把套餐编号、提取密钥或完整代理 IP 返回到网页或写入业务日志。

## 常用启动参数

```text
-host              监听地址，默认 127.0.0.1
-port              监听端口，默认 8000
-resource-root     运行资源目录，默认 resource
-tcp-proxy         可选 TCP 代理 socks5://host:port 或 http-connect://host:port，作为 /wx/* 默认出口
-pinzan-regions    品赞优质池地区编码表，默认 地区表.txt
-pinzan-minute     代理时长，默认 1 分钟
YYB_API_TOKEN      可选：设置后 /api/auth/validate 校验此令牌（不设置则为开放模式）
YYB_PINZAN_NO      品赞套餐购买编号
YYB_PINZAN_SECRET  品赞提取密钥
```

如确需直接监听全部网卡：

```bash
./yyb-go -host 0.0.0.0 -port 8000
```

请同时使用防火墙、反向代理鉴权或内网访问限制保护该端口。
