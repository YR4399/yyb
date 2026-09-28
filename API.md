# YYB Go 主要 API 参考

应用宝协议模拟服务（YYB Go）对外暴露一组 HTTP JSON 接口，用于**微信扫码登录、已登录账号管理、以及 wxapp 业务接口调用**。本文档汇总所有主要接口，给出路径、方法、参数与可复制的 `curl` 示例。

> 交互式文档（Swagger UI）随服务自带：访问 `/docs`；机器可读的 OpenAPI 3.0 描述文件在 `/openapi.json`。

---

## 0. 基础约定

| 项 | 说明 |
|----|------|
| 默认监听 | `127.0.0.1:8000`（建议用反向代理暴露，不要直连公网） |
| 协议 | HTTP/1.1，请求与响应均为 JSON（`Content-Type: application/json`），图片类接口除外 |
| 统一响应包 | 所有 JSON 接口返回 `{ "code": 0, "msg": "success", "data": ... }`，`code != 0` 表示业务错误 |
| 账号引用 `ref` | 创建扫码会话后，账号可用 `id` / `uin` / `openid` 任一作为 `ref` 引用；支持批量时省略 `ref` 表示全部账号 |
| 中文文件名 | 仓库含 `地区表.txt`，通过 `go:embed` 嵌入二进制，运行时无需额外文件 |

### 统一响应结构

```json
{
  "code": 0,
  "msg": "success",
  "data": { }
}
```

错误示例：

```json
{ "code": 400, "msg": "ref is required", "data": null }
```

---

## 1. 健康检查

### `GET /health`

检查服务是否存活。

```bash
curl http://localhost:8000/health
# => {"ok":true}
```

---

## 2. 品赞代理地区表

### `GET /pinzan/regions`

返回内置 `地区表.txt` 解析出的全国 / 省 / 市三级地区编码，供扫码页选择代理地区使用（全国为 `all`，省市为六位行政区划编码，如深圳 `440300`）。

```bash
curl http://localhost:8000/pinzan/regions
```

响应（节选）：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "nationwide": { "label": "全国", "code": "all" },
    "provinces": [
      {
        "label": "广东省",
        "code": "440000",
        "cities": [
          { "label": "深圳市", "code": "440300" },
          { "label": "广州市", "code": "440100" }
        ]
      }
    ]
  }
}
```

---

## 3. 微信扫码登录

扫码登录是一组**有状态会话**接口：先创建会话拿到二维码，前端轮询状态，用户扫码授权后再确认保存账号。

### `POST /qr` — 创建扫码会话

请求体（均可省略，直连模式）：

```json
{
  "use_proxy": true,
  "area": "440300"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `use_proxy` | bool | 是否走品赞优质池白名单代理完成本次扫码（默认 false 直连） |
| `area` | string | 代理地区编码；`all` 为全国随机，省市为六位编码 |

可选查询参数 `?as_base64=true` 可让响应直接带回二维码图片的 data URI，免去二次请求图片。

```bash
curl -X POST http://localhost:8000/qr \
  -H 'Content-Type: application/json' \
  -d '{"use_proxy":false}'
```

响应：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "session_id": "a1b2c3d4...",
    "status": "pending",
    "image_url": "/qr/a1b2c3d4.../image",
    "image_base64": null,
    "proxy_enabled": false,
    "proxy_area": null,
    "proxy_latency_ms": null,
    "expires_in": 60
  }
}
```

### `GET /qr/{session_id}/image` — 获取二维码图片

返回 `image/jpeg` 二维码图片。

```bash
curl http://localhost:8000/qr/a1b2c3d4.../image -o qr.jpg
```

### `GET /qr/{session_id}/poll` — 轮询扫码状态

前端按 `expires_in` 倒计时持续轮询。

```bash
curl http://localhost:8000/qr/a1b2c3d4.../poll
```

响应：

```json
{ "code": 0, "msg": "success", "data": { "status": "authorized", "errcode": null } }
```

`status` 取值：`pending` → `scanned` → `authorized` → `confirmed` / `expired` / `cancelled` / `unknown`。

### `POST /qr/{session_id}/confirm` — 确认并保存账号

用户扫码授权后调用，将账号写入本地存储。

```bash
curl -X POST http://localhost:8000/qr/a1b2c3d4.../confirm
```

响应：

```json
{ "code": 0, "msg": "success", "data": { "id": 1, "openid": "oXXX...", "created_at": 1700000000, "updated_at": 1700000000 } }
```

---

## 4. 账号管理

### `GET /accounts` — 账号列表

```bash
curl http://localhost:8000/accounts
```

响应：

```json
{
  "code": 0,
  "msg": "success",
  "data": [
    {
      "id": 1,
      "openid": "oXXX...",
      "uin": 123456,
      "alias": null,
      "nickname": "小明",
      "avatar": "/accounts/avatar?ref=1",
      "status": "alive",
      "last_checked_at": 1700000000,
      "created_at": 1700000000,
      "updated_at": 1700000000
    }
  ]
}
```

### `DELETE /accounts?ref={ref}` — 删除账号

`ref` 可为 `id` / `uin` / `openid`。

```bash
curl -X DELETE "http://localhost:8000/accounts?ref=1"
```

### `POST /accounts/refresh` — 刷新账号存活状态

不传 `ref` 刷新全部账号，返回数组；传 `ref` 刷新单个，返回对象。

```bash
curl -X POST http://localhost:8000/accounts/refresh \
  -H 'Content-Type: application/json' \
  -d '{"ref":"1"}'
```

### `POST /accounts/resync` — 重新同步账号资料

重新拉取昵称、头像等资料。

```bash
curl -X POST http://localhost:8000/accounts/resync \
  -H 'Content-Type: application/json' \
  -d '{"ref":"1"}'
```

### `GET /accounts/avatar?ref={ref}` — 获取账号头像

返回 `image/*`（或 `302` 跳转到远程头像地址）。

```bash
curl "http://localhost:8000/accounts/avatar?ref=1" -o avatar.jpg
```

---

## 5. wxapp 业务接口

以下三个接口都需要一个已登录账号的 `ref` 和目标小程序的 `app_id`。返回 `result` 为对应微信接口的原始结果。

### `POST /wxapp/getCode` — 获取小程序 code

```bash
curl -X POST http://localhost:8000/wxapp/getCode \
  -H 'Content-Type: application/json' \
  -d '{"ref":"1","app_id":"wx1234567890abcdef"}'
```

### `POST /wxapp/getPhoneNumber` — 获取手机号

```bash
curl -X POST http://localhost:8000/wxapp/getPhoneNumber \
  -H 'Content-Type: application/json' \
  -d '{"ref":"1","app_id":"wx1234567890abcdef"}'
```

### `POST /wxapp/operateWxData` — 调用云函数 / 业务数据接口

`payload` 为完整的 `operateWxData` 请求 JSON。

```bash
curl -X POST http://localhost:8000/wxapp/operateWxData \
  -H 'Content-Type: application/json' \
  -d '{
    "ref": "1",
    "app_id": "wx1234567890abcdef",
    "payload": { "appid": "wx1234567890abcdef", "data": {} }
  }'
```

响应：

```json
{ "code": 0, "msg": "success", "data": { "openid": "oXXX...", "result": { } } }
```

---

## 6. 页面与静态资源

| 路径 | 说明 |
|------|------|
| `GET /` | 控制台首页（账号列表、扫码入口） |
| `GET /scan` | 扫码登录页（支持按次开启品赞代理、选择地区、显示代理延迟） |
| `GET /docs` | Swagger UI 交互式文档 |
| `GET /openapi.json` | OpenAPI 3.0 描述文件 |
| `GET /static/*` | 静态资源 |
| `GET /pinzan/regions` | 见 §2 |

---

## 7. 部署后如何验证

```bash
# 健康检查
curl http://localhost:8000/health
# 查看完整接口文档
open http://localhost:8000/docs
# 拉取地区表
curl http://localhost:8000/pinzan/regions | head -c 200
```

详见 [README.md](./README.md) 的「Linux 单文件部署」「常用启动参数」章节。
