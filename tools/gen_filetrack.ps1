# Generate docs/FILETRACK.md baseline (file-level tracking) from I:\Zevms Java tree.
# NOTE: keep this script pure ASCII (harness pwsh reads no-BOM scripts as ANSI).
# Usage: & tools/gen_filetrack.ps1   (regenerates baseline; manual status edits will be lost)
$srcRoot = 'I:\Zevms\src\main\java'
$out     = 'I:\GMS\docs\FILETRACK.md'

$pkgMap = @{
  'a'                = @{ Go='-'; Disp='SKIP'; Note='launcher misc' }
  'a\http'           = @{ Go='-'; Disp='SKIP'; Note='downloader' }
  'abc'              = @{ Go='internal/anticheat+config'; Disp='TODO'; Note='split: detect/config/util' }
  'abc\sancu'        = @{ Go='-'; Disp='SKIP'; Note='file-delete demo' }
  'abc\www'          = @{ Go='-'; Disp='SKIP'; Note='browser demo' }
  'abc\yunxing'      = @{ Go='-'; Disp='SKIP'; Note='thread demo' }
  'client'           = @{ Go='internal/model'; Disp='TODO'; Note='' }
  'client\about2'    = @{ Go='-'; Disp='SKIP'; Note='swing about dialog' }
  'client\anticheat' = @{ Go='internal/anticheat'; Disp='TODO'; Note='' }
  'client\inventory' = @{ Go='internal/model/inventory'; Disp='TODO'; Note='' }
  'client\messages'  = @{ Go='internal/channel/command'; Disp='TODO'; Note='' }
  'client\status'    = @{ Go='internal/model'; Disp='TODO'; Note='' }
  'client\zev'       = @{ Go='-'; Disp='SKIP'; Note='legacy leftover' }
  'com'              = @{ Go='-'; Disp='SKIP'; Note='progressbar demo' }
  'com\cyb'          = @{ Go='-'; Disp='SKIP'; Note='cpu monitor demo' }
  'constants'        = @{ Go='internal/config'; Disp='TODO'; Note='' }
  'database'         = @{ Go='internal/database'; Disp='TODO'; Note='' }
  'database1'        = @{ Go='-'; Disp='SKIP'; Note='dup of database' }
  'download'         = @{ Go='-'; Disp='SKIP'; Note='updater' }
  'fumo'             = @{ Go='internal/custom/fumo'; Disp='TODO'; Note='' }
  'gui'              = @{ Go='-'; Disp='SKIP'; Note='swing gui -> web panel' }
  'handling'         = @{ Go='internal/protocol'; Disp='TODO'; Note='opcode/dispatch' }
  'handling\cashshop'      = @{ Go='internal/cashshop'; Disp='TODO'; Note='' }
  'handling\cashshop\handler' = @{ Go='internal/cashshop/handler'; Disp='TODO'; Note='' }
  'handling\channel'       = @{ Go='internal/channel'; Disp='TODO'; Note='' }
  'handling\channel\handler' = @{ Go='internal/channel/handler'; Disp='TODO'; Note='' }
  'handling\login'  = @{ Go='internal/login'; Disp='TODO'; Note='' }
  'handling\login\handler' = @{ Go='internal/login/handler'; Disp='TODO'; Note='' }
  'handling\netty'  = @{ Go='internal/netw'; Disp='TODO'; Note='' }
  'handling\world'  = @{ Go='internal/world'; Disp='TODO'; Note='' }
  'handling\world\family' = @{ Go='internal/world/family'; Disp='TODO'; Note='' }
  'handling\world\guild'  = @{ Go='internal/world/guild';  Disp='TODO'; Note='' }
  'module\system\common' = @{ Go='-'; Disp='SKIP'; Note='old launcher leftover' }
  'provider'        = @{ Go='internal/wzs'; Disp='TODO'; Note='' }
  'provider\WzXML'  = @{ Go='internal/wzs'; Disp='TODO'; Note='' }
  'pvp'             = @{ Go='internal/custom/pvp'; Disp='TODO'; Note='' }
  'quest'           = @{ Go='internal/script/quest'; Disp='TODO'; Note='' }
  'scripting'       = @{ Go='internal/script'; Disp='TODO'; Note='' }
  'server'          = @{ Go='internal/server-domain'; Disp='TODO'; Note='split per file' }
  'server\custom\auction'  = @{ Go='internal/custom/auction';  Disp='TODO'; Note='' }
  'server\custom\auction1' = @{ Go='internal/custom/auction';  Disp='MERG'; Note='into auction' }
  'server\custom\bankitem'  = @{ Go='internal/custom/bank';   Disp='TODO'; Note='' }
  'server\custom\bankitem1' = @{ Go='internal/custom/bank';   Disp='MERG'; Note='into bank' }
  'server\custom\bankitem2' = @{ Go='internal/custom/bank';   Disp='MERG'; Note='into bank' }
  'server\custom\bossrank'  = @{ Go='internal/custom/rank';   Disp='TODO'; Note='base impl' }
  'server\custom\capture'   = @{ Go='internal/custom/capture'; Disp='TODO'; Note='' }
  'server\custom\forum'     = @{ Go='internal/custom/forum';   Disp='TODO'; Note='optional' }
  'server\custom\rank'      = @{ Go='internal/custom/rank';    Disp='TODO'; Note='' }
  'server\custom\respawn'   = @{ Go='internal/mapp';           Disp='TODO'; Note='' }
  'server\custom\treasure_house' = @{ Go='internal/custom/treasure'; Disp='TODO'; Note='optional' }
  'server\events'  = @{ Go='internal/mapp/events';    Disp='TODO'; Note='' }
  'server\life'    = @{ Go='internal/life';           Disp='TODO'; Note='' }
  'server\maps'    = @{ Go='internal/mapp';           Disp='TODO'; Note='' }
  'server\movement' = @{ Go='internal/channel/movement'; Disp='TODO'; Note='' }
  'server\quest'   = @{ Go='internal/script/quest';   Disp='TODO'; Note='' }
  'server\shops'   = @{ Go='internal/custom/shop';    Disp='TODO'; Note='' }
  'tools'          = @{ Go='internal/protocol+crypto'; Disp='TODO'; Note='split per file' }
  'tools\data'     = @{ Go='internal/protocol';       Disp='TODO'; Note='' }
  'tools\packet'   = @{ Go='internal/packet';         Disp='TODO'; Note='' }
  'tools\wztosql'  = @{ Go='tools/wztosql';           Disp='TODO'; Note='' }
  'zevms\data'     = @{ Go='-'; Disp='SKIP'; Note='old launcher leftover' }
  '(root)'         = @{ Go='-'; Disp='SKIP'; Note='demo' }
}

function Get-PkgInfo([string]$pkg) {
  if ($pkgMap.ContainsKey($pkg)) { return $pkgMap[$pkg] }
  if ($pkg -match '^server\\custom\\bossrank\d+$') { return @{ Go='internal/custom/rank'; Disp='MERG'; Note='into rank multi-instance' } }
  return @{ Go='internal/TBD'; Disp='TODO'; Note='' }
}

$files = Get-ChildItem -Recurse -Include *.java -Path $srcRoot | ForEach-Object {
  $rel = $_.FullName.Replace("$srcRoot\", '')
  $pkg = Split-Path $rel -Parent
  if (-not $pkg) { $pkg = '(root)' }
  $info = Get-PkgInfo $pkg
  [PSCustomObject]@{
    Pkg   = $pkg
    File  = Split-Path $rel -Leaf
    Rel   = $rel.Replace('\','/')
    Lines = (Get-Content $_.FullName | Measure-Object -Line).Lines
    Go    = $info.Go
    Disp  = $info.Disp
    Note  = $info.Note
  }
} | Sort-Object Pkg, File

$sb = New-Object System.Text.StringBuilder
[void]$sb.AppendLine('# FILETRACK - file-level migration tracking')
[void]$sb.AppendLine('')
[void]$sb.AppendLine('<!-- Legend: TODO=not started ACTV=in progress DONE=done (Go impl + tests) MERG=merged into other impl SKIP=not migrated (see Note) -->')
[void]$sb.AppendLine('<!-- One row per Java file under I:\Zevms\src\main\java. Update the Status column as work completes. -->')
[void]$sb.AppendLine('')
$total  = ($files | Measure-Object).Count
$todo   = ($files | Where-Object { $_.Disp -eq 'TODO' -or $_.Disp -eq 'ACTV' } | Measure-Object).Count
$done   = ($files | Where-Object { $_.Disp -eq 'DONE' } | Measure-Object).Count
$merged = ($files | Where-Object { $_.Disp -eq 'MERG' } | Measure-Object).Count
$skip   = ($files | Where-Object { $_.Disp -eq 'SKIP' } | Measure-Object).Count
[void]$sb.AppendLine('## Summary')
[void]$sb.AppendLine('')
[void]$sb.AppendLine('| metric | value |')
[void]$sb.AppendLine('|---|---|')
[void]$sb.AppendLine("| java files total | $total |")
[void]$sb.AppendLine("| to migrate (TODO+ACTV) | $todo |")
[void]$sb.AppendLine("| done (DONE) | $done |")
[void]$sb.AppendLine("| merged (MERG) | $merged |")
[void]$sb.AppendLine("| skipped (SKIP) | $skip |")
[void]$sb.AppendLine('')

foreach ($grp in ($files | Group-Object Pkg | Sort-Object Name)) {
  $first = $grp.Group[0]
  $pkgDisp = $first.Disp
  $sum = ($grp.Group.Lines | Measure-Object -Sum).Sum
  [void]$sb.AppendLine("## pkg $($grp.Name) - $($grp.Count) files, $sum lines -> $($first.Go) [$pkgDisp]")
  [void]$sb.AppendLine('')
  [void]$sb.AppendLine('| java file | lines | status | go target | note |')
  [void]$sb.AppendLine('|---|---|---|---|---|')
  foreach ($f in ($grp.Group | Sort-Object File)) {
    [void]$sb.AppendLine("| $($f.Rel) | $($f.Lines) | $($f.Disp) | $($f.Go) | $($f.Note) |")
  }
  [void]$sb.AppendLine('')
}

[System.IO.File]::WriteAllText($out, $sb.ToString(), (New-Object System.Text.UTF8Encoding $false))
"generated: $out (total=$total todo=$todo done=$done merged=$merged skip=$skip)"
