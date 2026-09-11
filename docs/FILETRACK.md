# FILETRACK - file-level migration tracking

<!-- Legend: TODO=not started ACTV=in progress DONE=done (Go impl + tests) MERG=merged into other impl SKIP=not migrated (see Note) -->
<!-- One row per Java file under I:\Zevms\src\main\java. Update the Status column as work completes. -->

## Summary

| metric | value |
|---|---|
| java files total | 533 |
| to migrate (TODO+ACTV) | 397 |
| done (DONE) | 34 |
| merged (MERG) | 48 |
| skipped (SKIP) | 54 |

## pkg (root) - 1 files, 41 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| JComboBoxDemo1.java | 41 | SKIP | - | demo |

## pkg a - 3 files, 1154 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| a/本地数据库.java | 141 | SKIP | - | launcher misc |
| a/雇佣数据库函数.java | 49 | SKIP | - | launcher misc |
| a/用法大全.java | 964 | SKIP | - | launcher misc |

## pkg a\http - 2 files, 102 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| a/http/DownLoad.java | 59 | SKIP | - | downloader |
| a/http/数据库更新文件.java | 43 | SKIP | - | downloader |

## pkg abc - 16 files, 1873 lines -> internal/anticheat+config [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| abc/Game.java | 761 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/Game2.java | 303 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/OtherSettings2.java | 61 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/PNPC.java | 48 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/访问地区.java | 33 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/关键字屏蔽.java | 53 | SKIP | - | **死代码，不迁**：079MAX2.jar 全 2441 个 class 无任何引用（类名只在自己的 this_class 里出现）、`加载文件\关键字屏蔽.ini` 在部署中不存在、原版聊天无关键字过滤。见 PROGRESS GMS-P4.5 查证段 |
| abc/检测全屏.java | 68 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/拍卖行限制.java | 48 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/屏幕关键字.java | 48 | SKIP | - | **死代码，不迁**：同上（jar 无引用；ini 路径写成 `加载文件\加载文件\屏幕关键字.ini`，部署里没有这个嵌套目录）|
| abc/任务修复.java | 53 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/商城检测文件.java | 53 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/物品丢弃检测.java | 53 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/吸怪检测.java | 83 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/增加伤害的装备.java | 112 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/注册白名单.java | 48 | TODO | internal/anticheat+config | split: detect/config/util |
| abc/注册黑名单.java | 48 | TODO | internal/anticheat+config | split: detect/config/util |

## pkg abc\sancu - 3 files, 226 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| abc/sancu/DeleteFileUtil.java | 46 | SKIP | - | file-delete demo |
| abc/sancu/DeleteFileVisitor.java | 108 | SKIP | - | file-delete demo |
| abc/sancu/FileDemo_05.java | 72 | SKIP | - | file-delete demo |

## pkg abc\www - 4 files, 75 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| abc/www/BareBonesBrowserLaunch.java | 28 | SKIP | - | browser demo |
| abc/www/NewApp006.java | 16 | SKIP | - | browser demo |
| abc/www/Test.java | 15 | SKIP | - | browser demo |
| abc/www/TestHtml.java | 16 | SKIP | - | browser demo |

## pkg abc\yunxing - 3 files, 91 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| abc/yunxing/Multithread.java | 22 | SKIP | - | thread demo |
| abc/yunxing/ThreadAction.java | 43 | SKIP | - | thread demo |
| abc/yunxing/ThreadState.java | 26 | SKIP | - | thread demo |

## pkg client - 35 files, 16501 lines -> internal/model [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| client/BuddyEntry.java | 111 | TODO | internal/model |  |
| client/BuddyList.java | 194 | TODO | internal/model |  |
| client/CharacterNameAndId.java | 33 | TODO | internal/model |  |
| client/Class1.java | 117 | TODO | internal/model |  |
| client/Class2.java | 211 | TODO | internal/model |  |
| client/Class3.java | 211 | TODO | internal/model |  |
| client/CWvsContext.java | 8 | TODO | internal/model |  |
| client/DebugWindow.java | 1339 | TODO | internal/model |  |
| client/ISkill.java | 25 | TODO | internal/model |  |
| client/LoginCrypto.java | 74 | DONE | internal/login/crypto.go | P2.2 golden 对拍（含 length() 截断 quirk） |
| client/LoginCryptoLegacy.java | 129 | MERG | internal/login/crypto.go | $H$ 校验，golden 对拍 |
| client/MapleBuffStat.java | 219 | TODO | internal/model |  |
| client/MapleBuffStatValueHolder.java | 18 | TODO | internal/model |  |
| client/MapleCharacter.java | 8828 | ACTV | internal/model | P2.4 getDefault/saveNewCharToDB(characters 行)/deleteWhereCharacterId 子集入 internal/database/characters.go；P4.2 loadCharFromDB 的 characters 行读取 → database.GetCharacterByID + internal/channel 的 Player 过渡结构；本体(P5.1)未迁 |
| client/MapleCharacterUtil.java | 283 | ACTV | internal/login/chars.go | P2.4 canCreateChar/isEligibleCharName/getIdByName 已迁（namePattern/RESERVED） |
| client/MapleCharacterUtil副本.java | 270 | TODO | internal/model |  |
| client/MapleClient.java | 1717 | ACTV | internal/login/auth.go | P2.2 已迁 login 子集（login/finishLogin/updateLoginState/unban/updateMacs）；P2.4 已迁 loadCharacters/login_Auth/allowedChar/getCharacterSlots/deleteCharacter 子集；其余 P4+ |
| client/MapleCoolDownValueHolder.java | 14 | TODO | internal/model |  |
| client/MapleCSInventoryItem.java | 6 | TODO | internal/model |  |
| client/MapleDisease.java | 159 | TODO | internal/model |  |
| client/MapleDiseaseValueHolder.java | 18 | TODO | internal/model |  |
| client/MapleKeyLayout.java | 66 | TODO | internal/model |  |
| client/MapleLieDetector.java | 93 | TODO | internal/model |  |
| client/MapleQuestStatus.java | 145 | TODO | internal/model |  |
| client/MapleStat.java | 60 | TODO | internal/model |  |
| client/MonsterBook.java | 142 | TODO | internal/model |  |
| client/PlayerRandomStream.java | 96 | ACTV | internal/packet | P4.2：RandStream（CRand32 三元组 + connectData）；保真 connectData 三连取值相同的怪癖；伤害/命中三个流待 P6 |
| client/PlayerStats.java | 1143 | TODO | internal/model |  |
| client/RockPaperScissors.java | 71 | TODO | internal/model |  |
| client/Skill.java | 290 | TODO | internal/model |  |
| client/SkillEntry.java | 17 | TODO | internal/model |  |
| client/SkillFactory.java | 108 | TODO | internal/model |  |
| client/SkillMacro.java | 66 | TODO | internal/model |  |
| client/SummonSkillEntry.java | 9 | TODO | internal/model |  |
| client/脚本编辑器.java | 211 | TODO | internal/model |  |

## pkg client\about2 - 4 files, 361 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| client/about2/About.java | 133 | SKIP | - | swing about dialog |
| client/about2/MyBorder.java | 97 | SKIP | - | swing about dialog |
| client/about2/MyDongcLabel.java | 42 | SKIP | - | swing about dialog |
| client/about2/MyPicLabel.java | 89 | SKIP | - | swing about dialog |

## pkg client\anticheat - 4 files, 507 lines -> internal/anticheat [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| client/anticheat/CheatingOffense.java | 73 | TODO | internal/anticheat |  |
| client/anticheat/CheatingOffenseEntry.java | 53 | TODO | internal/anticheat |  |
| client/anticheat/CheatingOffensePersister.java | 44 | TODO | internal/anticheat |  |
| client/anticheat/CheatTracker.java | 337 | TODO | internal/anticheat |  |

## pkg client\inventory - 18 files, 2724 lines -> internal/model/inventory [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| client/inventory/Equip.java | 678 | TODO | internal/model/inventory |  |
| client/inventory/ExpTable.java | 54 | TODO | internal/model/inventory |  |
| client/inventory/IEquip.java | 51 | TODO | internal/model/inventory |  |
| client/inventory/IItem.java | 36 | TODO | internal/model/inventory |  |
| client/inventory/InventoryException.java | 13 | TODO | internal/model/inventory |  |
| client/inventory/Item.java | 209 | TODO | internal/model/inventory |  |
| client/inventory/ItemFlag.java | 22 | TODO | internal/model/inventory |  |
| client/inventory/ItemLoader.java | 551 | TODO | internal/model/inventory |  |
| client/inventory/MapleInventory.java | 228 | TODO | internal/model/inventory |  |
| client/inventory/MapleInventoryIdentifier.java | 106 | TODO | internal/model/inventory |  |
| client/inventory/MapleInventoryType.java | 53 | TODO | internal/model/inventory |  |
| client/inventory/MapleMount.java | 108 | TODO | internal/model/inventory |  |
| client/inventory/MaplePet.java | 267 | TODO | internal/model/inventory |  |
| client/inventory/MapleRing.java | 198 | TODO | internal/model/inventory |  |
| client/inventory/MapleWeaponType.java | 31 | TODO | internal/model/inventory |  |
| client/inventory/ModifyInventory.java | 47 | TODO | internal/model/inventory |  |
| client/inventory/PetCommand.java | 28 | TODO | internal/model/inventory |  |
| client/inventory/PetDataFactory.java | 44 | TODO | internal/model/inventory |  |

## pkg client\messages - 2 files, 244 lines -> internal/channel/command [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| client/messages/CommandProcessor.java | 169 | TODO | internal/channel/command |  |
| client/messages/CommandProcessorUtil.java | 75 | TODO | internal/channel/command |  |

## pkg client\messages\commands - 6 files, 2775 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| client/messages/commands/CommandExecute.java | 23 | TODO | internal/TBD |  |
| client/messages/commands/CommandObject.java | 26 | TODO | internal/TBD |  |
| client/messages/commands/活动管理.java | 10 | TODO | internal/TBD |  |
| client/messages/commands/玩家指令.java | 495 | TODO | internal/TBD |  |
| client/messages/commands/巡查管理.java | 153 | TODO | internal/TBD |  |
| client/messages/commands/游戏管理.java | 2068 | TODO | internal/TBD |  |

## pkg client\messages\commands\a - 4 files, 3447 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| client/messages/commands/a/AdminCommand.java | 3045 | TODO | internal/TBD |  |
| client/messages/commands/a/GMCommand.java | 196 | TODO | internal/TBD |  |
| client/messages/commands/a/InternCommand.java | 54 | TODO | internal/TBD |  |
| client/messages/commands/a/PlayerCommand.java | 152 | TODO | internal/TBD |  |

## pkg client\status - 2 files, 126 lines -> internal/model [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| client/status/MonsterStatus.java | 61 | TODO | internal/model |  |
| client/status/MonsterStatusEffect.java | 65 | TODO | internal/model |  |

## pkg client\zev - 1 files, 30 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| client/zev/MxmxdGainExpMonsterLog.java | 30 | SKIP | - | legacy leftover |

## pkg com - 1 files, 78 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| com/进度条1.java | 78 | SKIP | - | progressbar demo |

## pkg com\cyb - 1 files, 69 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| com/cyb/MonitorInfoBean.java | 69 | SKIP | - | cpu monitor demo |

## pkg com\cyb\dao - 2 files, 129 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| com/cyb/dao/IMonitorService.java | 8 | TODO | internal/TBD |  |
| com/cyb/dao/MonitorServiceImpl.java | 121 | TODO | internal/TBD |  |

## pkg com\cyb\test - 1 files, 21 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| com/cyb/test/test.java | 21 | TODO | internal/TBD |  |

## pkg com\cyb\util - 1 files, 14 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| com/cyb/util/Bytes.java | 14 | TODO | internal/TBD |  |

## pkg constants - 4 files, 2625 lines -> internal/config [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| constants/GameConstants.java | 2436 | TODO | internal/config | getBalloons 气球默认已并入 config（P2.3）；其余待 P5 |
| constants/MapConstants.java | 34 | TODO | internal/config |  |
| constants/OtherSettings.java | 61 | TODO | internal/config |  |
| constants/ServerConstants.java | 94 | DONE | internal/config |  |

## pkg database - 2 files, 354 lines -> internal/database [DONE]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| database/DatabaseConnection.java | 336 | DONE | internal/database |  |
| database/DatabaseException.java | 18 | MERG | internal/database | Go error wrapping (db.go) |

## pkg database1 - 2 files, 355 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| database1/DatabaseConnection1.java | 337 | SKIP | - | dup of database |
| database1/DatabaseException1.java | 18 | SKIP | - | dup of database |

## pkg download - 1 files, 98 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| download/Toupdate.java | 98 | SKIP | - | updater |

## pkg fumo - 2 files, 148 lines -> internal/custom/fumo [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| fumo/FumoSkill.java | 142 | TODO | internal/custom/fumo |  |
| fumo/Jianding.java | 6 | TODO | internal/custom/fumo |  |

## pkg gui - 23 files, 17875 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/QQMsgServer.java | 1920 | SKIP | - | swing gui -> web panel |
| gui/QQ通信.java | 32 | SKIP | - | swing gui -> web panel |
| gui/Start.java | 2475 | SKIP | - | swing gui -> web panel |
| gui/ZEVMS.java | 1052 | SKIP | - | swing gui -> web panel |
| gui/ZEVMS2.java | 8715 | SKIP | - | swing gui -> web panel |
| gui/ZevmsLauncherServer.java | 203 | SKIP | - | swing gui -> web panel |
| gui/更多应用.java | 138 | SKIP | - | swing gui -> web panel |
| gui/股票系统.java | 83 | SKIP | - | swing gui -> web panel |
| gui/活动OX答题.java | 395 | SKIP | - | swing gui -> web panel |
| gui/活动倍率活动.java | 67 | SKIP | - | swing gui -> web panel |
| gui/活动绝地求生.java | 125 | SKIP | - | swing gui -> web panel |
| gui/活动每日彩票.java | 71 | SKIP | - | swing gui -> web panel |
| gui/活动魔族攻城.java | 355 | SKIP | - | swing gui -> web panel |
| gui/活动魔族攻城1.java | 1628 | SKIP | - | swing gui -> web panel |
| gui/活动魔族入侵.java | 97 | SKIP | - | swing gui -> web panel |
| gui/活动神秘商人.java | 73 | SKIP | - | swing gui -> web panel |
| gui/活动推雪球比赛.java | 57 | SKIP | - | swing gui -> web panel |
| gui/活动喜从天降.java | 126 | SKIP | - | swing gui -> web panel |
| gui/活动野外通缉.java | 31 | SKIP | - | swing gui -> web panel |
| gui/活动鱼来鱼往.java | 109 | SKIP | - | swing gui -> web panel |
| gui/检测数据库表.java | 37 | SKIP | - | swing gui -> web panel |
| gui/冒险股票系统.java | 26 | SKIP | - | swing gui -> web panel |
| gui/启动进度条.java | 60 | SKIP | - | swing gui -> web panel |

## pkg gui\Jieya - 8 files, 1286 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/Jieya/Deflater.java | 159 | TODO | internal/TBD |  |
| gui/Jieya/DeflaterOutputStream.java | 84 | TODO | internal/TBD |  |
| gui/Jieya/InflaterInputStream.java | 133 | TODO | internal/TBD |  |
| gui/Jieya/ZipConstants.java | 46 | TODO | internal/TBD |  |
| gui/Jieya/ZipEntry.java | 142 | TODO | internal/TBD |  |
| gui/Jieya/ZipInputStream.java | 311 | TODO | internal/TBD |  |
| gui/Jieya/ZipOutputStream.java | 356 | TODO | internal/TBD |  |
| gui/Jieya/解压文件.java | 55 | TODO | internal/TBD |  |

## pkg gui\进阶BOSS - 1 files, 317 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/进阶BOSS/进阶BOSS线程.java | 317 | TODO | internal/TBD |  |

## pkg gui\控制台 - 18 files, 15800 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/控制台/channelServer.java | 8 | TODO | internal/TBD |  |
| gui/控制台/PVP控制台.java | 85 | TODO | internal/TBD |  |
| gui/控制台/充值卡后台.java | 584 | TODO | internal/TBD |  |
| gui/控制台/锻造控制台.java | 805 | TODO | internal/TBD |  |
| gui/控制台/活动控制台.java | 1304 | TODO | internal/TBD |  |
| gui/控制台/角色转移工具.java | 373 | TODO | internal/TBD |  |
| gui/控制台/脚本编辑器2.java | 443 | TODO | internal/TBD |  |
| gui/控制台/脚本更新器.java | 173 | TODO | internal/TBD |  |
| gui/控制台/控制台1号.java | 4737 | TODO | internal/TBD |  |
| gui/控制台/控制台2号.java | 3298 | TODO | internal/TBD |  |
| gui/控制台/控制台3号.java | 1997 | TODO | internal/TBD |  |
| gui/控制台/快捷面板.java | 589 | TODO | internal/TBD |  |
| gui/控制台/聊天记录显示.java | 68 | TODO | internal/TBD |  |
| gui/控制台/强化控制台.java | 107 | TODO | internal/TBD |  |
| gui/控制台/任务控制台.java | 153 | TODO | internal/TBD |  |
| gui/控制台/世界频道登陆显示窗.java | 77 | TODO | internal/TBD |  |
| gui/控制台/一键还原.java | 921 | TODO | internal/TBD |  |
| gui/控制台/游戏商城登陆显示窗.java | 78 | TODO | internal/TBD |  |

## pkg gui\控制台\活动 - 1 files, 149 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/控制台/活动/夺旗对抗赛.java | 149 | TODO | internal/TBD |  |

## pkg gui\通信 - 2 files, 1481 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/通信/ChatClient.java | 67 | TODO | internal/TBD |  |
| gui/通信/QQMsgServer.java | 1414 | TODO | internal/TBD |  |

## pkg gui\图片\folder - 1 files, 7535 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/图片/folder/副本控制台.java | 7535 | TODO | internal/TBD |  |

## pkg gui\图片\gui1 - 5 files, 272 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/图片/gui1/ProgressBar.java | 59 | TODO | internal/TBD |  |
| gui/图片/gui1/SetFont.java | 40 | TODO | internal/TBD |  |
| gui/图片/gui1/TestCmd.java | 45 | TODO | internal/TBD |  |
| gui/图片/gui1/TestURL.java | 88 | TODO | internal/TBD |  |
| gui/图片/gui1/ZPanel.java | 40 | TODO | internal/TBD |  |

## pkg gui\图片\shuchu - 5 files, 322 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/图片/shuchu/AppOutputCapture.java | 49 | TODO | internal/TBD |  |
| gui/图片/shuchu/ConsoleTextArea.java | 102 | TODO | internal/TBD |  |
| gui/图片/shuchu/Listing2.java | 39 | TODO | internal/TBD |  |
| gui/图片/shuchu/Listing3.java | 52 | TODO | internal/TBD |  |
| gui/图片/shuchu/LoopedStreams.java | 80 | TODO | internal/TBD |  |

## pkg gui\图片\xiazai - 7 files, 506 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/图片/xiazai/DownloadFile.java | 9 | TODO | internal/TBD |  |
| gui/图片/xiazai/DownloadFrame.java | 143 | TODO | internal/TBD |  |
| gui/图片/xiazai/DownThread.java | 123 | TODO | internal/TBD |  |
| gui/图片/xiazai/FileDownThread.java | 57 | TODO | internal/TBD |  |
| gui/图片/xiazai/MultiDown.java | 104 | TODO | internal/TBD |  |
| gui/图片/xiazai/StartDownload.java | 63 | TODO | internal/TBD |  |
| gui/图片/xiazai/ThreadController.java | 7 | TODO | internal/TBD |  |

## pkg gui\图片\动图 - 1 files, 163 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/图片/动图/JpanelForm.java | 163 | TODO | internal/TBD |  |

## pkg gui\网关 - 2 files, 636 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/网关/机器人群设置面板.java | 94 | TODO | internal/TBD |  |
| gui/网关/网关_1.java | 542 | TODO | internal/TBD |  |

## pkg gui\未分类 - 8 files, 788 lines -> internal/TBD [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| gui/未分类/Client.java | 324 | TODO | internal/TBD |  |
| gui/未分类/HttpDownload.java | 83 | TODO | internal/TBD |  |
| gui/未分类/User.java | 24 | TODO | internal/TBD |  |
| gui/未分类/广告.java | 69 | TODO | internal/TBD |  |
| gui/未分类/取名字.java | 32 | TODO | internal/TBD |  |
| gui/未分类/数据库提示.java | 85 | TODO | internal/TBD |  |
| gui/未分类/下载指定文件.java | 94 | TODO | internal/TBD |  |
| gui/未分类/娱乐玩法.java | 77 | TODO | internal/TBD |  |

## pkg handling - 6 files, 1942 lines -> internal/protocol [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| handling/ExternalCodeTableGetter.java | 74 | DONE | internal/protocol | opcode/dispatch |
| handling/MapleServerHandler.java | 1298 | ACTV | internal/login/server.go + channel/server.go | P2.2 已迁 opcode 分发（LOGIN_PASSWORD/PONG）+hello；P4.1 频道分支（channelActive 同款 IV/hello + channel>0 时的 isShutdown 门禁）；P4.2 频道侧 PLAYER_LOGGEDIN 实派（handlePlayerLoggedIn）+ OnClose 注销；其余频道 opcode（CHANGE_CHANNEL/移动/聊天…）待 P4.3+ |
| handling/MapleServerHandlerMBean.java | 7 | TODO | internal/protocol | opcode/dispatch |
| handling/RecvPacketOpcode.java | 238 | DONE | internal/protocol | opcode/dispatch |
| handling/SendPacketOpcode.java | 317 | DONE | internal/protocol | opcode/dispatch |
| handling/WritableIntValueHolder.java | 8 | DONE | internal/protocol | opcode/dispatch |

## pkg handling\cashshop - 1 files, 61 lines -> internal/cashshop [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| handling/cashshop/CashShopServer.java | 61 | TODO | internal/cashshop |  |

## pkg handling\cashshop\handler - 2 files, 1386 lines -> internal/cashshop/handler [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| handling/cashshop/handler/CashShopOperation.java | 1179 | TODO | internal/cashshop/handler |  |
| handling/cashshop/handler/MTSOperation.java | 207 | TODO | internal/cashshop/handler |  |

## pkg handling\channel - 4 files, 2101 lines -> internal/channel [ACTV]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| handling/channel/ChannelServer.java | 706 | ACTV | internal/channel/server.go | P4.1 骨架：端口公式 7574+channel（频道 1..N，Count 上限 10）、与登录服同一 hello/IV、PONG→PING、PLAYER_LOGGEDIN 骨架、exp/meso/drop 上限 100、load 上报登录服（LoginWorker 的 10 分钟轮询改实时推送）；P4.2：addPlayer（注册 + 滚动公告）、removePlayer、forceRemovePlayerByAccId（跨频道顶号）、getMapFactory().getMap(id)（惰性地图注册表）；P4.3：进图/登出的视野广播下沉到 internal/mapp（addPlayer/removePlayer）；事件/雇佣商店/PersistingTask 待 P4.3b+/P5 |
| handling/channel/fameRankingInfo.java | 6 | TODO | internal/channel |  |
| handling/channel/MapleGuildRanking.java | 1097 | TODO | internal/channel |  |
| handling/channel/PlayerStorage.java | 292 | ACTV | internal/channel/players.go | P4.1：nameToChar/idToChar 双 map + RW 锁、register/deregister/按名按 id 查/在线数 + World.Find 副作用；P4.2：CharacterTransfer 挂起表（registerPendingPlayer/getPendingCharacter 取走式/deregisterPendingPlayer，40s 过期由读时判断，Java 的 PersistingTask ticker 待 P5）；PersistingTask 存档待 P5 |

## pkg handling\channel\handler - 30 files, 11866 lines -> internal/channel/handler [ACTV]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| handling/channel/handler/AllianceHandler.java | 121 | TODO | internal/channel/handler |  |
| handling/channel/handler/AttackInfo.java | 52 | TODO | internal/channel/handler |  |
| handling/channel/handler/AttackType.java | 10 | TODO | internal/channel/handler |  |
| handling/channel/handler/BBSHandler.java | 124 | TODO | internal/channel/handler |  |
| handling/channel/handler/BeanGame.java | 108 | TODO | internal/channel/handler |  |
| handling/channel/handler/BuddyListHandler.java | 198 | TODO | internal/channel/handler |  |
| handling/channel/handler/ChatHandler.java | 420 | DONE | internal/channel | P4.5：`GeneralChat`（长度上限按 **UTF-16 码元** `>=80` 且 GM 豁免 → `broadcastMessage(pkt, position)` 的 **Point 重载带视野过滤**、source=null 故说话人自收；`whiteBG=isGM()`、`show`=包尾字节回写）与 `Whisper_Find`（mode 5/68 找人 → 本频道 `getFindReplyWithMap` / 跨频道 `getFindReply`，GM 隐身规则；mode 6 → `getWhisper`+`getWhisperReply`，非 GM 私聊 GM reply 归 0；注册表陈旧 `player==null` 时 Java 直接 break 不回包）已迁入 `channel/chat.go`；`Others`/`Messenger`/`MAPLETV` 属 P8；`玩家聊天开关`/`游戏找人开关` 由 P4.5b 转正；跳过 `CommandProcessor`（P7）、`getCanTalk()` 禁言、`checkMsg()`（空转）、`聊天记录开关` 文件日志（运营侧待做）；**未被调用的 `abc/关键字屏蔽`/`屏幕关键字` 是死代码，不迁** |
| handling/channel/handler/cms.java | 8 | TODO | internal/channel/handler |  |
| handling/channel/handler/DamageParse.java | 1286 | TODO | internal/channel/handler |  |
| handling/channel/handler/DueyHandler.java | 284 | TODO | internal/channel/handler |  |
| handling/channel/handler/FamilyHandler.java | 291 | TODO | internal/channel/handler |  |
| handling/channel/handler/GuildHandler.java | 209 | TODO | internal/channel/handler |  |
| handling/channel/handler/HiredMerchantHandler.java | 319 | TODO | internal/channel/handler |  |
| handling/channel/handler/InterServerHandler.java | 359 | ACTV | internal/channel | P4.2：Loggedin2 全链（挂起表优先 → loadCharFromDB(GetCharacterByID) → forceRemovePlayerByAccId → ChannelServer.addPlayer → getCharInfo + temporaryStats_Reset → map.addPlayer，login.go）；跳过 登陆验证开关/GM 技能/updateLoginState(2)；好友/组队/公会/家族/信使待 P8 |
| handling/channel/handler/InventoryHandler.java | 3048 | TODO | internal/channel/handler |  |
| handling/channel/handler/ItemMakerHandler.java | 312 | TODO | internal/channel/handler |  |
| handling/channel/handler/MobHandler.java | 213 | TODO | internal/channel/handler |  |
| handling/channel/handler/MonsterCarnivalHandler.java | 106 | TODO | internal/channel/handler |  |
| handling/channel/handler/MovementParse.java | 123 | DONE | internal/movement | P4.4：`Parse`（parseMovement kind=1，逐命令布局 + 两个 Java 怪癖：NewFh=构造器第 5 参、3/4/7/8/9/11 组丢弃 duration）+ `UpdatePosition`（Target 接口 = AnimatedMapleMapObject 的 setPosition/setFh/setStance） |
| handling/channel/handler/NPCHandler.java | 552 | TODO | internal/channel/handler |  |
| handling/channel/handler/PartyHandler.java | 169 | TODO | internal/channel/handler |  |
| handling/channel/handler/PetHandler.java | 230 | TODO | internal/channel/handler |  |
| handling/channel/handler/Player.java | 11 | TODO | internal/channel/handler |  |
| handling/channel/handler/PlayerHandler.java | 1378 | ACTV | internal/channel | P4.4：`MovePlayer`（skip(33) → MovementParse → broadcastMessage(player, movePlayer, false) → updatePosition + setOldPosition）已迁入 `channel/movement.go`；飞天检测/follow/clone/坠落计数（依赖 foothold）与其余 handler 待后续 |
| handling/channel/handler/PlayerInteractionHandler.java | 890 | TODO | internal/channel/handler |  |
| handling/channel/handler/PlayersHandler.java | 493 | TODO | internal/channel/handler |  |
| handling/channel/handler/StatsHandling.java | 303 | TODO | internal/channel/handler |  |
| handling/channel/handler/statups.java | 13 | TODO | internal/channel/handler |  |
| handling/channel/handler/SummonHandler.java | 170 | TODO | internal/channel/handler |  |
| handling/channel/handler/UserInterfaceHandler.java | 66 | TODO | internal/channel/handler |  |

## pkg handling\login - 4 files, 366 lines -> internal/login [ACTV]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| handling/login/Balloon.java | 14 | MERG | internal/login/worlds.go | P2.3 类型并入 login.Balloon + config |
| handling/login/LoginInformationProvider.java | 31 | TODO | internal/login |  |
| handling/login/LoginServer.java | 135 | MERG | internal/login/worlds.go | P2.3 静态字段(serverName/eventMessage/userLimit/load)->WorldConfig+config；P4.1 loginAuth/loginIPAuth 表 -> internal/world/registry.go（put/take/contains/remove/add 全迁）+ addChannel/removeChannel/load 计数由 channel 服 SetChannelLoad 推送 |
| handling/login/LoginWorker.java | 186 | MERG | internal/login/auth.go | P2.2 registerClient 子集（gender=10 分支+AuthSuccess）；P2.3 其 ServerList 双实现合并进 handleServerList；P4.1 load 换算（factor=1200*channels/userLimit，min 1200）迁至 displayChannelLoad（原始值不再被就地覆写） |

## pkg handling\login\handler - 2 files, 749 lines -> internal/login/handler [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| handling/login/handler/AutoRegister.java | 126 | DONE | internal/login/register.go | P2.5：自动注册 + createAccount（createAccount 的 email/birthday/qq 占位常量保留）；getAccountExists2/判断角色ID是否存在（DatabaseConnection1/游戏QQ号 表）不迁 |
| handling/login/handler/CharLoginHandler.java | 623 | ACTV | internal/login | P2.2 已迁 login()；P2.3 已迁 ServerListRequest/ServerStatusRequest；P2.4 已迁 CharlistRequest/CheckCharName/CreateChar/DeleteChar/SetGenderRequest；P2.5 已迁自动注册分支 + hasBannedIP/isBannedMac 门禁（见 AutoRegister.java 行）；登陆保护/登陆队列/IP|机器码多开 走 config 键暂不迁；P4.1 已迁 Character_WithoutSecondPassword（0x000A：DB loggedin==2 + login_Auth + 频道存活校验 → putLoginAuth + getServerIP）；Character_WithSecondPassword 在 v079 recvops 无 opcode（-2）不迁 |

## pkg handling\netty - 4 files, 295 lines -> internal/netw [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| handling/netty/MaplePacketDecoder.java | 102 | DONE | internal/netw |  |
| handling/netty/MaplePacketEncoder.java | 102 | DONE | internal/netw |  |
| handling/netty/ServerConnection.java | 61 | DONE | internal/netw |  |
| handling/netty/ServerInitializer.java | 30 | DONE | internal/netw |  |

## pkg handling\world - 12 files, 2531 lines -> internal/world [ACTV]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| handling/world/CharacterIdChannelPair.java | 40 | TODO | internal/world |  |
| handling/world/CharacterTransfer.java | 522 | TODO | internal/world |  |
| handling/world/CheaterData.java | 35 | TODO | internal/world |  |
| handling/world/MapleMessenger.java | 104 | TODO | internal/world |  |
| handling/world/MapleMessengerCharacter.java | 56 | TODO | internal/world |  |
| handling/world/MapleParty.java | 166 | TODO | internal/world |  |
| handling/world/MaplePartyCharacter.java | 102 | TODO | internal/world |  |
| handling/world/mch.java | 8 | TODO | internal/world |  |
| handling/world/PartyOperation.java | 14 | TODO | internal/world |  |
| handling/world/PlayerBuffStorage.java | 35 | TODO | internal/world |  |
| handling/world/PlayerBuffValueHolder.java | 16 | TODO | internal/world |  |
| handling/world/World.java | 1433 | ACTV | internal/world | P4.1 迁 World.Find 子集（register/forceDeregister/find/findByName → find.go，PlayerStorage 注册时同步）；World.Client 的登录会话注册表落在 login.Server.clients（双登踢人用）；Party/Guild/Buddy/Messenger 待 P8 |

## pkg handling\world\family - 3 files, 816 lines -> internal/world/family [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| handling/world/family/MapleFamily.java | 472 | TODO | internal/world/family |  |
| handling/world/family/MapleFamilyBuff.java | 99 | TODO | internal/world/family |  |
| handling/world/family/MapleFamilyCharacter.java | 245 | TODO | internal/world/family |  |

## pkg handling\world\guild - 6 files, 1384 lines -> internal/world/guild [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| handling/world/guild/MapleBBSThread.java | 62 | TODO | internal/world/guild |  |
| handling/world/guild/MapleGuild.java | 842 | TODO | internal/world/guild |  |
| handling/world/guild/MapleGuildAlliance.java | 327 | TODO | internal/world/guild |  |
| handling/world/guild/MapleGuildCharacter.java | 91 | TODO | internal/world/guild |  |
| handling/world/guild/MapleGuildResponse.java | 20 | TODO | internal/world/guild |  |
| handling/world/guild/MapleGuildSummary.java | 42 | TODO | internal/world/guild |  |

## pkg module\system\common - 1 files, 40 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| module/system/common/NetState.java | 40 | SKIP | - | old launcher leftover |

## pkg provider - 9 files, 251 lines -> internal/wzs [MERG] (P3.1)

| java file | lines | status | go target | note |
|---|---|---|---|---|
| provider/MapleCanvas.java | 10 | MERG | internal/wzs | into data.go Canvas (尺寸/PngPath，不解码) |
| provider/MapleData.java | 17 | MERG | internal/wzs | into data.go Node |
| provider/MapleDataDirectoryEntry.java | 13 | MERG | internal/wzs | into provider.go DirEntry |
| provider/MapleDataEntity.java | 8 | MERG | internal/wzs | Node.Name/Parent |
| provider/MapleDataEntry.java | 13 | MERG | internal/wzs | DirEntry 公共字段 |
| provider/MapleDataFileEntry.java | 9 | MERG | internal/wzs | DirEntry 文件条目 |
| provider/MapleDataProvider.java | 10 | DONE | internal/wzs | Provider（Data/Root） |
| provider/MapleDataProviderFactory.java | 26 | DONE | internal/wzs | OpenRoot/Root.WZ（wzpath 语义） |
| provider/MapleDataTool.java | 145 | DONE | internal/wzs | GetString/GetInt/GetIntConvert/… 全族 |

## pkg provider\WzXML - 8 files, 563 lines -> internal/wzs [MERG] (P3.1)

| java file | lines | status | go target | note |
|---|---|---|---|---|
| provider/WzXML/FileStoredPngMapleCanvas.java | 46 | MERG | internal/wzs | Canvas.PNGPath（导出无 png，仅路径语义） |
| provider/WzXML/MapleDataType.java | 23 | DONE | internal/wzs | type.go DataType |
| provider/WzXML/PNGMapleCanvas.java | 126 | SKIP | - | PNG 解码：服务端不渲染 |
| provider/WzXML/WZDirectoryEntry.java | 47 | MERG | internal/wzs | DirEntry（惰性列目录） |
| provider/WzXML/WZEntry.java | 40 | MERG | internal/wzs | DirEntry 嵌入 |
| provider/WzXML/WZFileEntry.java | 23 | MERG | internal/wzs | DirEntry 文件条目 |
| provider/WzXML/XMLDomMapleData.java | 189 | MERG | internal/wzs | xml.go parseXML + Node 取值 |
| provider/WzXML/XMLWZFile.java | 69 | DONE | internal/wzs | Provider（getData = <path>.xml） |

## pkg pvp - 2 files, 251 lines -> internal/custom/pvp [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| pvp/MaplePvp.java | 132 | TODO | internal/custom/pvp |  |
| pvp/Pvpskill.java | 119 | TODO | internal/custom/pvp |  |

## pkg quest - 1 files, 44 lines -> internal/script/quest [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| quest/Quest.java | 44 | TODO | internal/script/quest |  |

## pkg scripting - 17 files, 22654 lines -> internal/script [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| scripting/AbstractPlayerInteraction.java | 5877 | TODO | internal/script |  |
| scripting/AbstractScriptManager.java | 78 | TODO | internal/script |  |
| scripting/BytesEncodingDetect.java | 3973 | TODO | internal/script |  |
| scripting/Class1.java | 87 | TODO | internal/script |  |
| scripting/Encoding.java | 113 | TODO | internal/script |  |
| scripting/EncodingDetect.java | 16 | TODO | internal/script |  |
| scripting/EventInstanceManager.java | 1079 | TODO | internal/script |  |
| scripting/EventManager.java | 783 | TODO | internal/script |  |
| scripting/EventScriptManager.java | 62 | TODO | internal/script |  |
| scripting/LieDetectorScript.java | 71 | TODO | internal/script |  |
| scripting/NPCConversationManager.java | 9729 | TODO | internal/script |  |
| scripting/NPCScriptManager.java | 438 | TODO | internal/script |  |
| scripting/PortalPlayerInteraction.java | 29 | TODO | internal/script |  |
| scripting/PortalScript.java | 8 | TODO | internal/script |  |
| scripting/PortalScriptManager.java | 94 | TODO | internal/script |  |
| scripting/ReactorActionManager.java | 122 | TODO | internal/script |  |
| scripting/ReactorScriptManager.java | 95 | TODO | internal/script |  |

## pkg server - 39 files, 9083 lines -> internal/server-domain [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/AutobanManager.java | 170 | TODO | internal/server-domain | split per file |
| server/CashItemFactory.java | 133 | TODO | internal/server-domain | split per file |
| server/CashItemFactoryA.java | 101 | TODO | internal/server-domain | split per file |
| server/CashItemInfo.java | 159 | TODO | internal/server-domain | split per file |
| server/CashItemInfoA.java | 60 | TODO | internal/server-domain | split per file |
| server/CashShop.java | 275 | TODO | internal/server-domain | split per file |
| server/ItemMakerFactory.java | 170 | TODO | internal/server-domain | split per file |
| server/MapleAchievement.java | 6 | TODO | internal/server-domain | split per file |
| server/MapleAchievements.java | 6 | TODO | internal/server-domain | split per file |
| server/MapleCarnivalChallenge.java | 445 | TODO | internal/server-domain | split per file |
| server/MapleCarnivalFactory.java | 64 | TODO | internal/server-domain | split per file |
| server/MapleCarnivalParty.java | 102 | TODO | internal/server-domain | split per file |
| server/MapleDueyActions.java | 48 | TODO | internal/server-domain | split per file |
| server/MapleInventoryManipulator.java | 965 | TODO | internal/server-domain | split per file |
| server/MapleItemInformationProvider.java | 1459 | TODO | internal/server-domain | split per file |
| server/MaplePortal.java | 21 | DONE | internal/mapp | P4.3b：`portal.go` 的 `PortalMap=2`/`PortalDoor=6` 常量与入口（`pn`/`pt`/`x`/`y`/`tm`/`tn`/`script`）+ `Portal`/`PortalByName`（文档序，Java 是 HashMap 序）+ `FindClosestSpawnPoint`（type 0..2 且 tm==999999999，平方距离取最近、平手留先者）；`MAP_PORTAL` 与 `DOOR_PORTAL` 在 Java 里同为 `MapleGenericPortal` 语义（`MapleMapPortal` 无差异） |
| server/MapleShop.java | 264 | TODO | internal/server-domain | split per file |
| server/MapleShopFactory.java | 43 | TODO | internal/server-domain | split per file |
| server/MapleShopItem.java | 23 | TODO | internal/server-domain | split per file |
| server/MapleSquad.java | 369 | TODO | internal/server-domain | split per file |
| server/MapleStatEffect.java | 1773 | TODO | internal/server-domain | split per file |
| server/MapleStorage.java | 213 | TODO | internal/server-domain | split per file |
| server/MapleTrade.java | 355 | TODO | internal/server-domain | split per file |
| server/MerchItemPackage.java | 37 | TODO | internal/server-domain | split per file |
| server/MTSCart.java | 148 | TODO | internal/server-domain | split per file |
| server/MTSStorage.java | 385 | TODO | internal/server-domain | split per file |
| server/PortalFactory.java | 36 | DONE | internal/mapp | P4.3b：`loadPortals`（`portal.go`）——属性读取 + **id 规则**：`pt==6` 忽略 wz 节点名、`nextDoorPortal` 从 **128** 起按文档序编号，其余 `Integer.parseInt(节点名)`（所以 map 100000000 的 6 个 tp 是 id 128..133，`getPortal(28..33)` 为 nil）；空 `script` 归一为空串；非数字节点名跳过+告警（Java 抛 NumberFormatException 丢整张图） |
| server/PredictCardFactory.java | 71 | TODO | internal/server-domain | split per file |
| server/Randomizer.java | 32 | TODO | internal/server-domain | split per file |
| server/RandomRewards.java | 144 | TODO | internal/server-domain | split per file |
| server/RankingWorker.java | 130 | TODO | internal/server-domain | split per file |
| server/ServerProperties.java | 55 | DONE | internal/server-domain | split per file |
| server/ShutdownServer.java | 133 | TODO | internal/server-domain | split per file |
| server/ShutdownServerMBean.java | 8 | TODO | internal/server-domain | split per file |
| server/SpeedRunner.java | 151 | TODO | internal/server-domain | split per file |
| server/StructPotentialItem.java | 305 | TODO | internal/server-domain | split per file |
| server/StructRewardItem.java | 12 | TODO | internal/server-domain | split per file |
| server/StructSetItem.java | 31 | TODO | internal/server-domain | split per file |
| server/Timer.java | 181 | TODO | internal/server-domain | split per file |

## pkg server\custom\auction - 4 files, 878 lines -> internal/custom/auction [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/auction/AuctionItem.java | 71 | TODO | internal/custom/auction |  |
| server/custom/auction/AuctionManager.java | 750 | TODO | internal/custom/auction |  |
| server/custom/auction/AuctionPoint.java | 34 | TODO | internal/custom/auction |  |
| server/custom/auction/AuctionState.java | 23 | TODO | internal/custom/auction |  |

## pkg server\custom\auction1 - 4 files, 878 lines -> internal/custom/auction [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/auction1/AuctionItem1.java | 71 | MERG | internal/custom/auction | into auction |
| server/custom/auction1/AuctionManager1.java | 750 | MERG | internal/custom/auction | into auction |
| server/custom/auction1/AuctionPoint1.java | 34 | MERG | internal/custom/auction | into auction |
| server/custom/auction1/AuctionState1.java | 23 | MERG | internal/custom/auction | into auction |

## pkg server\custom\bankitem - 2 files, 247 lines -> internal/custom/bank [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bankitem/BankItem.java | 34 | TODO | internal/custom/bank |  |
| server/custom/bankitem/BankItemManager.java | 213 | TODO | internal/custom/bank |  |

## pkg server\custom\bankitem1 - 2 files, 247 lines -> internal/custom/bank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bankitem1/BankItem1.java | 34 | MERG | internal/custom/bank | into bank |
| server/custom/bankitem1/BankItemManager1.java | 213 | MERG | internal/custom/bank | into bank |

## pkg server\custom\bankitem2 - 2 files, 247 lines -> internal/custom/bank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bankitem2/BankItem2.java | 34 | MERG | internal/custom/bank | into bank |
| server/custom/bankitem2/BankItemManager2.java | 213 | MERG | internal/custom/bank | into bank |

## pkg server\custom\bossrank - 2 files, 305 lines -> internal/custom/rank [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bossrank/BossRankInfo.java | 41 | TODO | internal/custom/rank | base impl |
| server/custom/bossrank/BossRankManager.java | 264 | TODO | internal/custom/rank | base impl |

## pkg server\custom\bossrank1 - 2 files, 305 lines -> internal/custom/rank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bossrank1/BossRankInfo1.java | 41 | MERG | internal/custom/rank | into rank multi-instance |
| server/custom/bossrank1/BossRankManager1.java | 264 | MERG | internal/custom/rank | into rank multi-instance |

## pkg server\custom\bossrank10 - 2 files, 301 lines -> internal/custom/rank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bossrank10/BossRankInfo10.java | 41 | MERG | internal/custom/rank | into rank multi-instance |
| server/custom/bossrank10/BossRankManager10.java | 260 | MERG | internal/custom/rank | into rank multi-instance |

## pkg server\custom\bossrank2 - 2 files, 305 lines -> internal/custom/rank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bossrank2/BossRankInfo2.java | 41 | MERG | internal/custom/rank | into rank multi-instance |
| server/custom/bossrank2/BossRankManager2.java | 264 | MERG | internal/custom/rank | into rank multi-instance |

## pkg server\custom\bossrank3 - 2 files, 306 lines -> internal/custom/rank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bossrank3/BossRankInfo3.java | 41 | MERG | internal/custom/rank | into rank multi-instance |
| server/custom/bossrank3/BossRankManager3.java | 265 | MERG | internal/custom/rank | into rank multi-instance |

## pkg server\custom\bossrank4 - 2 files, 305 lines -> internal/custom/rank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bossrank4/BossRankInfo4.java | 41 | MERG | internal/custom/rank | into rank multi-instance |
| server/custom/bossrank4/BossRankManager4.java | 264 | MERG | internal/custom/rank | into rank multi-instance |

## pkg server\custom\bossrank5 - 2 files, 305 lines -> internal/custom/rank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bossrank5/BossRankInfo5.java | 41 | MERG | internal/custom/rank | into rank multi-instance |
| server/custom/bossrank5/BossRankManager5.java | 264 | MERG | internal/custom/rank | into rank multi-instance |

## pkg server\custom\bossrank6 - 2 files, 305 lines -> internal/custom/rank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bossrank6/BossRankInfo6.java | 41 | MERG | internal/custom/rank | into rank multi-instance |
| server/custom/bossrank6/BossRankManager6.java | 264 | MERG | internal/custom/rank | into rank multi-instance |

## pkg server\custom\bossrank7 - 2 files, 305 lines -> internal/custom/rank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bossrank7/BossRankInfo7.java | 41 | MERG | internal/custom/rank | into rank multi-instance |
| server/custom/bossrank7/BossRankManager7.java | 264 | MERG | internal/custom/rank | into rank multi-instance |

## pkg server\custom\bossrank8 - 2 files, 305 lines -> internal/custom/rank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bossrank8/BossRankInfo8.java | 41 | MERG | internal/custom/rank | into rank multi-instance |
| server/custom/bossrank8/BossRankManager8.java | 264 | MERG | internal/custom/rank | into rank multi-instance |

## pkg server\custom\bossrank9 - 2 files, 305 lines -> internal/custom/rank [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/bossrank9/BossRankInfo9.java | 41 | MERG | internal/custom/rank | into rank multi-instance |
| server/custom/bossrank9/BossRankManager9.java | 264 | MERG | internal/custom/rank | into rank multi-instance |

## pkg server\custom\capture - 2 files, 165 lines -> internal/custom/capture [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/capture/capture_yongfa.java | 159 | TODO | internal/custom/capture |  |
| server/custom/capture/Capturex.java | 6 | TODO | internal/custom/capture |  |

## pkg server\custom\forum - 3 files, 602 lines -> internal/custom/forum [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/forum/Forum_Reply.java | 178 | TODO | internal/custom/forum | optional |
| server/custom/forum/Forum_Section.java | 155 | TODO | internal/custom/forum | optional |
| server/custom/forum/Forum_Thread.java | 269 | TODO | internal/custom/forum | optional |

## pkg server\custom\rank - 2 files, 195 lines -> internal/custom/rank [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/rank/MiniGamePoints.java | 64 | TODO | internal/custom/rank |  |
| server/custom/rank/RankManager.java | 131 | TODO | internal/custom/rank |  |

## pkg server\custom\respawn - 4 files, 256 lines -> internal/mapp [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/respawn/mch.java | 8 | TODO | internal/mapp |  |
| server/custom/respawn/RespawnInfo.java | 85 | TODO | internal/mapp |  |
| server/custom/respawn/RespawnManager.java | 78 | TODO | internal/mapp |  |
| server/custom/respawn/召唤怪物.java | 85 | TODO | internal/mapp |  |

## pkg server\custom\treasure_house - 1 files, 65 lines -> internal/custom/treasure [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/custom/treasure_house/treasure_x.java | 65 | TODO | internal/custom/treasure | optional |

## pkg server\events - 8 files, 1048 lines -> internal/mapp/events [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/events/MapleCoconut.java | 212 | TODO | internal/mapp/events |  |
| server/events/MapleEvent.java | 158 | TODO | internal/mapp/events |  |
| server/events/MapleEventType.java | 25 | TODO | internal/mapp/events |  |
| server/events/MapleFitness.java | 116 | TODO | internal/mapp/events |  |
| server/events/MapleOla.java | 85 | TODO | internal/mapp/events |  |
| server/events/MapleOxQuiz.java | 118 | TODO | internal/mapp/events |  |
| server/events/MapleOxQuizFactory.java | 133 | TODO | internal/mapp/events |  |
| server/events/MapleSnowball.java | 201 | TODO | internal/mapp/events |  |

## pkg server\life - 23 files, 3602 lines -> internal/life [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/life/AbstractLoadedMapleLife.java | 89 | TODO | internal/life |  |
| server/life/BanishInfo.java | 23 | TODO | internal/life |  |
| server/life/ChangeableStats.java | 34 | TODO | internal/life |  |
| server/life/Element.java | 40 | TODO | internal/life |  |
| server/life/ElementalEffectiveness.java | 28 | TODO | internal/life |  |
| server/life/MapleLifeFactory.java | 243 | TODO | internal/life |  |
| server/life/MapleMonster.java | 1507 | TODO | internal/life |  |
| server/life/MapleMonsterInformationProvider.java | 111 | DONE | internal/life/drops.go | P3.5：掉落缓存（EQUIP chance/3、出错不缓存） |
| server/life/MapleMonsterStats.java | 272 | TODO | internal/life |  |
| server/life/MapleNPC.java | 55 | TODO | internal/life |  |
| server/life/MobAttackInfo.java | 41 | TODO | internal/life |  |
| server/life/MobAttackInfoFactory.java | 48 | TODO | internal/life |  |
| server/life/MobSkill.java | 482 | TODO | internal/life |  |
| server/life/MobSkillFactory.java | 57 | TODO | internal/life |  |
| server/life/MonsterDropEntry.java | 18 | TODO | internal/life |  |
| server/life/MonsterGlobalDropEntry.java | 33 | TODO | internal/life |  |
| server/life/MonsterListener.java | 7 | TODO | internal/life |  |
| server/life/OverrideMonsterStats.java | 44 | TODO | internal/life |  |
| server/life/PlayerNPC.java | 245 | TODO | internal/life |  |
| server/life/SpawnPoint.java | 105 | ACTV | internal/mapp | P4.3b：`LifeSpawn` 承载 SpawnPoint 的数据面（mobTime **秒**、`immobile`/`shouldSpawn` 的判定与重生计时留给 P6 的刷怪调度）；出生点坐标 = `calcPointBelow(X,Y).y - 1`（对齐 `MapleMap.addMonsterSpawn` 的 `--newpos.y`，斜面取插值） |
| server/life/SpawnPointAreaBoss.java | 84 | TODO | internal/life |  |
| server/life/Spawns.java | 16 | TODO | internal/life |  |
| server/life/SummonAttackEntry.java | 20 | TODO | internal/life |  |

## pkg server\maps - 35 files, 8636 lines -> internal/mapp [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/maps/AbstractAnimatedMapleMapObject.java | 35 | TODO | internal/mapp |  |
| server/maps/AbstractMapleMapObject.java | 34 | TODO | internal/mapp |  |
| server/maps/AnimatedMapleMapObject.java | 13 | ACTV | internal/movement | P4.4：setPosition/setFh/setStance 抽成 `movement.Target`，由 `channel.Player` 实现（updatePosition 的驱动面） |
| server/maps/AramiaFireWorks.java | 184 | TODO | internal/mapp |  |
| server/maps/chr.java | 8 | TODO | internal/mapp |  |
| server/maps/cserv.java | 11 | TODO | internal/mapp |  |
| server/maps/Event_DojoAgent.java | 381 | TODO | internal/mapp |  |
| server/maps/Event_PyramidSubway.java | 472 | TODO | internal/mapp |  |
| server/maps/FieldLimitType.java | 31 | TODO | internal/mapp |  |
| server/maps/MapleDoor.java | 138 | TODO | internal/mapp |  |
| server/maps/MapleDragon.java | 40 | TODO | internal/mapp |  |
| server/maps/MapleFoothold.java | 66 | DONE | internal/mapp | P4.3b：`Foothold`（ID/X1/Y1/X2/Y2/Prev/Next）+ `IsWall()`（x1==x2）；`compareTo` 的单边比较器在 `compareFoothold`（`foothold.go`） |
| server/maps/MapleFootholdTree.java | 175 | ACTV | internal/mapp | P4.3b：`FootholdTree` 平坦化（**Java 的四叉树永不细分**——lBound/uBound 从 (0,0) 起按 min/max 扩张，每个 foothold 都通过根节点包含判定，实测 13/13 与 315/315 都在根层）+ `FindBelow`（`x1<=x<=x2 && x1!=x2` 过滤、单边 compareTo 的 `sort.SliceStable`、斜面插值含 double 截断）+ `CalcPointBelow`；`insert`/`findWall`/`findWallR`/`checkRelevantFH` 待消费方（P6 的墙与掉落物）再迁 |
| server/maps/MapleGenericPortal.java | 119 | TODO | internal/mapp |  |
| server/maps/MapleLove.java | 54 | TODO | internal/mapp |  |
| server/maps/MapleMap.java | 4359 | ACTV | internal/mapp | P4.2 玩家集合（addPlayer/removePlayer 的 mapobjects 部分 + MapFactory 惰性建图）；P4.3 spawn/despawn 广播（addPlayer → 对同图其他人 spawnPlayerMapobject + 新人对每个老玩家 + 自身；removePlayer → 广播 removePlayerFromMap；broadcastMessage(source, pkt, false) 的 source 排除）；P4.4 movePlayer 由 `channel.MovePlayer` 的广播 + Player.ApplyMovement 承担（Map.movePlayer 只重设坐标/刷新物件可见性，无物件时为恒等）；**P4.5：`BroadcastRanged` = `broadcastMessage(packet, rangedFrom)` Point 重载（`distanceSq <= GameConstants.maxViewRangeSq` = 10000²，source=null 含发送者），另导出常量 `MaxViewRangeSq`**；foot-hold/life/portal/reactor/掉落/地图特化（送货/月妙等）待 P4.3b/P6 |
| server/maps/MapleMapEffect.java | 35 | TODO | internal/mapp |  |
| server/maps/MapleMapFactory.java | 775 | ACTV | internal/mapp | P4.3b：`MapImagePath`（= `getMapName`，`Map/Map<id/1e8>/%09d.img`）+ `LoadData`/`loadInfo`（info 的 returnMap/*quirk* 910000000→自身、forcedReturn、fieldLimit、mobRate、town/fly/everlast/personalShop、timeLimit、fixedMobCapacity、createMobInterval、onUserEnter/onFirstUserEnter、**顶层 clock 节点**）+ **`info/link` 一跳解析**（4260 张图里 1152 张是桩）+ portal/foothold/life 三段 + `Factory`（缓存 MapData + 失败负缓存，见下）；**不移植**：`addMonsterSpawn` 的双调用 bug（:146-147/:329-330）、`destroyMap` 的返回反转（:230-242）、`CreateInstanceMap`/`DeStorymaps`/`instanceMap`、`MapleNodes`/`loadNodes`(:723-802)、area boss(:542-721)、DB `customLife`(:515-540)、reactor/`back`/`tile`/`obj`/`miniMap`/`ToolTip`/`seat`/`pvp`/`ladderRope`（P6+ 或 Java 无消费方） |
| server/maps/MapleMapItem.java | 146 | TODO | internal/mapp |  |
| server/maps/MapleMapObject.java | 16 | ACTV | internal/mapp | P4.2：mapp.Player 接口（ObjectID）；玩家的 oid = cid（Java setObjectId 抛 UnsupportedOperationException）；P4.3：sendSpawnData → Player.SendSpawnData(sink)（= 把自身 spawn 包写给收件人），另有 DespawnData 供 removePlayer 广播 |
| server/maps/MapleMapObjectType.java | 17 | TODO | internal/mapp |  |
| server/maps/MapleMapPortal.java | 11 | TODO | internal/mapp |  |
| server/maps/MapleMist.java | 121 | TODO | internal/mapp |  |
| server/maps/MapleNodes.java | 178 | TODO | internal/mapp |  |
| server/maps/MapleReactor.java | 180 | TODO | internal/mapp |  |
| server/maps/MapleReactorFactory.java | 70 | TODO | internal/mapp |  |
| server/maps/MapleReactorStats.java | 100 | TODO | internal/mapp |  |
| server/maps/MapleSummon.java | 173 | TODO | internal/mapp |  |
| server/maps/MapScriptMethods.java | 584 | TODO | internal/mapp |  |
| server/maps/reactor.java | 8 | TODO | internal/mapp |  |
| server/maps/ReactorDropEntry.java | 16 | TODO | internal/mapp |  |
| server/maps/SavedLocationType.java | 38 | TODO | internal/mapp |  |
| server/maps/slea.java | 8 | TODO | internal/mapp |  |
| server/maps/SpeedRunType.java | 22 | TODO | internal/mapp |  |
| server/maps/SummonMovementType.java | 18 | TODO | internal/mapp |  |

## pkg server\movement - 4 files, 150 lines -> internal/movement [MERG]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/movement/AbstractLifeMovement.java | 41 | MERG | internal/movement | P4.4：type/position/duration/newstate/newfh 收进 `Fragment` 字段；newfh 的语义怪癖保留（见 MovementParse 行） |
| server/movement/LifeMovement.java | 12 | MERG | internal/movement | P4.4：getType/getDuration/getNewstate/getNewFh 变成 Fragment 字段 |
| server/movement/LifeMovementFragment.java | 10 | MERG | internal/movement | P4.4：serialize/getPosition -> `Fragment.Serialize`/`Pos` |
| server/movement/StaticLifeMovement.java | 87 | MERG | internal/movement | P4.4：`Fragment`（唯一的具体实现；pixelsPerSecond/unk/fh/wui + 6 种序列化布局） |

## pkg server\quest - 7 files, 1240 lines -> internal/script/quest [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/quest/MapleCustomQuest.java | 68 | TODO | internal/script/quest |  |
| server/quest/MapleCustomQuestData.java | 76 | TODO | internal/script/quest |  |
| server/quest/MapleQuest.java | 302 | TODO | internal/script/quest |  |
| server/quest/MapleQuestAction.java | 466 | TODO | internal/script/quest |  |
| server/quest/MapleQuestActionType.java | 41 | TODO | internal/script/quest |  |
| server/quest/MapleQuestRequirement.java | 236 | TODO | internal/script/quest |  |
| server/quest/MapleQuestRequirementType.java | 51 | TODO | internal/script/quest |  |

## pkg server\shops - 7 files, 1151 lines -> internal/custom/shop [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| server/shops/AbstractPlayerStore.java | 381 | TODO | internal/custom/shop |  |
| server/shops/HiredMerchant.java | 201 | TODO | internal/custom/shop |  |
| server/shops/HiredMerchantSave.java | 86 | TODO | internal/custom/shop |  |
| server/shops/IMaplePlayerShop.java | 48 | TODO | internal/custom/shop |  |
| server/shops/MapleMiniGame.java | 322 | TODO | internal/custom/shop |  |
| server/shops/MaplePlayerShop.java | 96 | TODO | internal/custom/shop |  |
| server/shops/MaplePlayerShopItem.java | 17 | TODO | internal/custom/shop |  |

## pkg tools - 23 files, 8289 lines -> internal/protocol+crypto [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| tools/ArrayMap.java | 111 | TODO | internal/protocol+crypto | split per file |
| tools/AttackPair.java | 14 | TODO | internal/protocol+crypto | split per file |
| tools/BitTools.java | 42 | DONE | internal/protocol+crypto | split per file |
| tools/CashShopDumper.java | 149 | TODO | internal/protocol+crypto | split per file |
| tools/CollectionUtil.java | 20 | TODO | internal/protocol+crypto | split per file |
| tools/ConcurrentEnumMap.java | 152 | TODO | internal/protocol+crypto | split per file |
| tools/CPUSampler.java | 268 | TODO | internal/protocol+crypto | split per file |
| tools/DateUtil.java | 22 | TODO | internal/protocol+crypto | split per file |
| tools/Eval.java | 548 | TODO | internal/protocol+crypto | split per file |
| tools/FileoutputUtil.java | 190 | TODO | internal/protocol+crypto | split per file |
| tools/FilePrinter.java | 11 | TODO | internal/protocol+crypto | split per file |
| tools/FixShopItemsPrice.java | 70 | TODO | internal/protocol+crypto | split per file |
| tools/GetInfo.java | 88 | TODO | internal/protocol+crypto | split per file |
| tools/HexTool.java | 89 | DONE | internal/protocol+crypto | split per file |
| tools/IPAddressTool.java | 32 | TODO | internal/protocol+crypto | split per file |
| tools/KoreanDateUtil.java | 51 | TODO | internal/protocol+crypto | split per file |
| tools/MapleAESOFB.java | 130 | DONE | internal/protocol+crypto | split per file |
| tools/MapleCustomEncryption.java | 74 | DONE | internal/protocol+crypto | split per file |
| tools/MaplePacketCreator.java | 5826 | ACTV | internal/packet | P4.2：getCharInfo(WARP_TO_MAP)/temporaryStats_Reset/serverMessage；P4.3：spawnPlayerMapobject（SPAWN_PLAYER 0x00A2 全字段 + CHAR_MAGIC_SPAWN 8 处重复）/removePlayerFromMap（0x00A3）+ addRingInfo(List)/addMarriageRingLook；P4.4：movePlayer（MOVE_PLAYER 0x00BB：int cid + int 0 + 移动列表）与 spawnPlayerMapobject 的 pos/stance 改取真实值；P4.5：getChatText/facialExpression/getWhisper/getWhisperReply/getFindReply(WithMap) 五族（`internal/packet/chat.go`）+ serverNotice（P4.5b）；其余按域拆分增量迁（LOGIN 系仍在 internal/login/packets.go） |
| tools/MockIOSession.java | 156 | TODO | internal/protocol+crypto | split per file |
| tools/Pair.java | 47 | TODO | internal/protocol+crypto | split per file |
| tools/StringUtil.java | 143 | TODO | internal/protocol+crypto | split per file |
| tools/Triple.java | 56 | TODO | internal/protocol+crypto | split per file |

## pkg tools\data - 5 files, 551 lines -> internal/protocol [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| tools/data/ByteArrayByteStream.java | 64 | DONE | internal/protocol |  |
| tools/data/InputStreamByteStream.java | 52 | DONE | internal/protocol |  |
| tools/data/LittleEndianAccessor.java | 223 | DONE | internal/protocol |  |
| tools/data/MaplePacketLittleEndianWriter.java | 154 | DONE | internal/protocol |  |
| tools/data/RandomAccessByteStream.java | 58 | DONE | internal/protocol |  |

## pkg tools\packet - 10 files, 3815 lines -> internal/packet [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| tools/packet/FamilyPacket.java | 262 | TODO | internal/packet |  |
| tools/packet/LoginPacket.java | 246 | ACTV | internal/login/packets.go | P2.2 已迁 getLoginFailed/getAuthSuccess/getGenderNeeded(+hello/ping)；P2.3 已迁 getServerList/getEndOfServerList/getServerStatus；getCharList 系 P2.4 |
| tools/packet/MobPacket.java | 456 | TODO | internal/packet |  |
| tools/packet/MonsterBookPacket.java | 52 | TODO | internal/packet |  |
| tools/packet/MonsterCarnivalPacket.java | 77 | TODO | internal/packet |  |
| tools/packet/MTSCSPacket.java | 1065 | TODO | internal/packet |  |
| tools/packet/PacketHelper.java | 684 | ACTV | internal/packet | P2.4：addCharLook；P4.2：addCharStats/getTime → internal/packet（login/channel 共享）+ addCharacterInfo（getCharInfo 用，背包/技能/任务等空段）；P4.3：addCharLook 也移入 internal/packet（带 mega 参数，login=mega=true、频道 spawn=mega=false；login 侧改调 packet.AddCharLook）+ addAnnounceBox 空态；P4.4：serializeMovementList → movement.SerializeMovementList；inventoryitems 实物序列化待 P5.2 |
| tools/packet/PetPacket.java | 226 | TODO | internal/packet |  |
| tools/packet/PlayerShopPacket.java | 573 | TODO | internal/packet |  |
| tools/packet/UIPacket.java | 174 | TODO | internal/packet |  |

## pkg tools\wztosql - 4 files, 1311 lines -> tools/wztosql [TODO]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| tools/wztosql/AddCashItemToDB.java | 31 | TODO | tools/wztosql |  |
| tools/wztosql/DumpMobSkills.java | 154 | TODO | tools/wztosql |  |
| tools/wztosql/MonsterDropCreator.java | 887 | DONE | internal/dropgen + tools/wztosql | P3.5：wz 侧重建 + `-diff` 漂移对比，不写库（本服掉落以库为准） |
| tools/wztosql/WzStringDumper.java | 239 | TODO | tools/wztosql |  |

## pkg zevms\data - 1 files, 29 lines -> - [SKIP]

| java file | lines | status | go target | note |
|---|---|---|---|---|
| zevms/data/DataPack.java | 29 | SKIP | - | old launcher leftover |

