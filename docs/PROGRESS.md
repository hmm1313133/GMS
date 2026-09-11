# GMS 进度追踪（PROGRESS）

| [PLAN.md](PLAN.md) 任务编号一一对应 + [FILETRACK.md](FILETRACK.md) 文件级状态。
> 规则：完成打 `[x]` 并附日期/(提交号)；进行中在行尾标 🔄；新增任务追加编号不改旧号；范围变化记入文末"变更记录"。
> **文件级追踪**：以 `docs/FILETRACK.md` 为准（533 个 Java 文件逐条标记 TODO/ACTV/DONE/MERG/SKIP），本文件只追踪阶段任务。

最后更新：2026-09-11（P4.4 移动处理完成：MOVE_PLAYER 解析 + 广播 + 坐标落地）

## 完成度汇总

| 维度 | 进度 |
|---|---|
| Phase | 1 / 12（P1 完成；P0 剩 0.6；P2 代码完成待 P0.6 验收；P3 进行中；P4 进行中） |
| 任务 | 21 / 77（P0.1-0.5 + P1.1-1.5 + P2.1-2.5 + P3.1 + P3.2 + P4.1 + P4.2 + P4.3 + P4.4；新增 P4.3b） |
| 文件级（FILETRACK） | DONE 30 / MERG 48 / ACTV 17 / TODO 386（余 SKIP 52，合计 533） |
| 脚本可用率（P7 起跟踪） | - |
| 当前里程碑 | **M1：登录闭环（P0–P2）🔄 / M2 数据层起步（P3）→ P4 进入世界（进图 + 同图互见已通）** |

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
- [x] GMS-P2.5 自动注册 + 白/黑名单 ✅ 2026-08-30
  - `internal/login/register.go`（新）：`bannedIP`（ipbans 前缀匹配）/ `bannedMac`（macbans 精确匹配 + Java 的零 MAC/长度≠17 豁免）/ `autoRegister`（账号不存在且未封禁时建号）/ `createAccount`（同机器码上限 ACCOUNTS_PER_MAC=100）
  - `internal/login/auth.go`：CharLoginHandler.login 的分支顺序完全对齐——先算 ipBan/macBan，再进自动注册分支，最后 `loginok==0 && banned && !gm -> 3`（GM 豁免）；顺带修掉 DB 不可达时 nil store 崩溃（降级答 5）
  - `internal/database/bans.go`（新）：`BannedIPs`/`IsBannedMac`/`CountAccountsByMac`/`InsertAutoRegisterAccount`；SQLite DDL 补 `ipbans`/`macbans`
  - `internal/config`：`login.register_enabled`（账号注册开关，取反对齐 Java `<=0`）/ `login.accounts_per_mac`（<=0 不限）；`Server.SetServerName`（Java MapleParty.开服名字，弹窗标题）
  - `tools/addaccount` 扩封禁表管理：`-bans`/`-banip`/`-banmac`/`-unbanip`/`-unbanmac`（P8 运营面板前唯一的封禁入口）
  - 测试：login +12 wire 端到端（注册落地/二次登录 CHOOSE_GENDER/开关关闭/保留密码 disconnect|fixme/机器码上限/封禁跳过注册/IP 与 MAC 封禁改写为 3/GM 豁免/无库降级）+ ban 匹配单测 2 + config 默认 1 + database SQLite 封禁/注册集成 1
  - live 冒烟（SQLite）：自动注册弹窗+reason 1 -> 落库（macs/SessionIP/gender=10）-> 二次登录 CHOOSE_GENDER；`-banip 127.0.0.` -> reason 3，解封 -> AuthSuccess；`-banmac` -> reason 3，解封 -> CHOOSE_GENDER
  - 顺带修的坑：`config.Load` 现在把相对 sqlite DSN 按**配置文件所在目录**解析（原来按进程 cwd——从 `tools\` 起服务会在 `tools\data\` 建出第二个空库，本次实踩）；`tools/start-gms.cmd` 固定 `cd /d I:\GMS`
- 冒烟：V079 客户端登录到角色列表并可建角

## P3 wz 数据层 🔄

- [x] GMS-P3.1 v079 wz reader ✅ 2026-08-30
  - `internal/wzs`（新包）：`type.go`（MapleDataType 全 17 值）/ `data.go`（Node + DirEntry 语义 + MapleDataTool 全族）/ `xml.go`（流式解析）/ `provider.go`（Provider + Root）
  - 语义保真点：getChildByPath 的"仅首段 .."规则、getChildren 只收元素节点、canvas 的 PNGPath 推导、目录名去 `.xml` / 跳过 `*.img` 目录、空 value 容忍（wz 里有 `<int value=""/>`）
  - 与 Java 的有意偏差：MapleDataTool 遇 nil/类型不符返回零值（Java 抛 NPE/CCE）；未知标签保留为 UNKNOWN_TYPE 节点（Java 返回 null 后 NPE）
  - 数据源：`I:\GMS\wz`（K:\079MAX2服务端\wz 的 WzXML 解包副本，39,986 XML / 735 MB，已加入 .gitignore）
  - 测试 10 个（类型映射/12 种标量/canvas+convex/路径与 `..`/DataTool 转换/provider 导航与缓存/错误路径/真实地图与 reactor UOL/解析报错/真实导出抽样 1187 文件）
- [x] GMS-P3.2 wzdump 工具 ✅ 2026-08-30
  - `tools/wzdump`：`-list`（16 wz 清单）/ `-tree`（目录树）/ `-dump`（节点树，含节点内路径）/ `-verify`（全量解析）
  - **`-verify` 实测：16 个 wz、39,986 个 img、22,026,219 个节点全部解析成功，failed=0，耗时 3m12s**
  - J3 的"与 Java XML diff"降级为不必要：Go 读的就是 Java 用的同一份 XML（见 MIGRATION_MAP §7 注 1）；真二进制 wz（J:\079MAX2客户端，3.9 GB）留作将来不依赖外部解包时的备选
- [ ] GMS-P3.3 数据缓存（Map/String/Item/Skill/Mob/Reactor/Npc/Quest）🔄
  - ✅ 已完成切片：**String.wz 名字表** `internal/wzs/names.go`
    - `Names` 一次装载 6 张表：Item（Cash/Consume/Eqp/Etc/Ins/Pet，name+desc）、Map（mapName+streetName）、Mob、NPC、Skill，外加 `Etc.wz/ForbiddenName.img`（Java LoginInformationProvider 的违禁名子串匹配）
    - 与 Java 的有意偏差：Java 按"计算出来的路径"查（SkillFactory 把 id 左补零到 7 位、MapleMapFactory 先由 mapid 算地区名），本服数据对不上（Skill.img 有 8 位键 10000018、Map.img 没有 Java 会查的 `china` 区），Go 改为按 id 建全量索引 —— 少一层字符串拼接，也救回了 Java 会漏掉的那批名字
    - 兜底值沿用 Java：`NO-NAME`（物品/怪物）、`MISSINGNO`（NPC）、技能缺失返回空串（Java 返回 null）
    - 缺图不致命：记入 `Names.Errors` 继续跑（Java 会抛 RuntimeException）
  - ✅ 配置与接线：`config.WZ{path,load_names}`（`[wz]` 段，默认 `wz` / true，对应 Java `net.sf.odinms.wzpath`）、`cmd/gms` 启动期装载 + 计数日志；wz 缺失/损坏只降级不崩（与 DB 降级同款）
  - 测试 2 个（testdata 固定装置全表断言 + 真实导出断言）+ config 2 个（默认值 / `[wz]` 段解码）
  - **live 冒烟**：`bin/gms.exe` 日志 `wz root opened path=wz` → `wz names loaded items=48371 maps=4235 mobs=1833 npcs=3192 skills=527 forbidden=466`（0.2s）→ `login server listening :8484`；protocoltest 握手回归正常（hello version=79）
  - 待做：Item（`Item.wz` + `Character.wz` 装备统计）、Skill、Mob、Npc、Reactor、Map（Map.wz 地图实例数据留给 P4.3）
- [ ] GMS-P3.4 （备选触发式）XML 兼容路线 —— 不需要：Go 读的就是 Java 的 WzXML 解包（见 MIGRATION_MAP §7 注 1）；真二进制 reader 若将来要做，接口已可插拔
- [x] GMS-P3.5 wztosql 掉落表入库 ✅ 2026-08-31
  - **数据归属（用户拍板，方向性前提）**：本服掉落是**第三方预置**的，不是 wz 生成的 —— `K:\079MAX2服务端\mysql\MySQL\data\079@002dmax2` 库里的 `drop_data`（14,243 行 / 900 个怪）/ `drop_data_global`（16 行）才是真相，`tools/wztosql/MonsterDropCreator.java` 那份生成器在本服**根本不是数据来源**（漂移实测见下）。所以本轮的"入库"= 把这条权威数据搬进 Go 树，wz 侧只做重建与对比
  - ✅ **`tools/dropexport`（新工具）**：连权威 MySQL 库把掉落表快照成可移植 SQL（`-dsn` / `-out` / `-tables` / `-batch` / `-stats`）。输出**只有 INSERT 不带 DDL**（建表两端各有：MySQL 见 `0001_base.sql`，SQLite 见 `db.go`），按 `dropperid, itemid, chance…` 排序保证重复导出零 diff，省掉自增 `id` 让目标库自己分配
  - ✅ **`migrations/0002_drop_data.sql`（14,259 行 / 493 KB）**：权威库快照，文件头写明来源与"不可手改"；同一份内容 embed 进 `internal/database/seed_drop_data.sql`
  - ✅ **`internal/database/drops.go`（新）**：`MonsterDrops`（Java `retrieveDrop` 的 `WHERE dropperid = ?`）/ `GlobalDrops`（`WHERE chance > 0`）/ `DropStats`；SQLite DDL 补 `drop_data` + `drop_data_global`（翻译 `0001_base.sql:1114/1129`）
  - ✅ **SQLite 自动装载**：`openSQLite` 建表后回放 embed 快照，**仅当 `drop_data` 为空时**执行（`seedDrops`）——运营手工调过的行永不被覆盖；MySQL 路径不导入（它连的就是权威库）
  - ✅ **`internal/life/drops.go`（新包）**：Java `MapleMonsterInformationProvider` 移植 —— 按怪物 id 缓存掉落表 + 构造期装载全局掉落，保真 **EQUIP 类 `chance /= 3`**（加载期削，存的是原值）；有意偏差：Java 用无锁 HashMap 跨 netty 线程，Go 加 `RWMutex`（另写并发回归测试）；出错不缓存（对齐 Java 的 SQLException 分支）
  - ✅ **`internal/dropgen`（新包）+ `tools/wztosql`**：MonsterDropCreator 移植 —— `getChance` 全表（含 Java 两处 fallthrough：case 112→130 组得 700、case 400→401/402 得 9000）、`multipleDropsIncrement`、`IncrementRate`、8 个硬编码特殊怪、MonsterBook.img reward 的 boss/rarity 倍率链、怪物卡按名匹配。**工具不写库**，只出 SQL 或 `-diff` 漂移报告
  - 实测（`I:\GMS\wz`）：`items=43050 mobs=1614 (boss 517) monsterbook rows=13894 entries=14400 monsters-with-drops=430 cards=411`
  - **漂移实测（`-diff`，印证"以库为准"）**：库 14,243 行/900 怪 vs wz 重建 14,400 行/430 怪；仅库有 5,693 对、仅 wz 有 6,068 对、同对 chance 不同 3,384 对 —— 两套数据基本无关
  - 本服特化：怪物卡名是中文「蜗牛卡片」，Java 硬编码的英文 `" Card"` 后缀只能匹配 1 张（库里有 422 条卡掉落），故加 `-card-suffix-cn`（= `卡片`）开关；中文实参在本机命令行会 mojibake，不能靠 `-card-suffix 卡片` 传（§三.9）
  - `monstercarddata` 表本服**不存在**（已核权威库表清单），Java 生成的这部分在 SQL 里以注释形式保留
  - 测试：`internal/database/drops_test.go` 5 个（SQLite 快照 14,243/900/16 逐值断言 / 未知怪空集 / 全局掉落含 `chance>0` 过滤 / 重开不重复导入 / **embed 快照与 `migrations/0002_drop_data.sql` 字节一致**的护栏）+ `internal/life/drops_test.go` 6 个（缓存/EQUIP÷3/出错不缓存/全局与 ClearDrops/并发/isEquip）+ `internal/dropgen/dropgen_test.go` 8 个（getChance 55 用例含死代码 4031456 与两处 fallthrough / 多份掉落展开 / 卡后缀中英对照 / boss 倍率含 8810018 落穿 / 真实 wz 全量 / SQL 输出）
  - **live 冒烟**（SQLite + `bin/gms.exe`）：`database connected` → `drop tables loaded rows=14243 monsters=900 global=16` → `wz names loaded …` → `login server listening :8484`；protocoltest 握手回归 OK（hello version=79）。测完停进程
- 冒烟：16 wz 全读无错；代表图数据与 Java 版一致

## P4 进入世界 🔄 （里程碑 M2 出口）

- [x] GMS-P4.1 channel server 骨架 ✅ 2026-09-11
  - `internal/channel`（新包）：`server.go`（多频道实例 + 每频道 netw.Acceptor）、`players.go`（PlayerStorage 骨架）
  - **端口公式保真**：`7574 + channel`（channel 从 1 起），即 gms.toml 的 `channel.port=7575` 是频道 1；`count=5` → 7575..7579（Java `ChannelServer.DEFAULT_PORT` / `ZEV.Port<channel>`）；Count 上限 10（`startChannel_Main`）；exp/meso/drop 上限 100（`run_startup_configurations`）
  - **握手与登录服同源**：IV `{70,114,12,rand}` / `{82,48,120,rand}` + version 79 的 raw hello（同一份 `MapleServerHandler.channelActive`）；PONG→PING；channel>0 时的 isShutdown 连接门禁
  - **选角交棒（P4.1 的可见出口）**：`Character_WithoutSecondPassword`（0x000A）迁入 `internal/login/select.go` —— DB `loggedin==2` → 内存登录态 → `login_Auth(charId)` → 频道存活/world==0 校验 → `putLoginAuth` + `getServerIP(port, charId)`；新增封包 `ServerIPPacket`（short 0 + ip4 + port + charId + `{1,0,0,0,0}`）与 `EnableActionsPacket`（UPDATE_STATS 空掩码）
  - **`internal/world`（新包）**：`registry.go`（Java LoginServer.loginAuth/loginIPAuth 两张静态表：put/take(取走)/contains/remove/add）+ `find.go`（Java World.Find 子集，PlayerStorage 注册时同步）
  - **load 上报**：频道服人数变化即推送 login（Java 是 LoginWorker 每 10 分钟轮询 `getChannelLoad`）；SERVERLIST 显示值按 `min(1200, 人数*1200*频道数/userLimit)` 换算（原版把换算值就地写回共享 map，第二轮会二次换算，Go 改为每次回包按真实值算）
  - **双登清理（`unlockAcc`，P4 前置项）**：双登仍答 7，但先踢人 —— 有活跃会话 → 弹窗"检测到其他登陆"+ 1s 后关连接（对齐 `unLockDisconnect` 的 `Thread.sleep(1000)`）；无活跃会话（崩溃残留）→ `accounts.loggedin` 置 0（Java else 分支，新 DAO `ResetAccountLogin`），使下一次登录可用
  - `config.Server.ExternalIP`（Java `MapleParty.IP地址`；原版默认 0.0.0.0 客户端不可用，Go 默认 127.0.0.1）
  - 测试：`channel` 6 个（端口公式/Count 上限/速率上限/hello+PONG/PLAYER_LOGGEDIN 骨架/PlayerStorage 与 load 回调）、`world` 4 个（票据 put-take-ip 集合/并发/Finder）、`login/select_test` 7 个（SERVER_IP 字节级 + 票据落地/DB 态门禁/未授权角色 enableActions/频道停机即断连/展开换算/双登踢活会话/双登清孤儿）
  - **live 冒烟**：`bin/gms.exe` 启动日志 5 频道 7575-7579 全监听；protocoltest `-login -charlist -selectchar` 得 `SERVER_IP ip=127.0.0.1 port=7575 charID=2`，选 channel 3（CHARLIST byte 2）得 `port=7577`；`-addr 127.0.0.1:7575 -loggedinas 2` 收 hello(79) 且服务端日志 `player logged in (skeleton)`；双登首连答 7 + 日志 `double login, stale loggedin reset`，次连成功；冒烟数据已清（删角 0x7FFE state=0 + 删号）
- [x] GMS-P4.2 player 装配（会话迁移 → 进图）✅ 2026-09-11
  - `internal/packet`（新包，PLAN §4.1 的 `packet/` 落点）：`packet.go` 的 `AddCharStats`（从 `login/packets.go` 搬来，login 改为调用它，P2.4 逐字节断言不变）/`PacketTimeNow`/`CharInfoPacket`（getCharInfo 全量）/`TemporaryStatsResetPacket`/`ServerMessagePacket`；`rand.go` 的 `RandStream`（client/PlayerRandomStream 的 CRand32 三元组）
  - **getCharInfo 布局逐字段对拍**：`WARP_TO_MAP(0x0081)` + `int(channel-1)` + `0,1,1` + `short 0` + CRand 3×int + `long -1` + `byte 0` + `addCharStats` + `buddyCapacity` + `byte 1`(bless) + 背包/技能/冷却/任务/戒指/传送石/怪物卡/questinfo 空段 + `int 0` + `short 0` + 尾部时间戳；空背包整包 **273 字节**
  - **保真怪癖**：`connectData` 三次 `CRand32__Random()` 返回值**完全相同**（只读 `seed1..3`、只写 `seed1_..3_`），实发三个相同 int（冒烟实测 `-554203/-554203/-554203`）
  - `internal/mapp`（新包）：最小地图实例（id + 玩家集合 + `Player` 接口 `ObjectID()`）；玩家 oid = cid（Java `setObjectId` 抛异常）
  - `internal/channel`：`login.go` 的 `handlePlayerLoggedIn`/`loggedIn2`（挂起表优先 → `loadCharFromDB` → 顶号 → 注册 + 滚动公告 → WARP_TO_MAP + TEMP_STATS_RESET → `map.addPlayer`）；`players.go` 的 `Player` 扩为角色行 + 随机流 + 地图，新增 `CharacterTransfer` 挂起表（取走式 / 40s 过期）；`server.go` 加 `characterStore` 接口 + `SetStore`、地图注册表（惰性创建）、`OnClose` 注销、`forceRemovePlayerByAccID` 顶号
  - `database.GetCharacterByID`（`SELECT * FROM characters WHERE id = ?`，未找到 `nil,nil`）；`inventoryslot` 表未迁 → 槽位用 `saveNewCharToDB` 默认 32/32/32/32/60
  - 跳过（注记）：`登陆验证开关` 机器人验证（configvalues 缺表，PLAN SKIP）、GM 隐身/加速、`updateLoginState(2)`（登录服已写）、好友/组队/公会/家族/信使（P8）、`MAP_CHANGE`/warp 到别的图（P4.3）
  - 测试：`packet` 3 个（WARP_TO_MAP 逐字段解码至 `Len()==0` / CRand32 确定性与同值怪癖 / reset+banner）、`channel` +7（进图全链含断线注销 / 未知角色 / 无库 / 库报错 / 挂起表优先 / 挂起表一次性+过期 / 顶号）、`database` 补 `GetCharacterByID` 命中与未命中
  - **live 冒烟**：建角 id=3 → `-selectchar` 得 `SERVER_IP port=7575` → `-addr 127.0.0.1:7575 -loggedinas 3` 收 `SERVERMESSAGE type=4 "GMS Go 服务端测试中"` + `WARP_TO_MAP len=273 charID=3 name="12345" map=0 buddy=20`；服务端日志 `player logged in/out`；收尾删角 0x7FFE + 删号 + 停进程
  - 探针：`-loggedinas` 等 WARP_TO_MAP 再退出 + WARP_TO_MAP/TEMP_STATS_RESET 解码 + SERVERMESSAGE type=4 修正
- [x] GMS-P4.3 mapp 玩家集合 + 视野广播（spawn/despawn）✅ 2026-09-11
  - `internal/packet` 加 `SpawnPlayerPacket`（= `MaplePacketCreator.spawnPlayerMapobject`，SPAWN_PLAYER 0x00A2）与 `RemovePlayerFromMapPacket`（0x00A3）；`addCharLook` 从 `internal/login/packets.go` 提到 `packet.AddCharLook(w, chr, mega)`（login 走 mega=true、频道 spawn 走 mega=false，字节不变，P2.4 断言仍全过）
  - **spawn 布局逐字段保真**（v079 新角色默认态：无公会/无 buff/无骑宠/无宠物/无商店/无戒指）：`short SPAWN_PLAYER + int cid + byte level + str name + str "" + 6×0(公会外观) + int 0 + 00 E0 1F 00 + int 0(变形) + int 0(buffmask 高) + int 0(buffmask 低) + 6×0 + (int CHAR_MAGIC_SPAWN + long 0 + short 0 + byte 0)×2 + int magic + short 0 + byte 0 + int magic + long 0 + byte 0(未骑宠) + long 0 + int magic + 00 01 41 9A 70 07 + long 0 + short 0 + int magic + long 0 + int 0 + byte 0 + int magic + long 0 + short 0 + byte 0 + int magic + byte 0 + short job + addCharLook(mega=false) + int 0(现金道具数) + int 0(道具特效) + int 0 + int -1 + int 0(椅子) + pos + byte 0(stance) + short 0(FH) + byte 0 + int 1 + int 0 + int 0(坐骑 lvl/exp/fatigue) + byte 0 + byte 0 + addRingInfo×2(byte 0 + int 0) + byte 0 + short 0`；空角色整包 **249 字节**
  - **CHAR_MAGIC_SPAWN**（= `Randomizer.nextInt()`，Java 注释的 tickCount）只抽一次、8 处重复；坐骑 level/exp/fatigue 取 `MapleCharacter.loadCharFromDB:894` 的 `new MapleMount(..., (byte)0, (byte)1, 0)` → 1/0/0
  - `internal/mapp`：`Player` 接口扩为 `PacketSink`（SendPacket）+ `ObjectID` + `SendSpawnData(sink)`（= `MapleMapObject.sendSpawnData`）+ `DespawnData`；`Map.AddPlayer` 复刻 `MapleMap.addPlayer` 广播（对同图其他人发新人 spawn → 给新人发每个老玩家的 spawn → 新人自身 spawn），`Map.RemovePlayer` 返回是否移除并广播 `removePlayerFromMap`（**先删后广播**，离开者看不到自己的 despawn；二次移除不重复广播），新增 `Map.Broadcast(pkt, except)`（= `broadcastMessage(source, pkt, false)`）
  - `internal/channel`：`Player` 实现 `SendPacket/SendSpawnData/DespawnData`；`login.go` 的 `Map(...).AddPlayer` 与 `server.go` 的 `OnClose → RemovePlayer` 自动带上广播（顶号路径先移除，OnClose 第二次移除是 no-op）
  - **有意偏差（已注记）**：`sendObjectPlacement` 在 Java 里按 view-range 且覆盖所有 map object 类型；P4.3 无怪物/道具、且角色尚无坐标（移动是 P4.4），Go 改为「同图所有其他玩家」；spawn 包的 pos 写 (0,0)，客户端在首个 `MOVE_PLAYER` 时归位；Java 因 chr 已在地图对象表里而把自身 spawn 发了两次，Go 只发一次
  - 测试：`packet` +2（SPAWN_PLAYER 逐字段解码至 `Len()==0` + 8 处 magic 相同 / REMOVE_PLAYER_FROM_MAP 逐字节）、`mapp`（新包测试 3：进图广播顺序 / 离开广播与二次移除 no-op / Broadcast 排除 source）、`channel` +1（双客户端同图 wire 端到端：A 收 B 的 SPAWN_PLAYER、B 收 A 的 SPAWN_PLAYER 与自身、B 断开后 A 收 REMOVE_PLAYER_FROM_MAP）
  - **live 冒烟全过**（SQLite + 双账号双角色直接连 7575）：A=`11111`(acc9)、B=`33333`(acc10) 同图；A 收 `TEMP_STATS_RESET` → `SPAWN_PLAYER charID=4`(自身) → `SPAWN_PLAYER charID=6`(B 进图) → `REMOVE_PLAYER_FROM_MAP charID=6`(B 退出)；B 收 `SPAWN_PLAYER charID=4`(看到 A) → `SPAWN_PLAYER charID=6`(自身)，spawn 包 len=249 与算出的字节数一致
  - 探针：`-hold`（脚本回包到齐后继续监听，用于观察 spawn/despawn）+ SPAWN_PLAYER/REMOVE_PLAYER_FROM_MAP 人类可读解码
  - **踩坑**：同账号两角色同图会被 P4.2 的 `forceRemovePlayerByAccId` 顶掉（冒烟首轮 A 只收到自身 spawn 就断线），必须用两个账号
- [x] GMS-P4.4 移动处理（MovementParse 语义）+ 状态广播 ✅ 2026-09-11
  - **`internal/movement`（新包）**：`Fragment`（= `server/movement/StaticLifeMovement`，本源码树唯一的 LifeMovement 实现）+ `Parse`（= `MovementParse.parseMovement`，kind=1 玩家）+ `Serialize`/`SerializeMovementList`（= `StaticLifeMovement.serialize` / `PacketHelper.serializeMovementList`）+ `UpdatePosition`（= `MovementParse.updatePosition`，`Target` 接口 = Java `AnimatedMapleMapObject` 的 setPosition/setFh/setStance）
  - **逐命令布局对拍**：0/5/15/17（pos+wobble+unk[+fh@15]）、1/2/6/12/13/16/18/19/22（wobble）、3/4/7/8/9/11（pos+unk）、10（wui byte）、14（wobble+fh）、default（仅 state/duration）
  - **两个 Java 怪癖保真**：① `NewFh` 是 `StaticLifeMovement` 构造器第 5 个实参 —— 0/5/15/17 组传 `unk`、其余传 0，`updatePosition` 用它写 foothold（不是序列化用的 `Fh`）；② 3/4/7/8/9/11 组读出的 duration 被丢弃（构造时传 0），重广播写 0
  - **`internal/packet`**：加 `MovePlayerPacket(cid, moves)`（= `MaplePacketCreator.movePlayer`：short MOVE_PLAYER + int cid + **int 0**（Java 把 writePos(startPos) 注释掉了）+ 移动列表）；`SpawnPlayerPacket` 加 `(x, y, stance)` 参数 = Java `chr.getPosition()`/`chr.getStance()`（foothold 仍硬编码 0）
  - **`internal/channel`**：新 `movement.go` 的 `handleMovePlayer`（= `PlayerHandler.MovePlayer`）——`skip(33)`（v079 客户端前缀）→ parse（失败即丢包）→ `map.broadcastMessage(player, movePlayer(...), false)`（排除自己）→ `ApplyMovement`（`updatePosition` + `setOldPosition`）；`Player` 加 `Pos/Stance/Fh/OldPos/FallCounter`
  - **保真校正（重要）**：`PlayerHandler.MovePlayer` 用的是 **boolean 重载** `broadcastMessage(player, pkt, false)`，它的 rangeSq 是 `POSITIVE_INFINITY`（**无视野过滤**）；只有 `Point rangedFrom` 重载才用 `GameConstants.maxViewRangeSq`。此前 SESSION_STATE §四 的"P4.4 要补按坐标过滤的重载"是误读，本轮不引入坐标过滤
  - **有意偏差（已注记）**：不复制 `slea.available() 13..26` 的启发式校验（误判会丢有效移动）；跳过 飞天检测 地图检查（configvalues 开关缺表）与 follow/clone 重广播、坠落计数（依赖 foothold，P4.3b/P6）
  - 测试：`movement` 5 个（12 种命令往返逐字段 + 丢弃 duration 的字节断言 + NewFh 怪癖 + 坏包 3 例 + updatePosition 取末段）、`packet` +1（MOVE_PLAYER 逐字段）+ spawn 位置断言、`channel` +1（双客户端 wire：B 收 A 的 MOVE_PLAYER、A 不收自己的、位置/oldPos 落地、坏包丢包不关连接）
  - **live 冒烟全过**（SQLite + 双账号 p44a/p44b + 双角色 7/8 直连 7575）：B 收 `[MOVE_PLAYER: charID=7 commands=1]`，A 不收自己的移动；服务端日志 `channel packet opcode=MOVE_PLAYER len=50`（= 2 opcode + 33 前缀 + 15 单条移动）无报错；spawn/despawn 回归正常
  - 探针：`protocoltest` 加 `-move "x,y"`（发 MOVE_PLAYER，body = 33 零字节前缀 + 单条 type 0）与 MOVE_PLAYER（0x00BB）解码
- [ ] GMS-P4.3b 地图实例数据（Map.wz `info`/`life`/`foothold`/`portal` = Java `MapleMapFactory`）
  - 消费方：怪物/NPC 生成（P6）、传送门/换图（P4.4 warp 未做，见 FALL 计数）；`Map(id)` 惰性实例已就位
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
- 2026-08-30 范围变更（P0.6 / J3）：①V079 客户端实连（P0.6）本轮**继续暂缓**，先推 P3 数据层；②wz 路线由"自研二进制 reader + 与 Java XML diff"改为**直读 WzXML 解包 XML**——实测 K:\079MAX2服务端\wz 的 16 个 *.wz 全是解包目录（39,986 XML / 735 MB），即 Java 服读的就是这份数据，Go 读同一份天然一致；解包已复制到 `I:\GMS\wz` 并加入 .gitignore。真二进制 wz（J:\079MAX2客户端\*.wz，约 3.9 GB）保留为将来"不依赖外部解包"时的备选，接口已按 provider.MapleData 抽象可插拔（详见 MIGRATION_MAP §7 注 1）。
- 2026-09-11 P4.1 完成（channel server 骨架）：新增 `internal/channel`（多频道监听 7575-7579、同源 hello/IV、PLAYER_LOGGEDIN 骨架、PlayerStorage）与 `internal/world`（LoginRegistry 票据 + World.Find 子集）；登录服补 `CHAR_SELECT` 全链路（`select.go` + `ServerIPPacket`/`EnableActionsPacket`），并把 `MapleClient.unlockAcc` 的双登清理一并实现（原先残留 `loggedin=2` 会导致账号永久登不进，冒烟实踩）。频道"注册到 world"按单进程架构落为直接接线：端口查询供 SERVER_IP + 人数变化回调供 SERVERLIST 负载。
- 2026-09-11 P4.2 完成（player 装配与会话迁移/进图）：新增 `internal/packet`（MaplePacketCreator/PacketHelper 共享层：`AddCharStats` 从 login 搬到此处复用、`CharInfoPacket`=getCharInfo、`TemporaryStatsResetPacket`、`ServerMessagePacket`、`RandStream`=client/PlayerRandomStream）与 `internal/mapp`（最小地图实例占位）；`internal/channel` 实装 `InterServerHandler.Loggedin2`（挂起表优先 → `loadCharFromDB` → 同账号顶号 → 注册 + 滚动公告 → WARP_TO_MAP + TEMP_STATS_RESET → `map.addPlayer`）+ `CharacterTransfer` 挂起表 + 地图注册表 + `OnClose` 注销；`database.GetCharacterByID` 新增。live 冒烟：`-loggedinas 3` 收 `WARP_TO_MAP len=273`（三连随机 int 相同，保真 CRand32 怪癖）。
- 2026-09-11 P4.3 完成（视野广播 spawn/despawn）：`internal/packet` 加 `SpawnPlayerPacket`（= `MaplePacketCreator.spawnPlayerMapobject`，249 字节逐字段保真 + `CHAR_MAGIC_SPAWN` 8 处重复）与 `RemovePlayerFromMapPacket`，`addCharLook` 提到 `packet.AddCharLook(..., mega)` 共享（login=true / 频道=false，P2.4 断言不变）；`internal/mapp` 的 `Player` 接口扩为 `PacketSink` + `SendSpawnData` + `DespawnData`，`Map.AddPlayer/RemovePlayer/Broadcast` 复刻 `MapleMap.addPlayer/removePlayer/broadcastMessage` 的视野广播；`internal/channel` 的进图与 `OnClose` 自动带上广播。新增任务 GMS-P4.3b（Map.wz 地图实例数据 = MapleMapFactory，供 P4.4 传送/换图与 P6 怪物生成）。live 冒烟：双账号双角色同图互见（A 收 B 的 SPAWN_PLAYER、B 收 A 的 + 自身，B 退出后 A 收 REMOVE_PLAYER_FROM_MAP）。
- 2026-09-11 P4.4 完成（移动处理）：新增 `internal/movement`（`server/movement` 的 LifeMovementFragment + `MovementParse`：`Parse`/`Serialize`/`SerializeMovementList`/`UpdatePosition`）；`packet.MovePlayerPacket`（= `MaplePacketCreator.movePlayer`，int 0 + 移动列表）与 `SpawnPlayerPacket(..., x, y, stance)`（spawn 起用真实坐标）；`channel.handleMovePlayer`（`PlayerHandler.MovePlayer`：skip(33) → parse → 广播给同图其他玩家 → 更新自身 Pos/Stance/Fh/OldPos），`Player` 加坐标态。**保真校正**：MovePlayer 的 `broadcastMessage(player, pkt, false)` 是无限视野（无 maxViewRangeSq 过滤），此前文档的"要补坐标过滤重载"作废。live 冒烟：B 收 A 的 MOVE_PLAYER，A 不收自己的。
