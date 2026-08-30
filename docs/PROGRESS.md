# GMS 进度追踪（PROGRESS）

| [PLAN.md](PLAN.md) 任务编号一一对应 + [FILETRACK.md](FILETRACK.md) 文件级状态。
> 规则：完成打 `[x]` 并附日期/(提交号)；进行中在行尾标 🔄；新增任务追加编号不改旧号；范围变化记入文末"变更记录"。
> **文件级追踪**：以 `docs/FILETRACK.md` 为准（533 个 Java 文件逐条标记 TODO/ACTV/DONE/MERG/SKIP），本文件只追踪阶段任务。

最后更新：2026-08-30（P2.4 完成：CHARLIST/建角/删角/SET_GENDER 全链 + live 冒烟）

## 完成度汇总

| 维度 | 进度 |
|---|---|
| Phase | 1 / 12（P1 完成；P0 剩 0.6；P2 进行中） |
| 任务 | 14 / 76（P0.1-0.5 + P1.1-1.5 + P2.1 + P2.2 + P2.3 + P2.4） |
| 文件级（FILETRACK） | DONE 21 / MERG 33 / ACTV+3 文件 |
| 脚本可用率（P7 起跟踪） | - |
| 当前里程碑 | **M1：登录闭环（P0–P2）🔄** |

状态图例：⬜ 未开始 | 🔄 进行中 | ✅ 完成 | ⛔ 被阻塞(注明原因) | ❌ 不做(注明替代)

---

## P0 基建接管 🔄

- [x] GMS-P0.1 清空 2021 旧代码，重建 go module（Go 1.25.5）✅ 2026-08-27
- [x] GMS-P0.2 目录骨架 + TOML 配置加载（对齐 ZEV.* 键，Defaults 复现原 ini 值）✅ 2026-08-27
- [x] GMS-P0.3 导出 079-max2 schema -> migrations/0001_base.sql + 表分类 A/B/C ✅ 2026-08-29（A=70 / B=113 / C=42，见 MIGRATION_MAP §12）
- [x] GMS-P0.4 opcode 生成器 ✅ 2026-08-27（实测 153 recv/225 send 唯一键；0xFF 占位值重复，双向查找表分两张）
- [x] GMS-P0.5 slog 日志基座 ✅ 2026-08-27（Makefile 未补，可用 build.ps1；cmd/gms 已接 slog）
- [ ] GMS-P0.6 联调环境固化（K: MySQL 修路径、客户端登录器连通验证）🔄（my.ini 已改 K:/ 并启动 mysqld 5.5.53；登录器连通待 V079 客户端验证）
- 冒烟：`go build ./... && go test ./...` 绿 ✅（config/crypto/netw/protocol 全 ok）

## P1 协议栈 ✅

- [x] GMS-P1.1 Shanda + AES-OFB 加解密 + golden 对拍 ✅ 2026-08-27
  - golden 59 向量（tools/goldengen 对原版 jar 生成）全对；jadx+javap 双重验证修复了 CFR 反编译失真（chunk 顺序/roll 符号）
- [x] GMS-P1.2 帧编解码 + 会话 IV 滚动 ✅ 2026-08-27（netw.Session，含 1MB 帧上限）
- [x] GMS-P1.3 PacketReader/Writer ✅ 2026-08-27（GB18030 字符串对齐 Java quirk）
- [x] GMS-P1.4 netw acceptor/session ✅ 2026-08-27（端到端回环测试绿）
- [x] GMS-P1.5 protocoltest 自测客户端 ✅ 2026-08-27（tools/protocoltest.exe 已构建）
- 冒烟：握手字节与 Java getHello 一致（login server_test 字节级校验）✅；加解密 round-trip 100% ✅（2026-08-29 protocoltest 实测 PONG→PING）

## P2 登录闭环 🔄（里程碑 M1 出口）

- [x] GMS-P2.1 database 基座 + accounts DAO ✅ 2026-08-29（internal/database：sqlx 池 + SET time_zone +08:00；accounts 查询实测通过）
- [x] GMS-P2.2 登录服：握手/版本/账密/错误码 ✅ 2026-08-29
  - LoginCrypto golden 三线（sha1/sha512/legacy）全对；发现并移植 2 个 Java quirk（见 SESSION_STATE §三）
  - LOGIN_PASSWORD(0x0001) 全链路：loginok 0/3/4/5/7 语义、密码升级 SHA-1、双登+20s transition 超时、gender=10 -> CHOOSE_GENDER、机器码/SessionIP/loggedin 落库
  - 假 store 端到端 10 测试 + 封包字节级 3 测试全绿；live MySQL 冒烟并入 P2.3 验收（V079 客户端）
- [x] GMS-P2.3 世界/频道列表封包 ✅ 2026-08-30
  - SERVERLIST(0x0009)/SERVERSTATUS(0x0006) 封包字节级 4 测试 + wire 端到端 2 测试（登录门禁/双世界排序）
  - ZEV 语义保真：LICENSE_REQUEST 也路由 ServerListRequest；NeedsChecking 门禁（未登录静默丢包）；缺频道 load=1200；世界状态 0/1/2（Load/游戏频道状态显示.txt）；原版默认无气球（容纳人数=999）
  - live 冒烟（SQLite）：-login -serverlist -status 探针全通（世界/5 频道/END_OF_LIST/NORMAL）
- [x] GMS-P2.4 选角列表/建角/删角 ✅ 2026-08-30
  - CHARLIST(0x000A) getCharList/addCharStats/addCharLook 逐字段保真（13 字节名/24 零/8 stat shorts/FT 时间戳/look 0xFF 0xFF）；CREATE_CHAR(0x0011) 默认属性+出生地图+白名单校验；CHECK_CHAR_NAME(0x000C) 名字正则（2-5 位数字/CJK）；DELETE_CHAR(0x0012) login_Auth+state 0/1/16；SET_GENDER(0x0004) GENDER_SET+license+状态复位
  - characters/character_slots DAO + SQLite 全列 DDL；`int` 列双方言引用
  - P2.4 简化（P3+ 补）：新角无 inventory/queststatus 入库（选角显示裸装）；创建上限用 charslots（原版 configvalues 创建角色数量行不在 dump）；2ndpassword 校验无 rand_r 包装
  - live 冒烟（SQLite）：-charlist 0/1 角色往返 + -hex CREATE_CHAR 建角回包/CHARLIST 回显全对
- [ ] GMS-P2.5 自动注册 + 白/黑名单
- 冒烟：V079 客户端登录到角色列表并可建角

## P3 wz 数据层 ⬜

- [ ] GMS-P3.1 v079 wz reader
- [ ] GMS-P3.2 wzdump + Java XML diff
- [ ] GMS-P3.3 数据缓存（Map/String/Item/Skill/Mob/Reactor/Npc/Quest）
- [ ] GMS-P3.4 （备选触发式）XML 兼容路线
- [ ] GMS-P3.5 wztosql 掉落表入库
- 冒烟：16 wz 全读无错；代表图数据与 Java 版一致

## P4 进入世界 ⬜ （里程碑 M2 出口）

- [ ] GMS-P4.1 channel server 骨架
- [ ] GMS-P4.2 会话迁移与进图
- [ ] GMS-P4.3 地图实例 + 视野广播
- [ ] GMS-P4.4 移动处理
- [ ] GMS-P4.5 聊天 + 关键字屏蔽
- [ ] GMS-P4.6 NPC 交互占位
- 冒烟：双客户端同图互见/聊天/掉线重连

## P5 角色与物品 ⬜

- [ ] GMS-P5.1 Character 模型与属性
- [ ] GMS-P5.2 inventory 四栏
- [ ] GMS-P5.3 item 定义 + 装备强化
- [ ] GMS-P5.4 存档/读档
- [ ] GMS-P5.5 仓库与邮件基础
- 冒烟：穿脱/移动物品/重登状态一致

## P6 战斗与技能 ⬜ （里程碑 M3 出口）

- [ ] GMS-P6.1 攻击处理 + 吸怪检测
- [ ] GMS-P6.2 伤害公式（对拍）
- [ ] GMS-P6.3 怪物 AI + 刷怪调度
- [ ] GMS-P6.4 掉落与拾取
- [ ] GMS-P6.5 buff/冷却
- [ ] GMS-P6.6 PVP 框架
- 冒烟：打怪掉落拾取入包；3 人同图无死锁

## P7 脚本系统 ⬜ （里程碑 M4 出口）

- [ ] GMS-P7.1 goja + 加载器（GBK 转码/缓存/热重载）
- [ ] GMS-P7.2 宿主 API（cm/pi/qm/em/rm）
- [ ] GMS-P7.3 java.* shim
- [ ] GMS-P7.4 scriptlint 可用率报告
- [ ] GMS-P7.5 quest 状态机
- [ ] GMS-P7.6 event 脚本框架
- 冒烟：50 个常用脚本抽检全可用；npc 可用率 ≥95%
- 脚本可用率：__%（每次更新填）

## P8 商业与社交 ⬜

- [ ] GMS-P8.1 NPC 商店
- [ ] GMS-P8.2 拍卖行（合并双实现）
- [ ] GMS-P8.3 银行 + 点券/充值卡
- [ ] GMS-P8.4 组队/好友/公会/家族
- [ ] GMS-P8.5 交易/玩家商店
- [ ] GMS-P8.6 商城 cashshop（MTS 可裁剪）
- 冒烟：买卖/交易/拍卖全流程存库正确

## P9 自定义玩法 ⬜（可裁剪，勾选范围即承诺）

- [ ] GMS-P9.1 PVP
- [ ] GMS-P9.2 副本
- [ ] GMS-P9.3 BOSS 排行（合并 10 份实现）
- [ ] GMS-P9.4 活动框架（OX/喜从天降/魔族攻城/推雪球/绝地求生）
- [ ] GMS-P9.5 段位/成就/怪物书/钓鱼/洗魔
- [ ] GMS-P9.6 反外挂 + Autoban
- 冒烟：每子系统一条端到端脚本入 CI

## P10 运营面板 ⬜

- [ ] GMS-P10.1 admin REST（在线/踢人/公告/倍率/发券）
- [ ] GMS-P10.2 充值卡管理
- [ ] GMS-P10.3 前端静态页 + token 鉴权
- [ ] GMS-P10.4 QQ webhook 桩（可选）
- 冒烟：覆盖 Java 控制台 Top10 操作；无明文密码

## P11 稳定化 ⬜ （里程碑 M5 = v1.0.0）

- [ ] GMS-P11.1 压测 500 并发 24h
- [ ] GMS-P11.2 pprof 体检
- [ ] GMS-P11.3 故障演练（DB 断连/存档中断/优雅关停）
- [ ] GMS-P11.4 部署物 + 运维文档
- 冒烟：3.1 节总 DoD 全绿，发布 tag

---

## 里程碑

| 里程碑 | 组成 | 出口判据 | 状态 |
|---|---|---|---|
| M1 可登录 | P0–P2 | 客户端到角色列表、建角入库 | 🔄 |
| M2 可进世界 | P3–P4 | 双客户端同图互动 | ⬜ |
| M3 可玩核心 | P5–P6 | 打怪掉落、重登无损 | ⬜ |
| M4 脚本生态 | P7 | 脚本可用率 ≥95% | ⬜ |
| M5 可运营 | P8–P11 | 面板+压测达标，v1.0.0 | ⬜ |

## 变更记录

- 2026-08-27 v1.0 方案制定。基线：Zevms 反编译源码 + 交流源码对照 + K: 发布包联调；发现并纳入 I:\GMS 2021 年遗留 Go 尝试（P0.1 清理）。
- 2026-08-27 P0.1/0.2/0.4/0.5 + P1 全部代码完成：加密 golden 100%（59 向量）、netw/protocol/config 测试全绿、login 冒烟服编译过。**方法论确立：先 golden 后实现；jadx(D:\Soft\jadx)+javap 双验反编译争议**。详细状态与断点见 SESSION_STATE.md。
- 2026-08-27 会话中断，进度固化至 SESSION_STATE.md（断点A：login server_test.go 已写未跑）。
- 2026-08-29 恢复现场：跑通 login 冒烟测试并修复三处——①getHello 在 Java 中是 client 属性未建立前写入，应走原始 15 字节（无帧头/不加密）；②Acceptor 补调 OnOpen；③测试 readN 精确读不吞包；protocoltest 同步修正。live 验证：`bin/gms.exe configs/gms.toml` + `tools/protocoltest.exe` 输出 hello version=79、PONG→PING opcode 0x0014。
- 2026-08-29 P0.3/P0.6/P2.1 推进：修复 my.ini 路径并启动 K: MySQL 5.5.53，`mysqldump --no-data 079-max2` 生成 migrations/0001_base.sql（225 表，A/B/C=70/113/42）；新增 internal/database（go-sql-driver/mysql + sqlx）与 accounts DAO，GMS_TEST_DB_DSN 实连 079-max2 验证通过。
