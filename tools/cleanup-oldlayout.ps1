# cleanup-oldlayout.ps1 -- remove pre-refactor flat-layout leftovers that
# came back with the remote rebase (they break `go build ./...`).
Set-Location I:\GMS
foreach ($p in @('handing', 'client', 'opcode', 'tools\data',
                 'recvops.properties', 'sendops.properties', 'main.go')) {
    if (Test-Path $p) {
        Remove-Item $p -Recurse -Force
        Write-Host "removed $p"
    } else {
        Write-Host "absent  $p"
    }
}
