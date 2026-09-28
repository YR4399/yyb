package httpapi

func newOpenAPISpec() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "YYB Go 接口文档",
			"description": "用于微信扫码登录、账号管理和 wxapp 接口调用的 API。提供与 WCS 兼容的 /api/* 与 /wx/* 路径。",
			"version":     "1.1.0",
		},
		"servers": []map[string]any{
			{"url": "/"},
		},
		"tags": []map[string]any{
			{"name": "health", "description": "服务健康检查"},
			{"name": "qr", "description": "微信扫码登录"},
			{"name": "accounts", "description": "已保存的微信账号"},
			{"name": "wxapp", "description": "wxapp 业务接口调用"},
			{"name": "wcs", "description": "WCS 兼容接口（仿照 WCS 的 /api/* 与 /wx/* 路径）"},
		},
		"paths": map[string]any{
			"/health": map[string]any{
				"get": openAPIOperation(
					[]string{"health"},
					"检查服务状态",
					nil,
					nil,
					defaulted(map[string]any{
						"200": jsonResponse("服务正常。", refSchema("HealthResponse")),
					}),
				),
			},
			"/pinzan/regions": map[string]any{
				"get": openAPIOperation(
					[]string{"qr"},
					"获取品赞优质池省市编码",
					nil,
					nil,
					defaulted(map[string]any{
						"200": jsonResponse("地区编码目录。", refSchema("PinzanRegionCatalog")),
					}),
				),
			},
			"/qr": map[string]any{
				"post": openAPIOperation(
					[]string{"qr"},
					"创建扫码登录会话",
					[]map[string]any{
						boolQueryParam("as_base64", "是否同时返回二维码图片的 data URI。"),
					},
					jsonOptionalRequestBody(refSchema("QRCreateRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("二维码会话创建成功。", refSchema("QRCreateResponse")),
					}),
				),
			},
			"/qr/{session_id}/image": map[string]any{
				"get": openAPIOperation(
					[]string{"qr"},
					"获取二维码图片",
					[]map[string]any{pathStringParam("session_id", "二维码会话 ID。")},
					nil,
					defaulted(map[string]any{
						"200": imageResponse("二维码图片。"),
					}),
				),
			},
			"/qr/{session_id}/poll": map[string]any{
				"get": openAPIOperation(
					[]string{"qr"},
					"轮询扫码登录状态",
					[]map[string]any{pathStringParam("session_id", "二维码会话 ID。")},
					nil,
					defaulted(map[string]any{
						"200": jsonResponse("当前扫码状态。", refSchema("QRPollResponse")),
					}),
				),
			},
			"/qr/{session_id}/confirm": map[string]any{
				"post": openAPIOperation(
					[]string{"qr"},
					"确认已授权的扫码会话并保存账号",
					[]map[string]any{pathStringParam("session_id", "二维码会话 ID。")},
					nil,
					defaulted(map[string]any{
						"200": jsonResponse("已保存的账号信息。", refSchema("AccountPublic")),
					}),
				),
			},
			"/accounts": map[string]any{
				"get": openAPIOperation(
					[]string{"accounts"},
					"获取账号列表",
					nil,
					nil,
					defaulted(map[string]any{
						"200": jsonResponse("已保存的账号列表。", arraySchema(refSchema("AccountPublic"))),
					}),
				),
				"delete": openAPIOperation(
					[]string{"accounts"},
					"删除账号",
					[]map[string]any{queryStringParam("ref", "账号 ID、UIN 或 openid。", true)},
					nil,
					defaulted(map[string]any{
						"200": jsonResponse("删除结果。", refSchema("DeleteAccountResponse")),
					}),
				),
			},
			"/accounts/refresh": map[string]any{
				"post": openAPIOperation(
					[]string{"accounts"},
					"刷新账号存活状态",
					nil,
					jsonOptionalRequestBody(refSchema("AccountRefRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("刷新结果。未传 ref 时返回数组。", refSchema("RefreshResponse")),
					}),
				),
			},
			"/accounts/resync": map[string]any{
				"post": openAPIOperation(
					[]string{"accounts"},
					"重新同步账号资料",
					nil,
					jsonOptionalRequestBody(refSchema("AccountRefRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("同步后的账号信息。未传 ref 时返回数组。", refSchema("ResyncResponse")),
					}),
				),
			},
			"/accounts/avatar": map[string]any{
				"get": openAPIOperation(
					[]string{"accounts"},
					"获取账号头像",
					[]map[string]any{queryStringParam("ref", "账号 ID、UIN 或 openid。", true)},
					nil,
					defaulted(map[string]any{
						"200": imageResponse("头像图片。"),
						"302": map[string]any{"description": "跳转到远程头像地址。"},
					}),
				),
			},
			"/wxapp/getCode": map[string]any{
				"post": openAPIOperation(
					[]string{"wxapp"},
					"获取小程序code",
					nil,
					jsonRequestBody(refSchema("WxappRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("getCode 调用结果。", refSchema("WxappResponse")),
					}),
				),
			},
			"/wxapp/getPhoneNumber": map[string]any{
				"post": openAPIOperation(
					[]string{"wxapp"},
					"获取手机号",
					nil,
					jsonRequestBody(refSchema("WxappRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("getPhoneNumber 调用结果。", refSchema("WxappResponse")),
					}),
				),
			},
			"/wxapp/operateWxData": map[string]any{
				"post": openAPIOperation(
					[]string{"wxapp"},
					"小程序云函数",
					nil,
					jsonRequestBody(refSchema("OperateWXDataRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("operateWxData 调用结果。", refSchema("WxappResponse")),
					}),
				),
			},
			// ---- WCS 兼容接口 ----
			"/api/accounts": map[string]any{
				"get": openAPIOperation(
					[]string{"wcs"},
					"列出全部账号（与 /accounts 等价）",
					nil,
					nil,
					defaulted(map[string]any{
						"200": jsonResponse("账号列表。", arraySchema(refSchema("AccountPublic"))),
					}),
				),
			},
			"/api/accounts/add": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"通过 login_buffer 导入账号",
					nil,
					jsonRequestBody(refSchema("AccountAddRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("已导入的账号。", refSchema("AccountPublic")),
					}),
				),
			},
			"/api/accounts/delete": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"删除账号",
					nil,
					jsonRequestBody(refSchema("AccountRefRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("删除结果。", refSchema("DeleteAccountResponse")),
					}),
				),
			},
			"/api/accounts/disable": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"启用/禁用账号",
					nil,
					jsonRequestBody(refSchema("AccountDisableRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("更新后的状态。", refSchema("AccountStatusResponse")),
					}),
				),
			},
			"/api/accounts/remark": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"设置账号备注/别名",
					nil,
					jsonRequestBody(refSchema("AccountRemarkRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("更新后的别名。", refSchema("AccountStatusResponse")),
					}),
				),
			},
			"/api/accounts/rescan": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"重新校验/刷新账号登录态",
					nil,
					jsonRequestBody(refSchema("AccountRefRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("刷新结果。", refSchema("RefreshResult")),
					}),
				),
			},
			"/api/accounts/status": map[string]any{
				"get": openAPIOperation(
					[]string{"wcs"},
					"获取单个账号状态",
					[]map[string]any{queryStringParam("ref", "账号 ID、UIN 或 openid。", true)},
					nil,
					defaulted(map[string]any{
						"200": jsonResponse("账号状态。", refSchema("AccountStatusResponse")),
					}),
				),
			},
			"/api/qr/start": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"创建扫码登录会话（镜像 /qr）",
					[]map[string]any{
						boolQueryParam("as_base64", "是否同时返回二维码图片的 data URI。"),
					},
					jsonOptionalRequestBody(refSchema("QRCreateRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("二维码会话创建成功。", refSchema("QRCreateResponse")),
					}),
				),
			},
			"/api/qr/status": map[string]any{
				"get": openAPIOperation(
					[]string{"wcs"},
					"轮询扫码状态（镜像 /qr/{id}/poll）",
					[]map[string]any{queryStringParam("session_id", "二维码会话 ID。", true)},
					nil,
					defaulted(map[string]any{
						"200": jsonResponse("当前扫码状态。", refSchema("QRPollResponse")),
					}),
				),
			},
			"/api/auth/validate": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"校验调用方令牌",
					nil,
					jsonRequestBody(refSchema("AuthValidateRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("校验结果。", refSchema("AuthValidateResponse")),
					}),
				),
			},
			"/api/path": map[string]any{
				"get": openAPIOperation(
					[]string{"wcs"},
					"返回服务基准路径",
					nil,
					nil,
					defaulted(map[string]any{
						"200": jsonResponse("服务基准路径。", objectSchema(nil, map[string]any{"path": map[string]any{"type": "string"}})),
					}),
				),
			},
			"/api/proxies": map[string]any{
				"get": openAPIOperation(
					[]string{"wcs"},
					"列出全部代理",
					nil,
					nil,
					defaulted(map[string]any{
						"200": jsonResponse("代理列表。", arraySchema(refSchema("Proxy"))),
					}),
				),
			},
			"/api/proxies/add": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"新增一条代理",
					nil,
					jsonRequestBody(refSchema("ProxyAddRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("已添加的代理。", refSchema("Proxy")),
					}),
				),
			},
			"/api/proxies/delete": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"删除代理",
					nil,
					jsonRequestBody(refSchema("ProxyDeleteRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("删除结果。", refSchema("ProxyDeleteResponse")),
					}),
				),
				"delete": openAPIOperation(
					[]string{"wcs"},
					"删除代理（DELETE 语义）",
					nil,
					jsonRequestBody(refSchema("ProxyDeleteRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("删除结果。", refSchema("ProxyDeleteResponse")),
					}),
				),
			},
			"/api/proxies/test": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"测试代理连通性",
					nil,
					jsonRequestBody(refSchema("ProxyTestRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("连通性测试结果。", refSchema("ProxyTestResponse")),
					}),
				),
			},
			"/wx/code": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"获取小程序 code（镜像 /wxapp/getCode）",
					nil,
					jsonRequestBody(refSchema("WxappRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("getCode 调用结果。", refSchema("WxappResponse")),
					}),
				),
			},
			"/wx/getphonenumber": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"获取手机号（镜像 /wxapp/getPhoneNumber）",
					nil,
					jsonRequestBody(refSchema("WxappRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("getPhoneNumber 调用结果。", refSchema("WxappResponse")),
					}),
				),
			},
			"/wx/operateWxData": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"小程序云函数（镜像 /wxapp/operateWxData）",
					nil,
					jsonRequestBody(refSchema("OperateWXDataRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("operateWxData 调用结果。", refSchema("WxappResponse")),
					}),
				),
			},
			"/wx/getuserinfo": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"获取已保存账号的用户资料",
					nil,
					jsonRequestBody(refSchema("AccountRefRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("账号资料。", refSchema("WXUserInfoResponse")),
					}),
				),
			},
			"/wx/getsession": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"获取账号协议会话状态",
					nil,
					jsonRequestBody(refSchema("AccountRefRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("会话状态。", refSchema("WXSessionResponse")),
					}),
				),
			},
			"/wx/refresh": map[string]any{
				"post": openAPIOperation(
					[]string{"wcs"},
					"刷新并保存账号登录态",
					nil,
					jsonRequestBody(refSchema("AccountRefRequest")),
					defaulted(map[string]any{
						"200": jsonResponse("刷新结果。", refSchema("RefreshResult")),
					}),
				),
			},
			"/wx/{op}": map[string]any{
				"get": openAPIOperation(
					[]string{"wcs"},
					"WCS 的其它 /wx/* 操作（yyb 核心未实现）",
					[]map[string]any{pathStringParam("op", "操作名，如 oauth/qrcodeauth/cloud/gateway/translatelink 等。")},
					nil,
					defaulted(map[string]any{
						"501": jsonResponse("yyb 核心协议层未实现该操作；详见 msg 中列出的已支持接口。", refSchema("APIErrorResponse")),
					}),
				),
				"post": openAPIOperation(
					[]string{"wcs"},
					"WCS 的其它 /wx/* 操作（yyb 核心未实现）",
					[]map[string]any{pathStringParam("op", "操作名，如 oauth/qrcodeauth/cloud/gateway/translatelink 等。")},
					nil,
					defaulted(map[string]any{
						"501": jsonResponse("yyb 核心协议层未实现该操作；详见 msg 中列出的已支持接口。", refSchema("APIErrorResponse")),
					}),
				),
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"APIResponse": objectSchema([]string{"code", "msg", "data"}, map[string]any{
					"code": map[string]any{"type": "integer", "example": 0, "description": "业务状态码，0 表示成功，非 0 表示业务错误。"},
					"msg":  map[string]any{"type": "string", "example": "success", "description": "提示信息，前端可直接用于 Toast 提示。"},
					"data": nullableObjectSchema("实际数据载荷，可以是对象、数组或 null。"),
				}),
				"APIErrorResponse": objectSchema([]string{"code", "msg", "data"}, map[string]any{
					"code": map[string]any{"type": "integer", "example": 400, "description": "非 0 业务错误码。"},
					"msg":  map[string]any{"type": "string", "example": "ref is required"},
					"data": nullableObjectSchema("错误响应当前固定返回 null。"),
				}),
				"HealthResponse": objectSchema([]string{"ok"}, map[string]any{
					"ok": map[string]any{"type": "boolean"},
				}),
				"QRCreateRequest": objectSchema(nil, map[string]any{
					"use_proxy": map[string]any{"type": "boolean", "description": "是否使用品赞白名单代理完成本次扫码会话。"},
					"area":      map[string]any{"type": "string", "example": "440300", "description": "品赞优质池地区编码；全国为 all，省市为地区表中的六位编码。"},
				}),
				"PinzanRegion": objectSchema([]string{"label", "code"}, map[string]any{
					"label": map[string]any{"type": "string", "example": "深圳市"},
					"code":  map[string]any{"type": "string", "example": "440300"},
				}),
				"PinzanProvince": objectSchema([]string{"label", "code", "cities"}, map[string]any{
					"label":  map[string]any{"type": "string", "example": "广东省"},
					"code":   map[string]any{"type": "string", "example": "440000"},
					"cities": arraySchema(refSchema("PinzanRegion")),
				}),
				"PinzanRegionCatalog": objectSchema([]string{"nationwide", "provinces"}, map[string]any{
					"nationwide": refSchema("PinzanRegion"),
					"provinces":  arraySchema(refSchema("PinzanProvince")),
				}),
				"QRCreateResponse": objectSchema([]string{"session_id", "status", "image_url", "proxy_enabled", "expires_in"}, map[string]any{
					"session_id":       map[string]any{"type": "string"},
					"status":           map[string]any{"type": "string", "example": "pending"},
					"image_url":        map[string]any{"type": "string", "example": "/qr/{session_id}/image"},
					"image_base64":     nullableStringSchema("当 as_base64=true 时返回二维码图片 data URI。"),
					"proxy_enabled":    map[string]any{"type": "boolean"},
					"proxy_area":       nullableStringSchema("本次请求传给品赞的 area；全国随机时为 null。"),
					"proxy_latency_ms": map[string]any{"type": "integer", "nullable": true, "description": "代理健康检查往返延迟，单位毫秒；直连时为 null。"},
					"expires_in":       map[string]any{"type": "integer", "description": "前端二维码倒计时秒数。"},
				}),
				"QRPollResponse": objectSchema([]string{"status"}, map[string]any{
					"status": map[string]any{
						"type": "string",
						"enum": []string{"pending", "scanned", "authorized", "confirmed", "expired", "cancelled", "unknown"},
					},
					"errcode": map[string]any{"type": "integer", "nullable": true},
				}),
				"AccountPublic": objectSchema([]string{"id", "openid", "created_at", "updated_at"}, map[string]any{
					"id":              int64Schema(),
					"openid":          map[string]any{"type": "string"},
					"uin":             nullableInt64Schema(),
					"alias":           nullableStringSchema("账号别名。"),
					"nickname":        nullableStringSchema("账号昵称。"),
					"avatar":          nullableStringSchema("本地头像路径或远程头像 URL。"),
					"status":          nullableStringSchema("账号状态。"),
					"last_checked_at": nullableInt64Schema(),
					"created_at":      int64Schema(),
					"updated_at":      int64Schema(),
				}),
				"RefreshResult": objectSchema([]string{"id", "openid", "status"}, map[string]any{
					"id":       int64Schema(),
					"openid":   map[string]any{"type": "string"},
					"uin":      nullableInt64Schema(),
					"nickname": nullableStringSchema("账号昵称。"),
					"status":   map[string]any{"type": "string", "example": "alive"},
				}),
				"DeleteAccountResponse": objectSchema([]string{"deleted", "openid"}, map[string]any{
					"deleted": int64Schema(),
					"openid":  map[string]any{"type": "string"},
				}),
				"AccountRefRequest": objectSchema(nil, map[string]any{
					"ref": map[string]any{"type": "string", "description": "账号 ID、UIN 或 openid。支持批量操作的接口不传时表示全部账号。"},
				}),
				"AccountAddRequest": objectSchema([]string{"login_buffer", "openid"}, map[string]any{
					"login_buffer": map[string]any{"type": "string", "description": "账号登录态缓冲（由扫码登录获得）。"},
					"openid":       map[string]any{"type": "string", "description": "账号 openid（导入时必填）。"},
					"nickname":     nullableStringSchema("账号昵称。"),
					"alias":        nullableStringSchema("账号别名/备注。"),
				}),
				"AccountDisableRequest": objectSchema([]string{"ref"}, map[string]any{
					"ref":      map[string]any{"type": "string", "description": "账号 ID、UIN 或 openid。"},
					"disabled": map[string]any{"type": "boolean", "description": "true 禁用，false 启用（默认 true）。"},
				}),
				"AccountRemarkRequest": objectSchema([]string{"ref", "remark"}, map[string]any{
					"ref":    map[string]any{"type": "string", "description": "账号 ID、UIN 或 openid。"},
					"remark": map[string]any{"type": "string", "description": "备注/别名内容。"},
				}),
				"AccountStatusResponse": objectSchema([]string{"id", "openid", "status"}, map[string]any{
					"id":              int64Schema(),
					"openid":          map[string]any{"type": "string"},
					"uin":             nullableInt64Schema(),
					"nickname":        nullableStringSchema("账号昵称。"),
					"status":          nullableStringSchema("账号状态。"),
					"last_checked_at": nullableInt64Schema(),
					"alias":           nullableStringSchema("账号别名/备注（remark 接口返回）。"),
				}),
				"RefreshResponse": oneOfSchema(
					refSchema("RefreshResult"),
					arraySchema(refSchema("RefreshResult")),
				),
				"ResyncResponse": oneOfSchema(
					refSchema("AccountPublic"),
					arraySchema(refSchema("AccountPublic")),
				),
				"WxappRequest": objectSchema([]string{"ref", "app_id"}, map[string]any{
					"ref":    map[string]any{"type": "string", "description": "账号 ID、UIN 或 openid。"},
					"app_id": map[string]any{"type": "string"},
					"proxy":  nullableStringSchema("可选：本次调用使用的代理地址，或代理库 id（配合 ?proxy_id 亦可）。"),
				}),
				"OperateWXDataRequest": objectSchema([]string{"ref", "app_id", "payload"}, map[string]any{
					"ref":     map[string]any{"type": "string", "description": "账号 ID、UIN 或 openid。"},
					"app_id":  map[string]any{"type": "string"},
					"payload": freeFormObjectSchema("完整的 operateWxData 请求 JSON。"),
					"proxy":   nullableStringSchema("可选：本次调用使用的代理地址，或代理库 id。"),
				}),
				"WxappResponse": objectSchema([]string{"openid", "result"}, map[string]any{
					"openid": map[string]any{"type": "string"},
					"result": freeFormObjectSchema("wxapp 接口返回结果。"),
				}),
				"AuthValidateRequest": objectSchema([]string{"token"}, map[string]any{
					"token": map[string]any{"type": "string", "description": "调用方令牌；未配置 YYB_API_TOKEN 时任意非空均可。"},
				}),
				"AuthValidateResponse": objectSchema([]string{"valid"}, map[string]any{
					"valid": map[string]any{"type": "boolean"},
					"note":  nullableStringSchema("开放模式下的补充说明。"),
				}),
				"Proxy": objectSchema([]string{"id", "scheme", "host", "port", "enabled"}, map[string]any{
					"id":       int64Schema(),
					"scheme":   map[string]any{"type": "string", "example": "socks5"},
					"host":     map[string]any{"type": "string", "example": "127.0.0.1"},
					"port":     map[string]any{"type": "integer", "example": 1080},
					"username": nullableStringSchema("代理用户名（可选）。"),
					"password": nullableStringSchema("代理密码（可选）。"),
					"note":     nullableStringSchema("备注。"),
					"enabled":  map[string]any{"type": "boolean"},
					"created_at": int64Schema(),
					"updated_at": int64Schema(),
				}),
				"ProxyAddRequest": objectSchema([]string{"host", "port"}, map[string]any{
					"scheme":   map[string]any{"type": "string", "example": "socks5", "description": "socks5 或 http-connect。"},
					"host":     map[string]any{"type": "string"},
					"port":     map[string]any{"type": "integer"},
					"username": nullableStringSchema("代理用户名（可选）。"),
					"password": nullableStringSchema("代理密码（可选）。"),
					"note":     nullableStringSchema("备注。"),
					"enabled":  map[string]any{"type": "boolean", "description": "是否启用，默认 true。"},
				}),
				"ProxyDeleteRequest": objectSchema([]string{"id"}, map[string]any{
					"id": map[string]any{"type": "integer", "description": "代理 id。"},
				}),
				"ProxyDeleteResponse": objectSchema([]string{"deleted"}, map[string]any{
					"deleted": int64Schema(),
				}),
				"ProxyTestRequest": objectSchema(nil, map[string]any{
					"id":   map[string]any{"type": "integer", "description": "按代理库 id 测试。"},
					"host": map[string]any{"type": "string", "description": "直接指定 host 测试。"},
					"port": map[string]any{"type": "integer", "description": "直接指定 port 测试。"},
				}),
				"ProxyTestResponse": objectSchema([]string{"ok", "addr"}, map[string]any{
					"ok":        map[string]any{"type": "boolean"},
					"latency_ms": map[string]any{"type": "integer"},
					"addr":      map[string]any{"type": "string"},
					"error":     nullableStringSchema("连通失败时的错误描述。"),
				}),
				"WXUserInfoResponse": objectSchema([]string{"openid"}, map[string]any{
					"openid":    map[string]any{"type": "string"},
					"nickname":  nullableStringSchema("昵称。"),
					"avatar":    nullableStringSchema("头像路径或 URL。"),
					"user_info": freeFormObjectSchema("账号原始资料对象。"),
				}),
				"WXSessionResponse": objectSchema([]string{"openid", "session"}, map[string]any{
					"openid":  map[string]any{"type": "string"},
					"session": map[string]any{"type": "string", "example": "alive", "description": "alive 或 expired。"},
					"uin":     nullableInt64Schema(),
				}),
			},
		},
	}
}

func openAPIOperation(tags []string, summary string, parameters []map[string]any, requestBody map[string]any, responses map[string]any) map[string]any {
	out := map[string]any{
		"tags":      tags,
		"summary":   summary,
		"responses": responses,
	}
	if len(parameters) > 0 {
		out["parameters"] = parameters
	}
	if requestBody != nil {
		out["requestBody"] = requestBody
	}
	return out
}

func defaulted(responses map[string]any) map[string]any {
	responses["default"] = jsonErrorResponse("错误响应。")
	return responses
}

func jsonResponse(description string, schema map[string]any) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": apiResponseSchema(schema),
			},
		},
	}
}

func jsonErrorResponse(description string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": refSchema("APIErrorResponse"),
			},
		},
	}
}

func imageResponse(description string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"image/jpeg": map[string]any{
				"schema": map[string]any{"type": "string", "format": "binary"},
			},
		},
	}
}

func jsonRequestBody(schema map[string]any) map[string]any {
	return map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": schema,
			},
		},
	}
}

func jsonOptionalRequestBody(schema map[string]any) map[string]any {
	return map[string]any{
		"required": false,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": schema,
			},
		},
	}
}

func pathStringParam(name, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "path",
		"description": description,
		"required":    true,
		"schema":      map[string]any{"type": "string"},
	}
}

func queryStringParam(name, description string, required bool) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"description": description,
		"required":    required,
		"schema":      map[string]any{"type": "string"},
	}
}

func boolQueryParam(name, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"description": description,
		"required":    false,
		"schema":      map[string]any{"type": "boolean"},
	}
}

func oneOfSchema(schemas ...map[string]any) map[string]any {
	return map[string]any{"oneOf": schemas}
}

func refSchema(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func arraySchema(item map[string]any) map[string]any {
	return map[string]any{
		"type":  "array",
		"items": item,
	}
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func apiResponseSchema(dataSchema map[string]any) map[string]any {
	if dataSchema == nil {
		dataSchema = nullableObjectSchema("实际数据载荷。")
	}
	return objectSchema([]string{"code", "msg", "data"}, map[string]any{
		"code": map[string]any{"type": "integer", "example": 0, "description": "业务状态码，0 表示成功，非 0 表示业务错误。"},
		"msg":  map[string]any{"type": "string", "example": "success", "description": "提示信息，前端可直接用于 Toast 提示。"},
		"data": dataSchema,
	})
}

func freeFormObjectSchema(description string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          description,
		"additionalProperties": true,
		"nullable":             true,
	}
}

func nullableObjectSchema(description string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          description,
		"additionalProperties": true,
		"nullable":             true,
	}
}

func nullableStringSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
		"nullable":    true,
	}
}

func int64Schema() map[string]any {
	return map[string]any{"type": "integer", "format": "int64"}
}

func nullableInt64Schema() map[string]any {
	return map[string]any{"type": "integer", "format": "int64", "nullable": true}
}
