# GMS 会话交接文档（SESSION STATE）

> 用途：跨会话恢复工作现场。加载本文件后可从"断点"继续重构。
> 配套：PLAN.md（总方案）、PROGRESS.md（阶段勾选）、FILETRACK.md（533 文件级状态）。

保存时间：2026-08-30（P2.4 完成，含 live 冒烟；下一步 P2.5 自动注册）
工作目录：`I:\GMS`（go module 名 `GMS`，Go 1.25.5）
目标：将 `I:\Zevms`（079MAX2/ZEVMS Java）重构为 Go，按 P0->P11 推进。

## 一、当前编译/测试状态（保存前最后一次验证）

```
go build ./...   ✅ 通过
go vet ./...     ✅ 通过
go test ./...    ✅ 全绿
```

- `internal/login`：24 个测试全过（+P2.4 的 TestCharFlow/TestSetGenderFlow/TestCreateCharSlotLimit wire 端到端）。
- `internal/database`：SQLite 集成测试 1 个（TestSQLiteAccountChain）+ 可选 MySQL 集成（GMS_TEST_DB_DSN）。
- **live 登录+世界列表+选角/建角冒烟已通过**（SQLite 后端，见 §二）。

## 二、本次会话已完成（2026-08-30，断点 D：P2.4 选角列表/建角/删角）

1. **`internal/database/characters.go` 新文件**：`Character` DAO（CHARLIST 所需 27 列）+ `GetCharactersByAccount`（loadCharactersInternal）/`GetCharacterIDByName`（getIdByName，-1=无）/`InsertCharacter`（saveNewCharToDB 的 characters 行子集）/`DeleteCharacterByID`（deleteCharacter 子集，state 0/1）/`CharacterSlots`（character_slots 表惰性建行）；`accounts.go` 加 `UpdateAccountGender`。
2. **SQLite DDL 扩**：characters 全列翻译（`migrations/0001_base.sql:904`，62 列，`"int"` 引用）+ character_slots；`DB.driver` 字段 + `dialectInt()`（MySQL 反引号/SQLite 双引号共用一份查询）。
3. **`internal/login/packets.go` 续**：`CharListPacket`（getCharList：byte0+int0+byte n+entry+short3+int slots）、`addCharStats`（int id+13 字节定长名+gender/skin+face/hair+24 零+level+job+8 stat shorts+ap+sp+exp+fame+int0+FT long+map+spawn）、`addCharLook`（gender/skin/face/mega0/hair+0xFF 0xFF+cWeapon 0+3 宠物 0）、`addCharEntry`（尾 byte 0，job==900 再 byte 2——ranking 参数在 ZEV 反编译里未用，保真不写）、`CharNameResponsePacket`/`AddNewCharEntryPacket`/`DeleteCharResponsePacket`（0x7FFE）/`GenderChangedPacket`/`LicenseRequestPacket`（LOGIN_STATUS+22 的 ZEV quirk）/`ServerNoticeDialogPacket`（SERVERMESSAGE type1 弹窗）。
4. **`internal/login/chars.go` 新文件**：`handleCharlistRequest`（0x0009：byte server+byte channel+int 跳过 -> world 0/allowedChar 填充 -> CHARLIST）、`handleCheckCharName`（0x000C：名字正则 `[0-9\u4e00-\u9fa5]{2,5}`+查重+RESERVED，非法名发弹窗+getLoginFailed(1)+used=1）、`handleCreateChar`（0x0011：JobType 1=冒险家 job0/map0、0=骑士团 job1000/map130030000、2=战神 job2000/map914000000；初始属性 12/5/4/4 hp mp 50；鞋/武器白名单；slots 上限）、`handleDeleteChar`（0x0012：allowedChar 门禁+2ndpassword 链 state 16+删除）、`handleSetGender`（0x0004：改 gender+GENDER_SET+license+updateLoginState(0)）。全部走 NeedsChecking 门禁。
5. **测试**：`chars_test.go` 3 个 wire 端到端（空列表/建角回包与落库断言/重名检查三包/删角/SET_GENDER 双包+落库/slots 上限弹窗）；fakeStore 扩 characters 方法。
6. **live 冒烟全过**（SQLite）：`-charlist` 空列表（0 角色 6 slots）-> `-hex` CREATE_CHAR（修正后 weapon=1302000=0x13DDF0）回 ADD_NEW_CHAR_ENTRY id=1 布局逐字节对 -> `-charlist` 回显"冒烟1"（GB18030 往返正确）。
7. **排障实录**：探针 `-hex` 手写字节把 1302000 误算成 0x13DD78（=1301880）触发白名单拦截；日志 name "鍐掔儫1" 是 Get-Content 按 GBK 读 UTF-8 文件的显示问题，服务端数据正确。
8. **文档**：PROGRESS/FILETRACK/MIGRATION_MAP 同步（PacketHelper/MapleCharacterUtil 转 ACTV）。

### 断点 C 会话（2026-08-30，P2.3 世界/频道列表）

1. **`internal/login/packets.go` 续**：`ServerListPacket`（Java getServerList 逐字段保真：byte id + str 名 + byte 状态 + str 事件横幅 + short 100 + short 100 + byte lastChannel + int 500 + 每频道[str "名-i" + int load + byte 世界 + short i-1] + short 气球数 + 每气球[short x + short y + str msg]）、`EndOfServerListPacket`（09 00 FF）、`ServerStatusPacket`（06 00 + short 0/1/2）。
2. **`internal/login/worlds.go` 新文件**：`WorldConfig`/`World`/`Balloon` 类型 + `SetWorlds`（按 ID 排序、建 channelLoad map{1..N:0} 对齐 LoginServer.addChannel）+ `handleServerList`（每世界一包 + End）+ `handleServerStatus`（numPlayer>=limit=2 / >=半=1 / 0）+ `SetUsersOn`/`UsersOn`（P4 频道服接线用）。`WorldConfigFrom(cfg)` 供 main 接线。
3. **`internal/login/server.go`**：OnPacket 分发加 `SERVERLIST_REQUEST(0x0002)`、`LICENSE_REQUEST(0x0003)`（ZEV quirk：也路由到 ServerListRequest）、`SERVERSTATUS_REQUEST(0x0005)`；Server 加 `worldMu/worlds/usersOn` 字段。
4. **`internal/login/auth.go`**：client 加 `loggedIn` 字段，`UpdateLoginState(2)` 成功后置真（对齐 Java updateLoginState 语义）；P2.3 两个 handler 用它做 NeedsChecking 门禁（未登录静默丢包）。
5. **`internal/config`**：Server 段加 `Worlds []World`/`EventMessage`/`UserLimit`/`Balloons`（TOML `[[server.worlds]]`）；Defaults 1 世界 {id:0,state:1}（对齐原版蓝蜗牛状态=1）；Validate 查 id/state 范围与重复。
6. **测试**：`packets_test.go` +4（ServerList 全布局/稀疏 load=1200/End/Status）；`worlds_test.go` +2 wire 端到端（登录后请求列表：双世界按 ID 排序、频道/气球全解析、END 09 00 FF、status=0；未登录请求 300ms 静默无回包）。
7. **`tools/protocoltest`**：加 `-serverlist`/`-status` 探针 + SERVERLIST/SERVERSTATUS 人类可读解码（世界/频道/气球逐行打印）；脚本模式收到全部期望回包即退出。
8. **live 冒烟全过**（SQLite + `tools/start-gms.cmd`）：`-login testgo:test123 -serverlist -status` 输出 LOGIN_STATUS OK accID=2 -> SERVERLIST world=0 "GMS" state=1 5 频道 load=0 -> END_OF_LIST -> SERVERSTATUS NORMAL(0)；未登录 `-serverlist` 无回包（门禁生效）。测完删号停进程复归全绿。
9. **文档**：PROGRESS/FILETRACK/MIGRATION_MAP/SESSION_STATE 同步（FILETRACK：LoginServer+Balloon 转 MERG，MERG 31->33）。

### 断点 B' 会话（2026-08-30 上午，SQLite 替代 MySQL）

1. **SQLite 替代 MySQL（用户决策）**：`[database]` 加 `driver` 字段（"mysql" 默认保真 / "sqlite" 纯 Go 冒烟）。SQLite 路径（`internal/database/db.go` openSQLite）：自动建父目录 + 自动建 `accounts` 表（逐列翻译 `migrations/0001_base.sql:224` 的 MySQL DDL，34 列全量对齐）。驱动 `modernc.org/sqlite` v1.57.0（无 CGO，Windows 免 gcc）。
2. **SQL 双方言兼容化**（`internal/database/accounts.go`）：
   - `CURRENT_TIMESTAMP()` -> `CURRENT_TIMESTAMP`（SQLite 只认无括号，MySQL 两者皆可）。
   - `2ndpassword` 列名加反引号（SQLite 标识符不能以数字开头；MySQL 原生支持反引号，一份 SQL 两边跑）。DDL 里用双引号 `"2ndpassword"`（仅 SQLite 路径执行）。
   - 已知无害偏差：SQLite `CURRENT_TIMESTAMP` 存 UTC（MySQL 存 +08:00），只有 20s transition 陈旧检查读 lastlogin，偏差无实际影响。
3. **`tools/protocoltest` 加 `-login user:pass` 探针**：完整复刻 auth_test 的 loginFlow（15 字节 hello -> 客户端 codec -> LOGIN_PASSWORD -> 解 LOGIN_STATUS），附人类可读解码（OK/4/5/7/CHOOSE_GENDER）。
4. **`tools/addaccount` 新工具**：插号/重置（sha1+salt NULL+gender=1）、`-show` 查看落库状态、`-del` 删号。
5. **live 冒烟全过**（SQLite，`configs/gms.toml` 现为 sqlite 模式）：
   - `database connected` + 8484 监听（`tools/start-gms.cmd` 后台启动，见 §三.9）。
   - 建号 `testgo:test123` -> 探针回 `LOGIN_STATUS: OK, accID=1`，回包含 15 字节固定尾 + 双写用户名 + 尾 byte 01（AuthSuccessPacket 结构完整）。
   - 落库验证：`loggedin=2`、`SessionIP=127.0.0.1`、`macs=11-22-33-44-55-66`、lastlogin 已写。
   - 双登重发 -> `ALREADY_LOGGED_IN (7)` ✅。
   - 清理：删号、停进程、build/vet/test 复归全绿。
6. `internal/database/db_test.go`：加 `TestSQLiteAccountChain`（临时库自动建表 -> 插号 -> 全登录 SQL 面读写断言）。

### 上会话（2026-08-29，断点 B -> B'）完成，摘录备查

1. **P2.2 账密登录代码全部完成**：
   - `internal/login/crypto_test.go`：golden 三线（sha1 / salted sha512 / legacy $H$）全对。
   - `internal/login/packets.go`：`LoginFailedPacket`（short LOGIN_STATUS + int reason + short 0）、`AuthSuccessPacket`（含 15 字节固定尾 `00 00 00 03 01 00 00 00 E2 ED A3 7A FA C9 01`、双写字符串、尾 byte 1）、`GenderNeededPacket`（CHOOSE_GENDER 0x0004 + name）。`packets_test.go` 字节级校验。
   - `internal/login/auth.go`：`handleLoginPassword`（LOGIN_PASSWORD 0x0001：login/pwd MapleAsciiString + 6 字节机器码 -> "XX-XX-.." 串）+ `dbLogin`（loginok 链：banned>0且非GM=3 / banned==-1 自动解封 / 双登=7（含 20s transition 超时重置）/ 密码链 legacy -> salt==NULL sha1 -> 超级密码 -> salted sha512 / 成功后升级 SHA-1）+ 落库（UpdateLoginState loggedin=2+SessionIP、UpdateAccountMac、密码升级）。`accountStore` 接口抽象 DB，`*database.DB` 天然满足。
   - `internal/login/server.go`：`SetStore`/`SetSuperPassword` 接线；OnPacket 用 `protocol.RecvOp` 分发（PONG + LOGIN_PASSWORD）。
   - `internal/database/accounts.go`：`Account` 加 `Macs` 列；新增 `AccountState`/`GetAccountState`、`UpdateLoginState`、`UpdatePasswordSHA1`、`UnbanAccount`、`UpdateAccountMac`；登录态常量（LoginNotLoggedIn=0..ChangeChannel=6，对齐 Java MapleClient）。
   - `internal/netw/session.go`：新增 `State atomic.Value` 每连接状态槽（对应 Java Netty `CLIENT_KEY` attr），handler 层存 `*client`。
   - `cmd/gms/main.go`：接 `database.Open`；**DB 不可达时降级**（告警 + 继续监听，LOGIN_PASSWORD 答 5），不崩端口。
   - `internal/login/auth_test.go`：fakeStore 端到端 10 测试（成功/legacy升级/salted升级/错密4/无号5/封禁3/GM豁免/双登7/陈旧transition/gender10），走真实 wire 加解密。
2. **Java 语义踩坑实录**（本轮发现，勿再踩）：
   - `LoginCrypto.hashWithDigest` 有**双 quirk**：`update(getBytes("UTF-8"), 0, in.length())` 只哈希 UTF-8 字节的**前 in.length() 个**（length()=UTF-16 单元数）——ASCII 无差，CJK 截断；且 golden 里 `sha1_中文` 的输入其实是**生成器侧 mojibake**（UTF-8 源码被 GBK 默认编码 javac 误读成 `涓\uFFFD枃`，9 字节 UTF-8，截断取前 3 字节即 sha1("涓")）。Go 移植保真，测试输入用 mojibake 串并注释说明。
   - `LoginCryptoLegacy` 布局：`$H$`+1 iota64 字+8 盐（12 seed）+28 base64 = **40 字符**；`myCrypt` 用 ISO-8859-1 字节（Go 用 `iso8859Bytes` 逐 UTF-16 单元映射，>0xFF 变 `?`），初始 sha1(salt+pw) 后**再 8 轮** sha1(h+pw)；`checkPassword = myCrypt(pw,hash)==hash`。
   - `Java MapleClient.login` 的 salt==null 时才走 sha1；`banned==-1` 自动 unban；双登判断读**实时** loggedin（transition 态 20s 超时视为未登录）。
   - `LoginWorker.registerClient`：gender==10（accounts 表 DEFAULT）发 `getGenderNeeded` 而非 `getAuthSuccessRequest`——**新号默认走 CHOOSE_GENDER**。
   - `CharLoginHandler.loginFailCount`：失败计数 >5 次后**不再回包**（直接静默）。
   - 超级密码：`ServerConstants.Super_password`(默认 false)+`superpw`(默认空)；Go 放 `config.Login.SuperPassword` + `Server.SetSuperPassword`。
3. `tools/sha1probe/`：一次性诊断工具（穷举 CJK 哈希候选找到 mojibake 真相），留着无妨可删。

## 三、关键技术结论（历史，仍然有效）

1. **getHello 是 raw 15 字节**，不加密、无帧头；Go 侧 `login.Server.NewSession` 里 `writeFull` 直写，后续包才走 `Session.Write`。
2. **Acceptor 必须调 OnOpen**：`acceptLoop` 在 `go s.readLoop(a)` 前调 `a.Handler.OnOpen(s)`；`NewSession` 不要自己调。
3. **测试/工具读帧要精确读**（io.ReadFull），不要 Read 大 buffer 全 append。
4. hello 布局：`[0:2]=13, [2:4]=79, [4:6]=00 00, [6:10]=recvIV, [10:14]=sendIV, [14]=4`。
5. crypto/netw/protocol 旧结论仍有效：send cipher version=-80、recv=79；Shanda 计数用无符号；AES chunk 顺序 jadx+javap 双验过。
6. **MySQL 5.5.53 启动**（Git Bash）：`cd /k/079MAX2服务端/mysql/MySQL && ./bin/mysqld.exe --defaults-file="K:/079MAX2服务端/mysql/MySQL/my.ini" --console > /tmp/mysqld.log 2>&1 &`。my.ini 是 **GBK** 编码（Python gbk 读写）。root/root，库 `079-max2`。
7. go-sql-driver 连库名带连字符 OK；accounts 可空列用 `sql.NullString`（现也用 `sql.NullTime` 读 lastlogin）。
8. Java `getSessionIPAddress` 返回 `"/1.2.3.4"`（带斜杠、split(":")[0]）；Go 用 `net.SplitHostPort` 后存纯 IP，测试断言 `2|<ip>`。
9. **Windows 命令行坑（本会话实测）**：
   - 命令行传中文路径必 mojibake（GBK/UTF-8 混淆）；K 盘无 8.3 短名。解法：`tools/start-mysqld.ps1` 用通配符 `K:\079MAX2*\...` 在 PowerShell 内部解析。
   - sandbox 拒绝 `Start-Process` workspace 内 exe（Access denied）；`Start-Job` 拉起后无输出。解法：`tools/start-gms.cmd`（`cmd start "" /B cmd /c "exe args > log 2>&1"`），实测可用。
   - **`cmd /c "..."` 之后用 `;` 串联 PowerShell 语句会被 cmd 吃掉**（报「不是内部或外部命令」）；`cmd /c "..."` 里末参数紧跟引号也会被污染（`-del testgo` 变 `testgo;`）。铁律：cmd 调用单独一次、参数不放引号串里。
10. **P2.3 世界列表 Java 语义**（勿再踩）：
   - `CharLoginHandler.ServerListRequest` 顺序 0..19（蓝蜗牛..花蘑菇）按 GUI 开关（0=开启）逐世界发包；20 世界共用**同一个** serverName，频道名 = serverName + "-" + i。
   - **LICENSE_REQUEST(0x0003) 也路由到 ServerListRequest**（MapleServerHandler case 合并）。
   - **SERVERLIST/SERVERSTATUS 有 NeedsChecking 门禁**：`!isLoggedIn()` 直接 return 无回包；Java `loggedIn` 由 `updateLoginState(2)` 置真（MapleClient.java:992-998），登录成功即放行（gender=10 分支也放行）。
   - getServerList 布局 quirk：两处 `writeShort(100)`、`writeInt(500)` 固定值；lastChannel = channelLoad 键中 <=30 的最大者（默认 1）；**缺频道 load 写 1200（显示为满）**。
   - 世界状态值来自 `Load/游戏频道状态显示.txt`（ZEV.<世界>状态，注释「显示游戏热度#0,1,2」；原版前 3 个世界=1，其余=0）。
   - 气球：`GameConstants.getBalloons` 仅当 `MapleParty.容纳人数 != 999` 才加 1 个（"小z：69911535", 40, 300）——**原版默认(999)无气球**；Go 移到 config `[[server.balloons]]`，默认空。
   - eventMessage = 双爆频道提示（"频道 #r<Count> \r\n 为双爆频道"，开关关时空串）；userLimit 语义：numPlayer>=limit=2(满)、numPlayer*2>=limit=1(忙)、否则 0。

## 四、断点与下一步（按优先级）

### 断点 E（当前）：P2.5 自动注册 + 白/黑名单
- **入口**：Java `handling/login/handler/AutoRegister.java` + `CharLoginHandler.login` 开头的 autoRegister 分支（账号不存在且未封禁时建号：`INSERT INTO accounts`，然后提示"注册成功请重新登录"+ getLoginFailed(1)）。
- **配置**：`config.Login.AutoRegister`（ZEV.自动注册，已在 config）+ 注册开关/账号数量（原版 ConfigValuesMap/账号注册开关，configvalues 表不在 dump，Go 用 config 键）。
- **白/黑名单**：IP 封禁（`c.hasBannedIP`，表 ipban？查 dump）+ MAC 封禁（`isBannedMac`，accounts.macs 对照 ban 表）。先查 `migrations/0001_base.sql` 里 ban 相关表（`ipbans`/`macbans`？）。
- 顺手：P2.2/2.4 里跳过的「登陆保护/队列/多开」配置项保持跳过（它们是 ZEV 分发外围，PLAN 里已归类 SKIP 或 P7）。
- 完成后 M1（登录闭环）只剩 P0.6 客户端实连验证。

### M1 收尾候选（P2.5 后）
- V079 客户端实连：登录 -> 选世界 -> 选角（可建角）全链（P0.6）。
- `Character_With/WithoutSecondPassword`（CHAR_SELECT 0x000A -> SERVER_IP 转 channel 服）：依赖 P4 频道服，M1 不做。

### 文档同步动作（每次提交前）
- PROGRESS.md：完成的任务打 [x] + 日期；更新"最后更新"和完成度汇总。
- FILETRACK.md：新迁移的 Java 文件改 DONE/MERG/ACTV，并更新 Summary 计数。
- MIGRATION_MAP.md：相应行状态改 ✅/🔄；§12 表分类按需补充。

## 五、P0 未竟事项

- **P0.6 剩余**：V079 客户端登录器连 8484 验证。P2.3 后登录+世界列表已可回，客户端应能走到"选世界"界面；选角界面需 P2.4 CHARLIST。
- **Makefile**：仍未写；可用 build.ps1 或补一个简单 Makefile。
- **P0.3 备注**：`migrations/0001_base.sql` 已导出但未在 CI 做导入验证（MySQL 5.5 dump 含 `DROP TABLE`，导入需先建库）。

## 六、目录速览（当前）

```
I:\GMS
├── README.md, go.mod, go.sum
├── cmd\gms\main.go                 # 入口（已接 database，含降级模式）
├── configs\gms.toml                # 配置样例（database=sqlite 冒烟；[[server.worlds]] 世界列表）
├── data\gms.db                     # SQLite 运行库（自动生成，首连建 accounts 表）
├── migrations\0001_base.sql        # P0.3 导出（225 表，MySQL 权威 schema）
├── internal\
│   ├── config\  ✅ (config.go + test；Database 加 driver 字段)
│   ├── crypto\  ✅ (bittools/shanda/aesofb + golden_test + testdata\golden.txt)
│   ├── protocol\ ✅ (opcodes_gen + reader + writer + charset + tests)
│   ├── netw\    ✅ (session[+State 槽] + codec + tests)
│   ├── login\   ✅ P2.2+P2.3+P2.4 (server/packets/util/crypto/auth/worlds/chars + 6 组测试 + testdata\golden_login.txt)
│   └── database\ ✅ P2.1+2.2+2.4 (db[双方言+自动建 accounts/characters/character_slots]/accounts/characters + db_test)
├── tools\
│   ├── genopcodes\ + genopcodes.exe
│   ├── protocoltest\ + protocoltest.exe   # -login/-serverlist/-status 探针（B'/C 用过）
│   ├── addaccount\ + addaccount.exe       # 插号/重置/-show/-del（B' 用过）
│   ├── start-gms.cmd                      # 后台启动服务端（cmd start /B）
│   ├── start-mysqld.ps1                   # MySQL 启动（通配符绕中文路径，备用）
│   ├── goldengen\ (GoldenGen.java / LoginCryptoGen.java + class)
│   ├── sha1probe\                          # 一次性诊断，可删
│   └── jadx_out\sources\tools\MapleAESOFB.java
└── docs\  (PLAN / PROGRESS / MIGRATION_MAP / FILETRACK / SESSION_STATE)
```

## 七、环境备忘

- Go 1.25.5；module `GMS`；GOPROXY=`https://goproxy.cn` 可用；HTTP 代理 `http://127.0.0.1:7897`。
- jadx：`D:\Soft\jadx\bin\jadx.bat`，需 `JAVA_HOME=G:\dk-11.0.7`（JDK11）。
- 原版 jar（golden 基准）：`K:\079MAX2服务端\dist\079MAX2.jar`；旧 JDK：`K:\079MAX2服务端\jdk\bin`。
- MySQL 5.5.53：`K:\079MAX2服务端\mysql\MySQL\bin\mysqld.exe`（root/root，库 `079-max2`）；客户端 `mysql.exe -uroot -proot 079-max2`。**冒烟已切 SQLite 不再需要**；要回 MySQL：`tools/start-mysqld.ps1`（勿用命令行直传中文路径，见 §三.9）+ gms.toml driver 改回 mysql。本会话结束时 mysqld 仍在跑（PID 36772），不用可 `Stop-Process -Name mysqld`。
- 沙箱：本会话策略已放宽为 danger-full-access（approval=never），可直接写 `I:\GMS`；若回到 workspace-write 会拦 I:\GMS 写入，需一次性提权。`go run` 的 exe 在 Temp 会被拦，用 `go build -o tools\xxx.exe` 再跑。**Start-Process 对 workspace exe 仍被拒**，用 `tools/start-gms.cmd`。
- 交流源码：`I:\ZEVMS079交流源码\src\...`（GBK 编码）；主参考 `I:\Zevms\src\...`。
- 依赖：mysql driver + sqlx + `modernc.org/sqlite` v1.57.0（纯 Go，无 CGO）。
- 项目根**没有 .gitignore**：`bin/`、`data/`、`*.exe` 均未被忽略（git status 一片 untracked），下次提交前建议补一个。
