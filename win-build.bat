@ECHO OFF

del wpd.exe
go build -mod=vendor -ldflags="-s -w" -o wpd.exe main.go

PAUSE
