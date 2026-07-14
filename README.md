# keyword-logger

Linux 后台键盘敲击记录工具。通过 X11 Record 扩展监听全局键盘事件，按应用、按键、时间三个维度统计敲击次数，并提供 Web 仪表盘和 HTTP API。

## 功能

- 全局键盘事件监听（X11）
- 按应用分组统计敲击次数
- 虚拟键盘热力图可视化
- 时间范围筛选（支持 `since` / `until`）
- 自定义时间粒度（hour / day / week / month / year）
- 数据持久化到本地 JSON 文件
- HTTP API 和 Web 仪表盘

## 要求

- Linux + X11（Wayland 不支持）
- Go 1.25（仅编译时需要）

## 快速开始

```bash
# 编译
go build -o keyword-logger ./ 

# 启动
./keyword-logger
```

默认监听 `http://127.0.0.1:5700`，数据存储在 `~/.local/share/keyword-logger/stats.json`。

### 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--port` | `5700` | HTTP 服务端口 |
| `--store` | `~/.local/share/keyword-logger/stats.json` | 数据文件路径 |

示例：

```bash
./keyword-logger --port 9090 --store /tmp/stats.json
```

## API

| 端点 | 说明 |
|------|------|
| `GET /` | Web 仪表盘 |
| `GET /stats` | 完整统计数据，支持 `?app=` `?since=` `?until=` `?granularity=` |
| `GET /health` | 健康检查 |
| `GET /summary` | 简洁文本摘要 |

### 示例

```bash
# 查看所有应用的统计
curl http://127.0.0.1:5700/stats

# 按应用筛选
curl "http://127.0.0.1:5700/stats?app=code"

# 按时间范围筛选
curl "http://127.0.0.1:5700/stats?since=2026-07-06T10:00&until=2026-07-06T11:00"

# 按周聚合
curl "http://127.0.0.1:5700/stats?granularity=week"
```

## 架构

```
X11 Key Events → Recorder → Counter（内存，分钟桶）→ Persister（JSON 落盘）
                                                      │
                                                    HTTP API
                                                    (:5700)
```

数据模型：`map[appName][minuteBucket][keyName] → count`，每分钟一个桶，查询时按需聚合。

## 项目结构

```
├── main.go                  # 入口：初始化各模块并启动服务
├── internal/
│   ├── counter/             # 内存计数器，管理按键/应用/时间桶
│   ├── recorder/            # X11 键盘事件监听
│   ├── persist/             # JSON 定时落盘（原子写入）
│   ├── window/              # 前台应用窗口追踪
│   └── api/                 # HTTP 服务 + Web 仪表盘模板
└── keyword-logger           # 编译产物
```

## License

MIT
