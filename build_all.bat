#windows
go build

# linux
set GOOS=linux
set GOARCH=amd64
go build -o hdmapFetecher_linux_amd64
