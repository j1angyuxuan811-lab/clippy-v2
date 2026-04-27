# Clippy v2

macOS 剪切板管理器。Go 后端 + Swift 菜单栏 + 液态玻璃 UI。

![Clippy UI](screenshots/main-ui.png)

## 特性

- **自动捕获** — 500ms 轮询系统剪切板，自动去重
- **液态玻璃 UI** — macOS Tahoe 设计语言，SVG 图标，毛玻璃效果
- **全局快捷键** — `⌘+Shift+V` 随时呼出/关闭面板
- **键盘导航** — ↑↓ 选择，Enter 复制，Delete 删除
- **智能识别** — 自动识别 Go/JS/Python/SQL/HTML/Shell/URL/JSON 等类型
- **实时搜索** — 200ms 防抖，即搜即得
- **固定功能** — 重要条目固定到顶部，不受自动清理影响
- **点击外部关闭** — 自动检测外部点击，优雅隐藏
- **SQLite 存储** — 最多 1000 条，自动清理旧记录
- **菜单栏常驻** — 点击图标快速访问，无需切换应用

## 架构

```
┌─────────────────┐    HTTP API     ┌─────────────────┐
│  Swift 前端     │◄──────────────►│  Go 后端        │
│  (Menu Bar)     │  :5100         │  (Clipboard)    │
│  • 菜单栏图标   │                │  • 剪切板监听   │
│  • 面板显示     │                │  • SQLite 存储  │
│  • 进程管理     │                │  • REST API     │
│  • 全局热键     │                │  • 数据导出     │
└─────────────────┘                └─────────────────┘
```

**技术栈：**
- **后端** — Go + SQLite3 + Gorilla Mux
- **前端** — Swift + WKWebView + HTML/CSS/JS
- **通信** — HTTP API (localhost:5100)
- **打包** — macOS .app bundle

## 安装

```bash
# 克隆
git clone https://github.com/j1angyuxuan811-lab/clippy-v2.git
cd clippy-v2

# 构建
bash build.sh

# 运行
open build/Clippy.app

# 或安装到 Applications
cp -r build/Clippy.app /Applications/
```

**首次运行**需要开启辅助功能权限：
**系统设置 → 隐私与安全性 → 辅助功能 → 添加 Clippy**

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/clips` | 获取剪切板列表 |
| POST | `/api/clips` | 添加新条目 |
| DELETE | `/api/clips/:id` | 删除条目 |
| PUT | `/api/clips/:id/pin` | 固定/取消固定 |
| GET | `/api/export` | 导出 JSON |
| GET | `/api/health` | 健康检查 |

## 开发

```bash
# Go 后端
cd go-backend
go run cmd/server/main.go -addr :5100 -db ./clippy.db -static ../ui-prototype

# Swift 前端
cd swift-frontend
swift build
swift run

# API 测试
curl http://localhost:5100/api/health
curl http://localhost:5100/api/clips
```

## License

MIT
