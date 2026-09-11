# 代码映射表（MIGRATION_MAP）

> Java 原件 -> Go 目标件的对照与状态。P0 建骨架、随做随标。
> 状态：⬜ 未开始 | 🔄 进行中 | ✅ 完成 | ❌ 不迁移（注明替代）
> 原则：反编译乱码名（如 `灏佸寘鏄剧ず`）一律在 Go 侧用语义名（`PacketDisplay`），对照交流源码恢复。

## 1. 网络与协议（-> internal/netw, crypto, protocol, packet）

| Java 原件 | 行数 | Go 目标 | 状态 | 备注 |
|---|---|---|---|---|
| handling/netty/ServerConnection 等 4 件 | 295 | netw/acceptor.go, session.go | ✅ | Netty->net+goroutine |
| tools/MapleAESOFB.java | 130 | crypto/aesofb.go | ✅ | J1 golden 对拍 |
| tools/MapleCustomEncryption.java | 74 | crypto/shanda.go | ✅ | 双向变换 |
| handling/RecvPacketOpcode + recv.properties | 238 | protocol/opcode_gen.go | ✅ | P0.4 生成 155 |
| handling/SendPacketOpcode + send.properties | 317 | protocol/opcode_gen.go | ✅ | P0.4 生成 245 |
| tools/data/*（LEAccessor 5 件） | 551 | protocol/reader.go, writer.go | ✅ | |
| tools/MaplePacketCreator.java | 5826 | packet/*.go（按域拆 10+ 文件） | 🔄 | P2 起按需逐域搬；P4.2 建 `internal/packet`：getCharInfo(WARP_TO_MAP)/temporaryStats_Reset/serverMessage/AddCharStats 共享；P4.3 加 spawnPlayerMapobject(SPAWN_PLAYER)/removePlayerFromMap + addRingInfo(List)/addMarriageRingLook；P4.4 加 movePlayer(MOVE_PLAYER 0x00BB) 与 spawn 的真实 pos/stance |
| tools/packet/*（10 件） | 3815 | packet/ 对应域 | 🔄 | LoginPacket 12 函数 + PacketHelper addCharStats/addCharLook（P2.2-P2.4）；P4.1 加 getServerIP（选角交棒 SERVER_IP）与 enableActions（UPDATE_STATS 空掩码）；P4.2 把 addCharStats/getTime/addCharacterInfo 提到 `internal/packet`（login+channel 共享），login 侧 addCharEntry 改为调用；P4.3 addCharLook 也提到 `internal/packet`（带 mega 参数：login=true、频道 spawn=false）；P4.4 serializeMovementList → movement.SerializeMovementList |
| handling/MapleServerHandler.java | 1298 | login|channel/cashshop handler 注册表 | 🔄 | P2.2 已迁 hello+LOGIN_PASSWORD/PONG 分发；P2.3 已迁 SERVERLIST/LICENSE/SERVERSTATUS 分发；P4.1 频道分支（同款 IV/hello + channel>0 的 isShutdown 门禁）；P4.2 频道侧 PLAYER_LOGGEDIN 实派（handlePlayerLoggedIn）+ OnClose 注销；P4.4 MOVE_PLAYER(0x24) 派发 handleMovePlayer |

## 2. 登录与账号（-> internal/login, database）

| Java 原件 | Go 目标 | 状态 | 备注 |
|---|---|---|---|
| handling/login/*（6 件 1115 行） | login/server.go | ✅ | P1 冒烟（握手/PING）；P2 补账密/选角；P4.1 补选角交棒（SERVER_IP + loginAuth 票据） |
| handling/login/handler/CharLoginHandler | login/auth.go+worlds.go+chars.go+select.go+register.go | 🔄 | P2.2 已迁 login()；P2.3 已迁 ServerList/ServerStatus；P2.4 已迁 Charlist/CheckCharName/CreateChar/DeleteChar/SetGender；P2.5 已迁自动注册分支 + IP/MAC 封禁门禁；P4.1 已迁 Character_WithoutSecondPassword（0x000A）；Character_WithSecondPassword 在 v079 recvops 无 opcode（-2）不迁 |
| handling/login/LoginServer.loginAuth/loginIPAuth | world/registry.go | ✅ | P4.1：charId→票据(ip/tempIp/channel) + ip 集合，put/get(取走)/contains/remove/add 全迁；ZEV 原版频道侧那个 containsIPAuth 判断是空 if（不拦），Go 保真不拦 |
| handling/login/LoginWorker.load 换算 | login/worlds.go displayChannelLoad | ✅ | P4.1：factor=1200*频道数/userLimit、min 1200 上限；Go 每次回包按真实人数换算（原版是把换算值就地写回共享 map，下一轮会二次换算） |
| client/MapleClient.unlockAcc | login/server.go | ✅ | P4.1 双登清理：有活跃会话→弹窗+关连接（对齐 unLockDisconnect 的 1s 延迟）；无活跃会话→accounts.loggedin 置 0（Java else 分支），本次仍答 7 |
| handling/login/handler/AutoRegister | login/register.go | ✅ | P2.5：ZEV.自动注册 + createAccount（同机器码 ≤100）；IP/MAC 封禁见 client/MapleClient 行 |
| client/MapleClient.hasBannedIP / isBannedMac | login/register.go + database/bans.go | ✅ | P2.5：ipbans 前缀匹配 / macbans 精确匹配（含零 MAC 与长度≠17 豁免）；表结构出自 migrations/0001_base.sql:1822/1911 |
| client/LoginCrypto(+)Legacy | login/crypto.go | ✅ | SHA+盐，golden 对拍（含 2 个 Java quirk） |
| gui.ZEVMS2 中 wzpath/启动装配段 | cmd/gms/main.go | ✅ | 只取装配语义 |
| database/DatabaseConnection(.1) | database/db.go | ✅ | GORM 池替代线程绑定连接（断点 I 从 sqlx 迁移） |

## 3. 世界与频道（-> internal/world, channel）

| Java 原件 | Go 目标 | 状态 | 备注 |
|---|---|---|---|
| handling/world/*（21 件 4731 行） | world/*.go | 🔄 | P4.1：World.Find 子集（register/forceDeregister/find/findByName）+ LoginRegistry；跨频道广播/组队/好友/公会/家族待 P8 |
| handling/channel/*（34 件 13967 行） | channel/*.go + channel/handler/*.go | 🔄 | P4.1：ChannelServer 骨架（端口 7574+channel、多频道实例、hello、load 上报）+ PlayerStorage（nameToChar/idToChar + World.Find 副作用）；P4.2：Loggedin2 进图链（login.go）、PlayerStorage 的 CharacterTransfer 挂起表、ChannelServer 的地图注册表/addPlayer/forceRemovePlayerByAccId、INTERSERVER 部分 → `internal/packet`+`internal/mapp`；P4.3：spawn/despawn 广播（mapp.Player.SendSpawnData/DespawnData）；P4.4：PlayerHandler.MovePlayer → channel/movement.go（MOVE_PLAYER 解析/广播/坐标落地）；事件/商店/PersistingTask/地图内容待 P4.5+ |
| handling/cashshop/*（3 件 1447 行） | cashshop/*.go | ⬜ | MTS 可裁剪 |
| server/Timer.java | internal/scheduler.go | ⬜ | Ticker+context |
| server/ShutdownServer | cmd 关停钩子 | ⬜ | |

## 4. 领域模型（-> internal/model）

| Java 原件 | Go 目标 | 状态 | 备注 |
|---|---|---|---|
| client/MapleCharacter.java | model/character/*.go（拆 5+ 文件） | 🔄 | P2.4 getDefault/saveNewCharToDB 子集；P4.2 loadCharFromDB 的 characters 行 + `channel.Player` 过渡结构（`CHR`/`MapID()`/`AccountID()`）；8828 行本体拆属性/存档/技能/任务/社交待 P5.1 |
| client/inventory/*（18 件 2724 行） | model/inventory/*.go | ⬜ | |
| client/PlayerStats, Skill, SkillFactory | model/stats.go, skill.go | ⬜ | J6 对拍 |
| client/messages/*（12 件 6466 行） | channel/command/*.go | ⬜ | GM 指令 |
| client/anticheat/*（4 件 507 行） | anticheat/*.go | ⬜ | |
| client/BuddyList, MapleKeyLayout, MonsterBook 等 | model/*.go | ⬜ | |

## 5. 地图与生命体（-> internal/mapp, life）

| Java 原件 | Go 目标 | 状态 | 备注 |
|---|---|---|---|
| server/maps/*（35 件 8636 行） | mapp/*.go | 🔄 | P4.2：`mapp.Map` 最小实例（id + 玩家集合 + `Player` 接口 ObjectID）+ `ChannelServer.getMapFactory().getMap(id)` 惰性注册；P4.3：spawn/despawn 视野广播（`Map.AddPlayer` 对同图其他人 + 新人对每个老玩家 + 自身；`Map.RemovePlayer` 广播 removePlayerFromMap；`Map.Broadcast(pkt, except)`）+ `MapleMapObject.sendSpawnData` → `Player.SendSpawnData`；P4.4：`AnimatedMapleMapObject` 的 setPosition/setFh/setStance → `movement.Target`（`channel.Player` 实现），`MapleMap.movePlayer` 的坐标落地由 `Player.ApplyMovement` 承担；地图内容（MapleMapFactory 的 info/foothold/life/portal）与 view-range 过滤（Point 重载）待 P4.3b/P6 |
| server/life/*（23 件 3602 行） | life/*.go | ⬜ | 怪物/NPC/反应堆 |
| server/movement/*（4 件 150 行）+ handling/channel/handler/MovementParse | movement/*.go | ✅ | P4.4：`Fragment`（= StaticLifeMovement，本树唯一实现）+ `Parse`/`Serialize`/`SerializeMovementList`/`UpdatePosition`；保真两个怪癖（NewFh=构造器第 5 参；3/4/7/8/9/11 组丢弃 duration） |
| server/custom/respawn | mapp/spawner.go | ⬜ | |

## 6. 战斗与效果（-> internal/combat）

| Java 原件 | Go 目标 | 状态 | 备注 |
|---|---|---|---|
| handling/channel/handler/DamageParse, AttackInfo | combat/attack.go | ⬜ | |
| server/MapleStatEffect（1773 行） | combat/effect.go | ⬜ | |
| client/MapleBuffStat 等值持有器 | combat/buff.go | ⬜ | |
| abc/吸怪检测、检测全屏 | anticheat/suck.go, fullscreen.go | ⬜ | |

## 7. wz 数据（-> internal/wzs, tools/wzdump）

| Java 原件 | Go 目标 | 状态 | 备注 |
|---|---|---|---|
| provider/WzXML/*（8 件 563 行） | wzs/type.go + xml.go + provider.go | ✅ | P3.1：读 WzXML 解包（读的是与 Java 同一份 XML，见注 1） |
| provider/MapleData* 接口族 | wzs/data.go（Node/DirEntry/DataTool） | ✅ | P3.1：getChildByPath/getChildren/getData 语义保真 |
| String.wz 名字表（Cash/Consume/Eqp/Etc/Ins/Pet/Map/Mob/Npc/Skill + Etc.wz ForbiddenName） | wzs/names.go Names | ✅ | P3.3 首片：按 id 建索引（Java 按计算路径查，本服 Skill.img 有 8 位键、Map.img 无 china 区） |
| server/MapleItemInformationProvider | wzs/itemdata.go | ⬜ | 1459 行 |
| server/life/MapleLifeFactory, maps/MapleMapFactory | wzs/mapfactory.go, life/factory.go | ⬜ | |
| tools/wztosql/MonsterDropCreator.java（887 行） | internal/dropgen + tools/wztosql | ✅ | P3.5：仅"wz 侧重建 + 漂移对比"，**不写库**（本服掉落是第三方预置，库为准） |
| tools/wztosql/WzStringDumper.java, DumpMobSkills.java, AddCashItemToDB.java | - | ⬜ | 239/154/31 行，P3.5 未做（掉落之外的导出工具） |
| server/life/MapleMonsterInformationProvider | internal/life/drops.go | ✅ | P3.5：掉落表缓存（EQUIP chance/3、出错不缓存、Go 加 RWMutex 修 Java 数据竞争） |
| server/life/MapleMonsterInformationProvider 读的表 | internal/database/drops.go + migrations/0002_drop_data.sql | ✅ | P3.5：DAO + 权威库快照（14,259 行），SQLite 空表时自动回放 |
| tools/dropexport（Go 新增，无 Java 对应） | tools/dropexport | ✅ | P3.5：把权威 079-max2 掉落表导出成可移植 SQL |
| client/SkillFactory | wzs/skilldata.go | ⬜ | |

> 注 1（2026-08-30 决策）：J3 原定"自研二进制 wz reader + 与 Java XML 输出 diff"。
> 实测 K:\079MAX2服务端\wz 下的 16 个 *.wz 都是**目录**（HaRepacker/WzXML 解包，39,986 个 XML，735 MB），
> 即 Java 服本身读的就是这份 XML，Go 读同一份数据即天然与 Java 一致，无需 diff。
> 解包已复制到 `I:\GMS\wz`（.gitignore 忽略）。真二进制 wz 在 `J:\079MAX2客户端\*.wz`（v079 客户端，约 3.9 GB）、
> `M:\MapleStory\萌萌Pro\*.wz`；若将来需要自给自足（不依赖外部解包），再按 J3 主线补二进制 reader，
> 接口已按 provider.MapleData 抽象，可插拔。

## 8. 脚本系统（-> internal/script, tools/scriptlint）

| Java 原件 | Go 目标 | 状态 | 备注 |
|---|---|---|---|
| scripting/NPCConversationManager.java | script/host/cm.go | ⬜ | 9729 行 API 面，最小充分集 |
| scripting/AbstractPlayerInteraction.java | script/host/interaction.go | ⬜ | 5877 行 |
| scripting/EventManager / EventInstanceManager | script/host/em.go | ⬜ | 1079 行 |
| scripting/NPCScriptManager, Portal*, Reactor* | script/manager.go | ⬜ | |
| scripting/AbstractScriptManager | script/loader.go | ⬜ | GBK 转码/缓存 |
| scripting/EncodingDetect.java（3973 行） | tools/scriptlint/charset.go | ⬜ | 只取探测逻辑 |
| K: scripts/（3922 个 js） | data/scripts（原样搬运+转码） | ⬜ | P7 批量处理 |

## 9. 商业与自定义玩法（-> internal/custom）

| Java 原件 | Go 目标 | 状态 | 备注 |
|---|---|---|---|
| server/MapleShop, MapleShopFactory, shops/* | custom/shop/*.go | ⬜ | |
| server/custom/auction + auction1（合并） | custom/auction/ | ⬜ | 单实现 |
| server/custom/bankitem + 1 + 2（合并） | custom/bank/ | ⬜ | 单实现 |
| server/custom/bossrank 1..10（合并） | custom/rank/ | ⬜ | 实例化 10 配置 |
| pvp/*（3 件 483 行） | custom/pvp/ | ⬜ | |
| folder/副本相关.js + MapleCharacterUtil副本 | custom/dungeon/ | ⬜ | |
| server/custom/forum, capture, treasure_house | custom/forum/, ... | ⬜ | 可裁剪 |
| gui/活动*（魔族攻城/OX/喜从天降等 12 件） | custom/events/*.go | ⬜ | 活动调度框架化 |
| server/MTSStorage, CashShop, CashItemFactory(A) | cashshop/mts.go（可裁） | ⬜ | A 后缀重复系合并 |
| fumo/*（锻造/洗魔/鉴定） | custom/fumo/ | ⬜ | |
| 充值卡后台（gui）+ K: 充值卡库存格式 | custom/redeem/ + admin API | ⬜ | P10 |

## 10. 配置/常量/工具（-> internal/config, tools）

| Java 原件 | Go 目标 | 状态 | 备注 |
|---|---|---|---|
| constants/ServerConstants + GameConstants + MapConstants | config/consts.go | ⬜ | 乱码名对照交流源码 |
| constants/OtherSettings + abc/OtherSettings2 | config/settings.go | ⬜ | 合并 |
| server/ServerProperties + K: Load/*.ini | config/gms.toml | ⬜ | ZEV.* 键映射表 |
| tools/FilePrinter, FileoutputUtil | slog 封装 | ⬜ | |
| tools/Pair/Triple/ArrayMap 等 | 标准库/泛型替代 | ⬜ | 不直接迁移 |
| tools/Randomizer | internal/rng.go（可注入） | ⬜ | J6 |
| tools/HexTool, StringUtil, DateUtil, KoreanDateUtil | packet/util.go | ⬜ | 按需收编 |
| abc/关键字屏蔽, 屏幕关键字, 注册白/黑名单, 拍卖行限制 | anticheat/, config/ | ❌/⬜ | **关键字屏蔽两个类是死代码，不迁**（jar 字节级查证：全 jar 无引用、ini 路径写错、`关键字屏蔽.ini` 不存在 → 原版聊天无任何过滤，见 PROGRESS GMS-P4.5）。注册白/黑名单已由 P2.5 的 `login/register.go` 覆盖；拍卖行限制随 P8 |

## 11. 不迁移清单（❌，替代方案见 PLAN 3.2）

| Java 原件 | 行数 | 替代 |
|---|---|---|
| gui/* 全部 Swing（控制台/图片/网关/Jieya/通信等） | ~45k | Web 面板（P10） |
| gui/QQMsgServer, KuQ 对接 | ~3.4k | webhook 桩（P10.4 可选） |
| download/, a/http/, com/进度条1, abc/www | ~1.5k | 无（部署脚本替代） |
| com/cyb（CPU 监控 demo） | 233 | pprof 替代 |
| database1/, Class1/2/3, 脚本编辑器, JComboBoxDemo1 | ~1.7k | 无（重复/演示代码） |
| zevms/data/DataPack, module/system | 69 | 无（旧启动器残留） |
| tools/CPUSampler, Eval, MockIOSession | 972 | pprof / goja REPL 替代 |

## 12. 数据库表分类（P0.3 已落地：migrations/0001_base.sql，225 张表）

- **A 原生核心（70 张）**：`accounts`、`accounts_info`、`achievements`、`alliances`、`auth_server_channel`、`auth_server_channel_ip`、`auth_server_cs`、`auth_server_login`、`auth_server_mts`、`bbs_replies`、`bbs_threads`、`buddies`、`characters`、`configvalues`、`csequipment`、`csitems`、`drop_data`、`drop_data_global`、`dueyequipment`、`dueyitems`、`dueypackages`、`eventstats`、`famelog`、`families`、`gifts`、`guilds`、`hiredmerch`、`hiredmerchequipment`、`hiredmerchitems`、`inventoryequipment`、`inventoryitems`、`inventorylog`、`inventoryslot`、`ipbans`、`keymap`、`macbans`、`macfilters`、`mail`、`map`、`monsterbook`、`mountdata`、`mts_cart`、`mts_items`、`mtsequipment`、`mtstransfer`、`mtstransferequipment`、`mulungdojo`、`notes`、`nxcode`、`pets`、`playernpcs`、`playernpcs_equip`、`questactions`、`questrequirements`、`queststatus`、`queststatusmobs`、`reactordrops`、`regrocklocations`、`reports`、`rings`、`savedlocations`、`shopitems`、`shops`、`skillmacros`、`skills`、`skills_cooldowns`、`speedruns`、`storages`、`trocklocations`、`wishlist`
- **B ZEVMS 自定义（113 张）**：`会员`、`地图吸怪检测`、`家具`、`广播信息`、`强化会员`、`技能范围检测`、`是否是认证玩家`、`游戏npc管理`、`物品摆放`、`疯狂乱斗`、`白名单`、`禁用脚本地图`、`管理员指令记录`、`钓鱼物品`、`锻造材料表`、`锻造物品表`、`aclog`、`admin`、`auctionitems`、`auctionpoint`、`auth_ip`、`awarp`、`bank`、`bank_item`、`bank_jl`、`blocklogin`、`bosslog`、`bossrank`、`caipiao`、`capture_cs`、`capture_jl`、`capture_zj`、`capture_zk`、`cashshop_limit_sell`、`cashshop_modified_items`、`character_mobkilledcount`、`character_slots`、`cheatlog`、`delay`、`divine`、`fengye`、`fishing_rewards`、`forum_reply`、`forum_section`、`forum_thread`、`fubenjilu`、`fullpoint`、`game_poll_reply`、`gmlog`、`guiidld`、`hddyzbs`、`hdoxdtbs`、`hdtxqbs`、`hdxgdbs`、`hire`、`hiredmerchantshop`、`htsquads`、`hypay`、`invitecodedata`、`ipvotelog`、`jiezoudashi`、`kimob`、`loginlog`、`mapidban`、`mxmxd_fumo_info`、`mxmxd_gainexpmonster_logs`、`mxmxd_mapkilledcount`、`mxmxd_qianneng_info`、`mxmxd_qq_ip`、`ox`、`oxdt`、`personal`、`pipei`、`pnpc`、`prizelog`、`pvp`、`pvpskills`、`qiandao`、`qianneng`、`qllog`、`qqlog`、`qqstem`、`qqstem_caiquan`、`questinfo`、`readable_cheatlog`、`readable_last_hour_cheatlog`、`rebirth`、`robot`、`saiji`、`sell_goods`、`shangxianlaba`、`share`、`shares`、`shouce`、`skillsuper`、`skillsuper_pvp`、`strengthening`、`task_jl`、`task_liebiao`、`time`、`treasure_house`、`upgrade_career`、`uselog`、`warp`、`wz_customlife`、`wz_gateway`、`wz_mob`、`wz_mobskilldata`、`wz_oxdata`、`wz_weapon`、`z挖矿`、`z猜数字`、`zfuben`
- **C 废弃/重复（42 张）**：`auctionitems1`、`auctionpoint1`、`bank_item1`、`bank_item2`、`bosslogg`、`bossrank1`、`bossrank2`、`bossrank3`、`bossrank4`、`bossrank5`、`bossrank6`、`bossrank7`、`bossrank8`、`bossrank9`、`character7`、`charactera`、`characterz`、`configvalues_copy`、`drop_data_tixing`、`drop_data_vana`、`drop_data_vip`、`guildsl`、`hiredy`、`hirex`、`inventoryitems_zev`、`jiezoudashi2`、`kimob2`、`mtsitems`、`nxcodez`、`onetimeloa`、`onetimelob`、`onetimeloc`、`onetimelod`、`onetimeloe`、`onetimelog`、`questinfo2`、`regrocklocations_copy`、`time_copy`、`wz_testing`、`yinhang_1`、`yinhang_2`、`zaksquads`

> 分类口径：A 以 OdinMS v079 原生 schema 为基准；B 为 ZEVMS/079MAX2 扩展（含中文表与拍卖/银行/PVP/签到等）；C 为多套重复实现、旧后缀表与测试残留。

## 统计（随进度更新）

| 状态 | 条目数 |
|---|---|
| ✅ 完成 | 9 |
| 🔄 进行中 | 0 |
| ⬜ 未开始 | 57 |
| ❌ 不迁移 | 7 |
