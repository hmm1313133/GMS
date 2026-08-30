# GMS - ZEVMS(079MAX2) 冒险岛服务端 Go 重构

把 `I:\Zevms`（079MAX2 / ZEVMS 系 Java 源码，Netty + MySQL + Rhino JS）重构为纯 Go 实现。

> 本目录内 2021 年遗留的旧 Go 尝试（gnet echo）将于 P0 清理，仓库以 docs/ 为起点重建。

## 文档导航

| 文档 | 用途 |
|---|---|
| [docs/PLAN.md](docs/PLAN.md) | **主方案**：现状盘点、目标架构、技术决策 J1–J8、P0–P11 阶段计划、风险、估算 |
| [docs/SESSION_STATE.md](docs/SESSION_STATE.md) | **会话交接（跨会话先读）**：断点位置、已完成包、踩坑结论、下一步优先级 |
| [docs/PROGRESS.md](docs/PROGRESS.md) | **进度追踪**：里程碑 M1–M5、任务 GMS-Px.y 勾选、冒烟清单、变更记录 |
| [docs/FILETRACK.md](docs/FILETRACK.md) | **文件级追踪**：533 个 Java 文件逐条状态（TODO/ACTV/DONE/MERG/SKIP），由 `tools/gen_filetrack.ps1` 生成基线 |
| [docs/MIGRATION_MAP.md](docs/MIGRATION_MAP.md) | **代码映射**：Java -> Go 逐条状态（⬜/🔄/✅/❌）、DB 表分类 |

## 参考材料（只读）

- `I:\Zevms` - 主参考源码（079MAX2 反编译版，功能最全）
- `I:\ZEVMS079交流源码` - 原始可读源码（恢复反编译失真处的语义）
- `K:\079MAX2服务端` - 完整发布包：wz 数据、3922 个脚本、带数据 MySQL、V079 客户端登录器（联调环境）

## 阶段总览

P0 基建 -> P1 协议栈 -> P2 登录闭环(M1) -> P3 wz 数据 -> P4 进入世界(M2) -> P5 角色与物品 ->
P6 战斗与技能(M3) -> P7 脚本系统(M4) -> P8 商业社交 -> P9 自定义玩法 -> P10 运营面板 -> P11 稳定化(M5/v1.0.0)

估算：单人全职 18–25 周（P9 裁剪后 16–22 周）。

当前状态：**P0 待启动** - 进度详见 [docs/PROGRESS.md](docs/PROGRESS.md)。
