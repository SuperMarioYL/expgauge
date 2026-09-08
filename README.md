[English](./README.en.md) | **简体中文**

<div align="center">

<img src="https://readme-typing-svg.demolab.com?font=Fira+Code&weight=600&size=28&duration=3000&pause=1000&color=7D3CFF&center=true&vCenter=true&width=600&height=100&lines=expgauge;run-level+exposure+gauge;for+coding+agents" alt="expgauge" />

<br/>

<a href="https://github.com/SuperMarioYL/expgauge"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go" /></a>
<a href="https://github.com/SuperMarioYL/expgauge/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-7D3CFF?style=flat-square" alt="License" /></a>
<a href="https://github.com/SuperMarioYL/expgauge"><img src="https://img.shields.io/github/stars/SuperMarioYL/expgauge?style=flat-square&color=7D3CFF&logo=github" alt="Stars" /></a>
<a href="https://github.com/SuperMarioYL/expgauge/releases"><img src="https://img.shields.io/badge/version-0.1.0-7D3CFF?style=flat-square" alt="Version" /></a>
<a href="https://gitee.com/SuperMarioYL/expgauge"><img src="https://img.shields.io/badge/Gitee-mirror-C71D23?style=flat-square&logo=gitee&logoColor=white" alt="Gitee" /></a>

<br/>
<br/>

**你的 Qwen agent 已经跑了 6 小时，做了 47 次文件修改，烧了 230 万 token——自从你上次查看以来。**

expgauge 是一个面向本地模型 coding agent 的「无人值守暴露计」。它追踪累计暴露量（运行时长、状态变更、token 消耗），在越过阈值时强制暂停，等待人工审查后才放行。

免费 · 开源 · MIT 协议

</div>

---

## 目录

- [为什么需要 expgauge](#为什么需要-expgauge)
- [快速开始](#快速开始)
- [命令一览](#命令一览)
- [实时演示](#实时演示)
- [配置](#配置)
- [架构](#架构)
- [路线图](#路线图)

---

## 为什么需要 expgauge

在 Qwen3、DeepSeek 等本地模型上跑 coding agent 的开发者越来越多。社区里「跑了 8 小时不间断」已经是被庆祝的能力——但唯一的「安全保障」是你自己的信任。没有仪表盘告诉你 agent 自上次审查以来运行了多久、改了多少文件、烧了多少 token。

DeepSeek-Reasonix（35k stars）把「挂机运行」当卖点，却没有提供运行级别的安全遥测。「I Have Been Clawed」社区事故索引证明无人值守的 agent 确实会出事——但那是事后归档，不是事前拦截。

expgauge 填补这个空白：

| 维度 | 追踪内容 | 默认阈值 |
|------|---------|---------|
| 运行时长 | 自上次人工审查以来的 wall-clock 时间 | 2 小时 |
| 状态变更 | 文件写入 / 创建 / 删除事件（fsnotify） | 50 次 |
| Token 消耗 | 从 agent 日志解析的 token 用量 | 100 万 |

任何一个维度越过阈值，终端暂停，弹出审查提示——你必须手动确认（y/n）才能让 agent 继续。

这不是 per-request 可观测性，不是 per-action 风险评分，而是 **运行级别的累积暴露追踪 + 强制审查检查点**。

---

## 快速开始

### 安装

```bash
go install github.com/SuperMarioYL/expgauge@latest
```

或从源码构建：

```bash
git clone https://github.com/SuperMarioYL/expgauge.git
cd expgauge
go build -o expgauge ./cmd/expgauge
```

### 首次运行

```bash
# 监控当前目录的文件变更，追踪 wall-clock 暴露
expgauge watch --paths .

# 指定 agent PID + 日志路径，同时追踪 token 消耗
expgauge watch --pid $(pgrep -f coding-agent) --log /tmp/agent.log --paths ./src

# 查看当前运行状态
expgauge status

# 标记一次人工审查，重置 wall-clock 计时
expgauge review

# 导出运行时间线为 JSONL
expgauge export -o timeline.jsonl
```

无需配置文件也能运行——内置默认阈值（2h / 50 次 / 1M token）。自定义阈值见 [配置](#配置) 章节。

---

## 命令一览

| 命令 | 说明 |
|------|------|
| `expgauge watch` | 启动监控，在终端显示实时暴露徽章；越过阈值时暂停 |
| `expgauge status` | 显示当前运行的累计暴露量 |
| `expgauge review` | 标记人工审查检查点，重置 wall-clock 计时 |
| `expgauge export -o file.jsonl` | 将运行时间线导出为 JSONL 格式 |

`watch` 命令支持的 flag：

| Flag | 说明 |
|------|------|
| `--pid, -p` | agent 进程 PID（覆盖配置文件） |
| `--paths, -w` | 监控变更的目录列表 |
| `--log, -l` | agent 日志文件路径（用于 token 解析） |

---

## 实时演示

`expgauge watch` 启动后，终端显示实时暴露徽章：

```
 expgauge  exposure badge

  status: OK

  wall 0m05s  ██░░░░░░░░░░░░░░░░░░  0%
  mut  3       ██████░░░░░░░░░░░░░░  6%
  tok  12.5K   █░░░░░░░░░░░░░░░░░░░  1%

  [q] quit
```

当任一阈值被越过时，终端暂停并弹出审查提示：

```
 ⚠ THRESHOLD EXCEEDED

The agent has crossed an exposure threshold:

  wall-clock:  2h01m
  mutations:   52
  tokens:      1.2M

  Review the agent's work. Continue? [y/n]
```

输入 `y` 继续（重置 wall-clock），输入 `n` 退出监控。

---

## 配置

配置文件位于 `~/.expgauge.yaml`，所有字段均可选——缺失字段使用内置默认值：

```yaml
# 监控目录列表（文件写入/创建/删除会被计为状态变更）
watch_paths:
  - /home/user/my-project

thresholds:
  wall_clock_sec: 7200   # 2 小时
  mutations: 50           # 50 次文件变更
  tokens: 1000000         # 100 万 token

agent:
  pid_file: /tmp/agent.pid         # 从 PID 文件读取 agent PID
  log_path: /tmp/agent.log        # 从日志文件解析 token 用量
  name_pattern: "qwen|deepseek|glm" # 用 pgrep -f 按进程名搜索
```

`expgauge watch` 会按以下顺序定位 agent 进程：`--pid` flag > `agent.pid_file` > `agent.name_pattern`。如果都未配置，仅追踪文件变更（不追踪 token）。

---

## 架构

```
expgauge
├── cmd/expgauge/          # cobra CLI 入口（watch / status / review / export）
├── internal/
│   ├── config/            # ~/.expgauge.yaml 解析 + 默认值
│   ├── watcher/           # agent 进程检测（PID 文件 / pgrep / 显式 PID）
│   ├── metrics/           # 暴露量累加器（wall-clock + fsnotify + 日志 tail）
│   ├── threshold/         # 阈值评估 + SIGTSTP 强制暂停
│   ├── ui/                 # bubbletea/lipgloss 实时徽章 TUI
│   └── export/            # JSONL 时间线导出
```

数据流：

1. `watch` 加载配置 → 定位 agent 进程（可选）→ 创建累加器（fsnotify + 日志 tail）
2. 累加器在后台轮询文件事件和日志尾部，更新 wall-clock / 变更数 / token 数
3. 每秒刷新 TUI 徽章，同时评估阈值
4. 阈值越过 → 发送 SIGTSTP 给 agent 进程 + 弹出审查提示
5. 每分钟持久化运行状态到 `~/.expgauge/run.json`，追加快照到 `~/.expgauge/timeline.jsonl`
6. `status` / `review` / `export` 读写状态文件和时间线文件

---

## 路线图

| 版本 | 目标 | 状态 |
|------|------|------|
| v0.1 | 核心：CLI watch + status + 阈值暂停 + JSONL 导出 | ✅ 已实现 |
| v0.2 | 配置文件增强、项目级阈值、Web 仪表盘 | 计划中 |
| v0.3 | 多 agent 编排、云端指标托管 | 计划中 |

---

<div align="center">

MIT © 2026 SuperMarioYL

</div>
