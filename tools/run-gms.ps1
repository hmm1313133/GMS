# run-gms.ps1 -- build + run the GMS login server in a background job and
# show its first log lines (Start-Process is denied for workspace exes in
# this sandbox; Start-Job works).
# Usage: powershell -NoProfile -ExecutionPolicy Bypass -File tools\run-gms.ps1

Set-Location I:\GMS

if (Get-Process gms -ErrorAction SilentlyContinue) {
    Stop-Process -Name gms -Force
    Start-Sleep -Seconds 1
}

$job = Start-Job -ScriptBlock {
    Set-Location I:\GMS
    & I:\GMS\bin\gms.exe configs\gms.toml 2>&1 |
        Out-File -Encoding utf8 I:\GMS\bin\gms_out.log
}
Write-Host "job id: $($job.Id)"

Start-Sleep -Seconds 3
Write-Host '--- gms_out.log ---'
if (Test-Path I:\GMS\bin\gms_out.log) {
    Get-Content I:\GMS\bin\gms_out.log
}
$listen = netstat -ano | Select-String ':8484\s.*LISTENING'
Write-Host '--- port 8484 ---'
Write-Host "$listen"
