# keyword-logger — Linux 键盘敲击记录工具

## 概述

后台守护进程，通过 X11 Record 扩展监听全局键盘事件，按键盘键、应用和时间（分钟粒度）三个维度统计敲击次数，通过 HTTP API 提供查询，数据存储在内存中并定时落盘到 JSON 文件。

## 技术栈

- **语言**: Go 1.25
- **平台**: Linux (X11)
- **X11 库**: `github.com/jezek/xgb`（Go X11 客户端绑定）
- **HTTP**: Go 标准库 `net/http`
- **存储**: 内存 + JSON 定时落盘

## 架构

```
X11 Key Events ──→ Recorder ──→ Counter (内存, 分钟桶) ──→ Persister (JSON 落盘)
                                          │
                                          └──→ HTTP API (:5700)
```

## 数据模型

```
Counter: map[appName][minuteBucket][keyName] → count
```

- `minuteBucket`: 格式 `2006-01-02T15:04`（如 `2026-07-06T10:30`）
- 查询时按需聚合跨分钟桶的总数

## HTTP API

| 端点 | 说明 |
|------|------|
| `GET /stats` | 完整统计数据（可加 `?app=&since=&until=`） |
| `GET /stats?app=code` | 按应用筛选 |
| `GET /stats?since=2026-07-06T10:00` | 按起始时间筛选 |
| `GET /stats?until=2026-07-06T11:00` | 按结束时间筛选 |
| `GET /health` | 健康检查 |
| `GET /summary` | 简洁文本摘要 |

## CLI

```
keyword-logger              # 前台启动 daemon
keyword-logger --port 9090  # 自定义端口
keyword-logger --store /path/to/data.json  # 自定义数据路径
```

## 存储

- 默认路径: `~/.local/share/keyword-logger/stats.json`
- 间隔: 每 10 秒
- 策略: 写入临时文件 → os.Rename 原子替换
- 启动时恢复历史数据
