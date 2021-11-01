cd /d %~dp0
set GOOS=linux
go build -ldflags "-s -w"
move TimeManager bot