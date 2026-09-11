param([string]$Path = 'I:\GMS\bin\gms_out.log', [int]$Tail = 0)
$fs = [IO.File]::Open($Path, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::ReadWrite)
$sr = New-Object IO.StreamReader($fs)
$text = $sr.ReadToEnd()
$sr.Close(); $fs.Close()
if ($Tail -gt 0) {
  $lines = $text -split "`r?`n"
  $start = [Math]::Max(0, $lines.Count - $Tail)
  $lines[$start..($lines.Count - 1)]
} else {
  $text
}
