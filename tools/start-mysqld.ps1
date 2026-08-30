# start-mysqld.ps1 -- start the bundled MySQL 5.5.53 for live smoke tests.
# The install path contains CJK chars; command-line passing of such paths
# gets mojibake'd, so we resolve them via wildcards inside PowerShell instead.
# Usage: powershell -NoProfile -ExecutionPolicy Bypass -File tools\start-mysqld.ps1

$mysqld = Get-Item 'K:\079MAX2*\mysql\MySQL\bin\mysqld.exe' -ErrorAction Stop | Select-Object -First 1
$ini    = Get-Item 'K:\079MAX2*\mysql\MySQL\my.ini' -ErrorAction Stop | Select-Object -First 1
Write-Host "mysqld: $($mysqld.FullName)"
Write-Host "ini   : $($ini.FullName)"

# already listening on 3306? then nothing to do
$listen = netstat -ano | Select-String ':3306\s'
if ($listen) {
    Write-Host 'mysqld already listening on 3306, skip start.'
    exit 0
}

Start-Process -FilePath $mysqld.FullName `
    -ArgumentList ('"--defaults-file=' + $ini.FullName + '"'), '--console' `
    -WindowStyle Hidden `
    -RedirectStandardOutput 'I:\GMS\bin\mysqld_out.log' `
    -RedirectStandardError  'I:\GMS\bin\mysqld_err.log'

# wait for the port (up to 30s)
for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 1
    $listen = netstat -ano | Select-String ':3306\s.*LISTENING'
    if ($listen) {
        Write-Host "mysqld up after $($i+1)s: $listen"
        exit 0
    }
}
Write-Host 'ERROR: mysqld did not come up within 30s; check bin\mysqld_err.log'
exit 1
