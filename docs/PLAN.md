# GMS 重构总方案（PLAN）

> 把 ZEVMS/079MAX2（Java, Netty, MySQL, Rhino JS, Swing）重构为纯 Go 实现。
> 本文档是唯一事实来源；进度勾选在 [PROGRESS.md](PROGRESS.md)，代码映射在 [MIGRATION_MAP.md](MIGRATION_MAP.md)。

版本：v1.0（2026-08-27 制定）
维护规则：任务只增不改号；范围变更走"变更记录"追加；每完成一项同步 PROGRESS.md 与 MIGRATION_MAP.md 状态。

---

## 1. 背景与输入材料

| 材料 | 角色 | 用法 |
|---|---|---|
| `I:\Zevms` | 079MAX2 反编译源码（aVer.26，功能最全） | 功能范围与协议行为的**主参考** |
| `I:\ZEVMS079交流源码` | 原始可读源码（aVer.5，GBK 中文注释） | 反编译失真处的**语义恢复**（对照同名文件） |
| `K:\079MAX2服务端` | 完整发布包（jar/wz/scripts/带数据 MySQL/客户端登录器） | **联调环境**、wz 与脚本数据源、验收基准 |
| `I:\GMS`（现存） | 2021 年 Go 尝试：gnet echo + 登录握手 hello | P0 评估接管或清除，历史产物不直接采信 |

## 2. 现状量化盘点（2026-08-27 实测）

- Java 源码：533 文件 / ~122,000 行
  - 其中 Swing GUI（`gui.*`、QQ 机器人、控制台）~46,000 行 → **不迁移**，由 Web 管理面板替代
  - 核心业务 ~76,000 行：scripting ~25k、server ~27k（含 custom 7k）、client ~23k、handling ~21k、tools ~12k、provider ~1.5k
- 协议面：**155 个 recv opcode / 245 个 send opcode**；分发器 `MapleServerHandler` 1,298 行；`MaplePacketCreator` 5,826 行 + tools/packet 10 文件
- 网络：Netty 4.0（登录 8484 / 频道 7575 / 商城独立端口）；加密 = MapleAESOFB(AES-128-OFB, 双向 IV) + MapleCustomEncryption(按字节变换)
- 数据库：MySQL，库 `079-MAX2`，**225 张表**（Odin 原生 + ZEVMS 自定义中文表 ~44 张：拍卖/锻造/钓鱼/白名单/指令记录等），root/root 硬编码
- 脚本：**3,922 个 JS**（npc 1159、npc1 1337、portal 801、quest 487、event 124、pnpc 14），Rhino 引擎 + 宿主对象（`cm`/`npc`/`pi`/`em` 等）
  - 抽样 400 个：`importClass` 0 次；`java.*` 引用 70 个（17.5%，需 shim）
  - 最大宿主面：`NPCConversationManager` 9,729 行、`AbstractPlayerInteraction` 5,877 行、`EventInstanceManager` 1,079 行
- wz 数据：K: 下 16 个二进制 wz（39,986 文件）；Java 侧实际读 **XML 化 wz**（WzXML provider，`net.sf.odinms.wzpath`）
- 既有 Go 尝试：go.mod(Go 1.16) + gnet v1 + zap；仅实现握手 hello 包

## 3. 目标与范围

### 3.1 总目标（最终 DoD）

1. V079 客户端（K: 的登录器）可完成：登录 → 建号 → 建角 → 进图 → 移动/聊天 → 打怪掉落拾取 → 商店买卖 → 下线重登数据完整。
2. 常用 NPC/传送门脚本可用率 ≥ 95%（以 npc 主目录 1159 个计）。
3. 单机 500 并发在线稳定 24h（压测脚本模拟登录+移动+攻击）。
4. 064+21+66 等 ZEVMS 核心自定义玩法（PVP/拍卖/银行/点券）按 P9 裁剪清单落地。
5. 全部配置外置（TOML），无硬编码账密；日志结构化；提供 Web 管理面板。

### 3.2 明确不迁移（Go 侧替代方案）

| 原件 | 行数 | Go 替代 |
|---|---|---|
| Swing 控制台/活动控制台/各 GUI（gui.* 8 个子包） | ~45k | Web 管理面板（P10）+ REST API |
| QQ 机器人（酷Q/KuQ、QQMsgServer） | ~3.4k | Webhook/适配器桩（P10，按需） |
| 自动更新器/下载器/解压器（download/gui.Jieya） | ~2.5k | 不迁移（部署用二进制+脚本） |
| 内存释放器/MP3 提示音等运营杂物 | - | 不迁移 |
| `Class1/2/3`、测试 demo、`JComboBoxDemo1` 等散件 | ~1k | 不迁移 |
| 反编译备份副本（`database1`、`bossrank1..10` 重复系） | ~4k | Go 版抽象为单实现+配置实例 |

### 3.3 交付物

Go 单 module（`GMS`），`cmd/gms` 单二进制多角色（login/channel/cashshop/world 由配置启用），
`migrations/` SQL、`configs/` TOML、`tools/` 辅助（wz dump、卡号导入、压测机器人）、`docs/` 三文档。

## 4. 目标架构

### 4.1 目录结构

```
I:\GMS
├── cmd/gms/                 # 入口：装配 config→db→wz→servers
├── internal/
│   ├── config/              # TOML 加载（映射 ZEV.* ini 键 + ServerProperties）
│   ├── database/            # GORM 连接池、DAO 基座（断点 I 起，mysql+纯Go sqlite 双方言）
│   ├── netw/                # TCP acceptor、session、帧 codec（加解密）
│   ├── crypto/              # MapleAESOFB + Shanda 自定义变换（J1）
│   ├── protocol/            # opcode 表(155/245)、PacketReader/Writer、封包结构体
│   ├── packet/              # MaplePacketCreator 的 Go 版（send 封包构造器，按域拆文件）
│   ├── login/               # 登录服：握手、账密、选角、建号、PIC
│   ├── world/               # 跨频道：公会/好友/组队/广播、世界定时器
│   ├── channel/             # 频道服：player、map、movement、chat、trade、handlers/
│   ├── cashshop/            # 商城/MTS
│   ├── model/               # Character/Inventory/Item/Skill/Quest/Buff 领域模型
│   ├── mapp/                # 地图引擎：地图实例、生命体、反应堆、传送门、刷怪调度
│   ├── combat/              # 伤害计算、技能效果、buff 生命周期
│   ├── life/                # 怪物/ NPC 工厂与 AI
│   ├── script/              # goja 引擎 + 宿主 API（cm/npc/pi/em/qm）+ java.* shim
│   ├── wzs/                 # v079 二进制 wz 读取器 + 数据缓存（J3）
│   ├── custom/              # ZEVMS 自定义玩法（拍卖/银行/PVP/排行/充值）
│   ├── anticheat/           # 吸怪检测/全屏检测/关键字屏蔽等
│   └── admin/               # REST 管理面板后端
├── web/                     # 管理面板前端（轻量，可选静态页）
├── tools/                   # wzdump / scriptlint / import_cards / bot 压测
├── migrations/              # schema 演进（自 079-max2 快照起步）
├── configs/gms.toml
└── docs/                    # 本三件套
```

### 4.2 技术选型

| 域 | 选择 | 理由（对照 Java 原件） |
|---|---|---|
| 语言/运行时 | Go 1.22+ | 静态单二进制，部署替代 jdk 捆绑 |
| 网络 | 标准库 `net`，goroutine/conn | Netty 4 语义直接映射；gnet 不必要（旧尝试弃用，见 J8） |
| 加解密 | 自实现 `internal/crypto` | MapleAESOFB+Shanda 必须逐字节对齐（J1） |
| DB | `gorm.io/gorm` + `glebarez/sqlite`（纯 Go）+ `gorm.io/driver/mysql` | 替代 JDBC 裸 SQL；沿用 079-max2 schema 起步（J4；断点 I 从 sqlx 迁来） |
| 脚本 | `github.com/dop251/goja` | 纯 Go ES 引擎替代 Rhino；宿主 API 由 Go 注入（J2） |
| wz | 自研 v079 读取器 `internal/wzs` | 免 Java/XML 依赖链；备选 XML 路线（J3） |
| 配置 | `spf13/viper`（TOML） | 替代 ServerProperties+ini 双轨 |
| 日志 | `log/slog`（JSON） | 替代 FilePrinter/FileoutputUtil 落盘惯例 |
| 定时 | `time.Ticker` + 调度器接口 | 替代 `server.Timer` 的 ScheduledExecutor |
| Web 面板 | `net/http` + 静态页 | 替代 Swing 控制台（P10） |
| 随机 | 可注入 `RNG` 接口（默认 math/rand/v2） | 对齐 Java `Random` 行为做测试（J6） |

### 4.3 并发模型

- 一连接一 goroutine（读循环）+ 写 channel（单写 goroutine，顺序发送对齐 Netty 语义）。
- 地图为并发单元：`mapp.Instance` 内互斥锁保护生命体/玩家集合；跨玩家消息经 map 事件队列串行化（避免 Java 版散锁死锁面）。
- 全局单例（World/Guild/Storage 工厂）用 `sync.RWMutex` + 快照读。
- 关停：context 树 + 存档 flush（对齐 ShutdownServer）。

### 4.4 包依赖方向（只允许向下）

`cmd → login/channel/cashshop/world → model/mapp/combat/script → protocol/packet/netw/crypto/database/wzs/config`

## 5. 关键技术决策

| # | 决策 | 内容 | 验证方式 |
|---|---|---|---|
| J1 | 加密移植 | 逐行移植 MapleAESOFB（AES-128-OFB，IV 滚动）与 MapleCustomEncryption（Shanda 双向变换）；封包头 4 字节（长度^0xFFFF+循环） | 用 Java 版生成随机包密文 → Go 解密 golden 单测（P1） |
| J2 | 脚本引擎 | goja + 宿主对象按"接口面"实现（cm/pi/em/qm/npc）；`java.*` 17.5% 引用做 shim（Math/String/Integer 常用子集）；工具 `scriptlint` 全量扫描暴露缺口 | 每 phase 跑脚本可用率统计，P7 达 ≥95% |
| J3 | wz 读取 | 主线：自研 v079 二进制 reader（header/目录/uol/canvas/vector/img；079 无复杂加密）；备选：外部工具转 XML 后 Go 读 XML（WzXML 兼容） | `tools/wzdump` 与 Java 侧 XML 输出 diff 对齐（P3） |
| J4 | DB 策略 | P0 导出 079-max2 schema 快照；表分三类：A 原生（accounts/characters/inventory…）、B 自定义（auctionitems…44 张）、C 废弃（重复系）；Go DAO 只实现 A+B，C 归档 | 快照入 migrations/0001_base.sql；DAO 覆盖清单核对（P2 起） |
| J5 | opcode 表 | 从 `recv/send.properties`（155/245）生成 Go 常量 + 名称映射（生成器脚本入 tools/）；handler 注册表模式替代 1298 行 switch | 生成物单测 + 双向查表一致（P1） |
| J6 | 数值/随机对齐 | 伤害公式、Randomizer 换可注入 RNG；测试模式固定种子与 Java 输出比对 | 单测固定种子对拍（P6） |
| J7 | 编码统一 | 源资产（脚本 GBK、DB latin1 惯例）入库前统一转 UTF-8；脚本加载时按 BOM/探测转码 | scriptlint 检出乱码脚本清单（P7） |
| J8 | 旧 Go 尝试处置 | 现存 gnet/zap 代码仅保留 `getHello` 语义参考，P0 清空重写；选标准库 net（少一层抽象、codec 需要会话级 IV 状态，gnet 收益为零） | P0 验收时仓库干净（P0） |

## 6. 阶段计划

> 编号规则：`GMS-Px.y` 全局唯一。状态只记于 PROGRESS.md。
> 每阶段出口 = DoD 全绿 + 冒烟通过 + 文档三件套同步。

### P0 基建接管（0.5 周）

目标：干净仓库、可构建、联调环境就位。
- GMS-P0.1 清空 2021 旧代码（保留 docs），重建 module（go 1.22，标准库为主）
- GMS-P0.2 目录骨架 + config 加载（TOML：端口/倍率/DB，对齐 `服务端配置加载项.ini` 的 ZEV.* 键）
- GMS-P0.3 从 K: 导出 079-max2 schema（mysqldump 结构）→ `migrations/0001_base.sql`；表分类清单 A/B/C 落 MIGRATION_MAP
- GMS-P0.4 opcode 生成器：recv/send.properties → `internal/protocol/opcode_gen.go`
- GMS-P0.5 slog 日志基座 + Makefile（build/test/lint/vet）
- GMS-P0.6 联调环境固化：K: MySQL 启动方式（修 my.ini 路径）写入 docs；V079 客户端登录器可连（空跑）
DoD：`go build ./... && go test ./...` 绿；config 能读示例 TOML；opcode 155/245 全量生成。

### P1 协议栈（1–1.5 周）

目标：不依赖业务的字节级协议层完备。
- GMS-P1.1 crypto：Shanda 变换 + AES-OFB（J1）+ golden 对拍（Java 侧预制向量）
- GMS-P1.2 帧编解码：4 字节头解析、粘包、会话级 sendIV/recvIV 滚动
- GMS-P1.3 PacketReader/Writer（对齐 tools/data 的 LittleEndianAccessor 语义）
- GMS-P1.4 netw：acceptor + session（读循环/写队列/关闭），替换旧 gnet 代码
- GMS-P1.5 协议自测客户端（tools/protocoltest）：完成握手→收发加密 echo
DoD：握手包与 Java `getHello` 字节一致；加解密 round-trip 与 golden 100% 一致。

### P2 登录闭环（1–2 周）

目标：真实客户端走到角色列表。
- GMS-P2.1 database 基座（sqlx 池）+ accounts DAO（A 类表起步）
- GMS-P2.2 login server：版本检查/登录请求/账密校验（LoginCrypto SHA+盐） /错误码封包
- GMS-P2.3 世界/频道列表封包（packet/login.go）
- GMS-P2.4 选角列表/创建角色/删除（CharLoginHandler + CharlistPacket 对照）
- GMS-P2.5 自动注册开关（ZEV.自动注册）与账号白/黑名单表接入
DoD：V079 客户端登录 → 看到世界列表 → 建角成功并出现在列表（存 MySQL）。

### P3 wz 数据层（2–3 周）

目标：游戏数据可用，摆脱 Java 依赖。
- GMS-P3.1 v079 wz reader：header/目录树/节点类型（int/string/vector/uol/canvas/sound/img）
- GMS-P3.2 wzdump 工具 + 与 Java WzXML 输出 diff（J3 验证）
- GMS-P3.3 数据缓存：Map（MapleMapFactory 语义）、String（名字表）、Item（含 drop/奖励）、Skill、Mob、Reactor、Npc、Quest
- GMS-P3.4 备选 XML 路线（如 diff 阻塞超过 3 天触发）：转换脚本 + XML reader 实现同一接口
- GMS-P3.5 wztosql 工具移植：怪物掉落/道具表入库（MonsterDropCreator 语义）
DoD：读取 16 个 wz 全部无错；910000000 等代表图的地图名/NPC/刷怪点与 Java 版一致；掉落表生成可入库。

### P4 进入世界（1.5–2 周）

目标：角色站上地图并能互动。
- GMS-P4.1 channel server 骨架（频道注册到 world、端口 7575）
- GMS-P4.2 player 装配：从登录服迁移会话（CharacterTransfer 语义）→ 进图
- GMS-P4.3 mapp：地图实例、玩家集合、视野广播（spawn/despawn/move）
- GMS-P4.4 移动处理（MovementParse 语义）+ 状态广播
- GMS-P4.5 聊天（公屏/私聊/表情）+ 关键字屏蔽（abc/关键字屏蔽 表）
- GMS-P4.6 NPC 交互基础：点击 NPC 触发脚本占位（P7 完整化）
DoD：两个客户端同图互相可见、移动流畅、聊天互通、掉线重连正常。

### P5 角色与物品（2 周）

目标：角色状态完整可持久化。
- GMS-P5.1 model/Character：属性/经验/职业/技能点（MapleCharacter 存档字段拆分）
- GMS-P5.2 inventory：装备/消耗/设置/ETC 四栏 + 背包操作封包
- GMS-P5.3 item 定义（MapleItemInformationProvider 语义）+ 装备属性/强化（fumo 锻造）
- GMS-P5.4 存档：全量/脏位写库（对齐 saveToDB）+ 上线读档
- GMS-P5.5 仓库（MapleStorage）、Duey 邮件基础
DoD：穿脱装备/移动物品/下线重登状态一致；经验升级正确。

### P6 战斗与技能（2–3 周）

目标：核心玩法闭环。
- GMS-P6.1 攻击处理：近战/远程/召唤（AttackInfo/DamageParse 语义）+ 吸怪检测
- GMS-P6.2 伤害公式（PlayerStats/SkillFactory 语义，J6 对拍）
- GMS-P6.3 怪物 AI：巡逻/技能/无敌/死亡（MapleMonster）+ 刷怪调度（RespawnManager）
- GMS-P6.4 掉落与拾取（含盗窃/组队分配）
- GMS-P6.5 buff/debuff/冷却生命周期（Timer 调度）
- GMS-P6.6 玩家互伤（PVP 开关，先框架后 P9 完整化）
DoD：单杀/群杀怪物掉落可拾取入包；技能 buff 生效与过期正确；3 人同图战斗无死锁。

### P7 脚本系统（2–3 周）

目标：3922 个存量 JS 尽量零改跑起来。
- GMS-P7.1 goja 集成 + 脚本加载器（GBK 转码、缓存、热重载）
- GMS-P7.2 宿主 API：`cm`（NPCConversationManager 面收敛为最小充分集）+ `pi`/`qm`/`em`/`rm`
- GMS-P7.3 `java.*` shim 子集（按 scriptlint 扫描结果实现）
- GMS-P7.4 scriptlint 工具：全量扫描缺 API/语法错误/编码异常，产出可用率报告
- GMS-P7.5 quest 数据与状态机（server/quest + quest 脚本）
- GMS-P7.6 event 脚本框架（EventManager/EventInstanceManager 语义）
DoD：主城/转职/传送门常用脚本抽检 50 个全可用；npc 主目录可用率 ≥95%（scriptlint 报告为准）。

### P8 商业与社交（2 周）

目标：经济与社交系统可用。
- GMS-P8.1 NPC 商店（MapleShop）+ 商店数据
- GMS-P8.2 拍卖行（auction/auction1 合并为单实现，J4-B 类表）
- GMS-P8.3 银行（bankitem1/2 合并）与点券/充值卡（充值卡后台表）
- GMS-P8.4 组队/好友/公会/家族（world 域）
- GMS-P8.5 交易（MapleTrade）、玩家商店（PlayerShop/HiredMerch）
- GMS-P8.6 商城 cashshop + MTS（按需可裁剪 MTS）
DoD：买/卖/交易/拍卖挂单取回全流程存库正确；公会入退与公告广播正常。

### P9 ZEVMS 自定义玩法（2–3 周，可裁剪）

目标：娱乐服差异化功能（按需勾选，默认全做）。
- GMS-P9.1 PVP 系统（pvp/ + PVP/ 职业配置 ini）
- GMS-P9.2 副本系统（folder/副本相关 + 副本控制台语义）
- GMS-P9.3 BOSS 排行（bossrank1..10 合并为 rank 实例化）
- GMS-P9.4 活动框架：OX 答题/喜从天降/魔族攻城/推雪球/绝地求生（每日调度）
- GMS-P9.5 段位/成就/怪物书/钓鱼/洗魔（fumo）
- GMS-P9.6 反外挂（anticheat：吸怪/全屏/包频率 + AutobanManager）
DoD：每子系统一条端到端冒烟脚本入 CI。

### P10 运营面板（1–2 周）

目标：替代全部 Swing 控制台职能。
- GMS-P10.1 admin REST：在线玩家/踢人/发公告/倍率调整/发点券/广播
- GMS-P10.2 充值卡管理（导入/作废/查询，替代充值卡后台 exe）
- GMS-P10.3 简易前端（静态页即可）+ token 鉴权
- GMS-P10.4 QQ 机器人 webhook 桩（可选，对接协议留接口）
DoD：面板完成 Java 控制台 Top10 高频操作；无明文密码。

### P11 稳定化与上线（1–2 周）

- GMS-P11.1 压测机器人（tools/bot：模拟登录+移动+攻击）500 并发 24h
- GMS-P11.2 内存/锁/goroutine 泄漏体检（pprof）
- GMS-P11.3 故障演练：DB 断连重连、玩家存档中断恢复、优雅关停
- GMS-P11.4 部署物：单二进制 + configs + migrations + 静态资源 + 运维文档
DoD：压测达标（3.1-3）、无泄漏、优雅关停零丢档；发布 tag v1.0.0。

## 7. 进度追踪机制

- **PROGRESS.md**：唯一勾选点。三级结构：Phase 里程碑 → `GMS-Px.y` 任务 → 冒烟项。完成打 `[x]` 并附提交号/日期。
- **MIGRATION_MAP.md**：Java 件 → Go 件映射与状态（⬜未开始/🔄进行中/✅完成/❌不迁移）。P0 建全量骨架表，随做随标。
- **可用率指标**：P7 起每阶段记录脚本可用率 %（scriptlint 产出）。
- **完成度汇总**：PROGRESS.md 顶部表格自动更新（Phase x/y、任务 n/m）。
- **变更记录**：范围/决策变化追加于 PROGRESS.md 尾部，不改历史。

## 8. 风险登记册

| # | 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|---|
| R1 | goja 与 Rhino 语义差异（老 JS、java.* 依赖） | 高 | P7 延期 | scriptlint 早期全量扫描（P0 就跑一次摸底）；shim 白名单扩展 |
| R2 | v079 wz 二进制细节踩坑 | 中 | P3 延期 | wzdump 与 Java 输出逐文件 diff；3 天未破切 XML 备选线（GMS-P3.4） |
| R3 | 反编译语义失真（空 catch/block 标签/乱码名） | 高 | 隐蔽 bug | 三源交叉：Zevms↔交流源码↔079MAX2.jar 行为；关键公式以实测客户端为准 |
| R4 | 225 表中废弃/重复表误导 | 中 | 数据层返工 | P0 表分类 A/B/C，DAO 只碰 A+B |
| R5 | 客户端协议黑盒（079 老协议） | 中 | 各阶段联调卡壳 | Java 版可运行：架对照服抓包 diff（工具 protocoltest 支持录制回放） |
| R6 | 乱码资产（GBK 脚本/DB 字符集） | 高 | 脚本不可读 | J7 转码管线；无法恢复者标记并人工补 |
| R7 | 单人工程量超预期 | 高 | 周期拉长 | 阶段独立可交付；P9 可裁剪；每 Phase DoD 允许缩范围不减质量 |
| R8 | 数值手感漂移（随机/公式舍入） | 中 | 玩家可感知 | J6 固定种子对拍 + 伤害抽样回归 |
| R9 | K: 环境（E: 路径 my.ini、root/root）不可复现 | 低 | 联调阻塞 | P0.6 修路径+改密快照，独立 docs 化 |

## 9. 工作量估算汇总

| 阶段 | 估时（单人全职） |
|---|---|
| P0 基建 | 0.5 周 |
| P1 协议栈 | 1–1.5 周 |
| P2 登录闭环 | 1–2 周 |
| P3 wz 数据 | 2–3 周 |
| P4 进入世界 | 1.5–2 周 |
| P5 角色与物品 | 2 周 |
| P6 战斗与技能 | 2–3 周 |
| P7 脚本系统 | 2–3 周 |
| P8 商业与社交 | 2 周 |
| P9 自定义玩法 | 2–3 周（可裁剪至 1） |
| P10 运营面板 | 1–2 周 |
| P11 稳定化 | 1–2 周 |
| **合计** | **18–25 周**（裁剪 P9 后约 16–22 周） |

> 建议节奏：P0–P6 为"核心线"不可裁剪；P7 是价值兑现点优先保；P8/P10 可并行穿插；P9 按运营需求裁剪。
