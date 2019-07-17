@echo Windows Install Script
go get github.com/docker/docker/client
go get github.com/docker/go-connections/nat
go get github.com/oklog/run
del /s /q ..\github.com\docker\docker\vendor\github.com\docker\go-connections\nat
go build ./...
go build tlex
@echo "Run ->"
@echo 			tlex.exe