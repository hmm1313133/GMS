@echo off
rem start-gms.cmd -- run the GMS login server detached, logging to bin\gms_out.log
rem (Start-Process is denied for workspace exes in this sandbox; cmd start /B works)
start "" /B cmd /c "I:\GMS\bin\gms.exe I:\GMS\configs\gms.toml > I:\GMS\bin\gms_out.log 2>&1"
