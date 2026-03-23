@echo off
echo Building Windows Audio Controller...

echo Building Console Version (winvol.exe)
go build -o winvol.exe .

echo Building Background/Hidden Version (winvol_bg.exe)
go build -ldflags "-H windowsgui" -o winvol_bg.exe .

echo Build complete!
pause
