# Clippy

macOS 剪切板管理工具。Go 后端 + Swift 前端 + WebView UI，菜单栏常驻，全局快捷键呼出。

## 功能

- **自动捕获** — Go 后端 500ms 轮询系统剪切板，自动去重
- **搜索过滤** — 实时关键词搜索，200ms 防抖
- **固定条目** — Pin 后置顶，不受自动清理影响
- **类型识别** — 自动识别 Go / JS / Python / SQL / HTML / Shell / URL / JSON
- **键盘导航** — `↑` `↓` 选择，`Enter` 粘贴，`Esc` 关闭
- **全局快捷键** — `⌘ + Shift + V` 随时呼出面板
- **隐私保护** — 点击面板外部自动关闭，暂停记录模式
- **导出** — 支持导出为 JSON / CSV

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.21+ |
| 数据库 | SQLite |
| 前端 | Swift + WKWebView |
| UI | HTML / CSS (液态玻璃风格) |
| 构建 | Swift Package Manager |

## 架构

```
Clippy.app
├── Clippy (Swift)          ← 菜单栏 + WebView + 进程管理
│   └── clippy-backend (Go) ← 剪切板监听 + SQLite + HTTP API
│       └── index.html      ← UI (WKWebView 渲染)
```

Swift 启动时 fork Go 进程，Go 暴露 `localhost:5100` API，WebView 通过 API 交互。退出时 Swift 自动清理 Go 进程。

## 安装

```bash
git clone https://github.com/j1angyuxuan811-lab/clippy-v2.git
cd clippy-v2
bash build.sh
open build/Clippy.app

# 或安装到 Applications
cp -r build/Clippy.app /Applications/
```

### 系统要求

- macOS 12+
- 首次运行需开启辅助功能权限：**系统设置 → 隐私与安全性 → 辅助功能 → 添加 Clippy**

## API

后端运行在 `http://localhost:5100`

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/clips` | 获取所有剪切板记录 |
| PUT | `/api/clips/{id}/pin` | 切换固定状态 |
| DELETE | `/api/clips/{id}` | 删除条目 |
| GET | `/api/health` | 健康检查 |

## 项目结构

```
clippy-v2/
├── build.sh                        # 一键构建
├── go-backend/                     # Go 后端
│   ├── main.go                     # 入口 + 信号处理
│   └── internal/
│       ├── clipboard/monitor.go    # 剪切板轮询
│       ├── db/store.go             # SQLite CRUD
│       └── api/server.go           # HTTP API
├── swift-frontend/                 # Swift 前端
│   ├── Package.swift
│   └── Sources/ClippyApp.swift     # 菜单栏 + WebView
└── ui-prototype/
    └── index.html                  # Web UI
```

## License

MIT
