.PHONY: dev

dev:
	go run cmd/main.go

build:
	go build -o ./bin/inkdrop ./cmd

run:
	chmod 755 ./bin/inkdrop
	./bin/inkdrop

create:
	sudo mkdir -p bin
	sudo chmod 7777 bin

reload:
	sudo systemctl daemon-reload

start:
	sudo systemctl start inkdrop

enable:
	sudo systemctl enable inkdrop

stop:
	sudo systemctl stop inkdrop

restart:
	sudo systemctl restart inkdrop

status:
	sudo systemctl status inkdrop

log:
	journalctl -u inkdrop -f

pull:
	git pull

update: pull build reload restart status log
