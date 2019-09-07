GOMASPRID_USER=gomasprid

bin/gomasprid: gomasprid/gomasprid.go config.go daemon.go
	@mkdir -p bin
	go build -o $@ $<

clean:
	rm gomasprid

install_bin: bin/gomasprid
	install $< /usr/local/bin/gomasprid

install_user:
	useradd -r -s /bin/false ${GOMASPRID_USER} || true

install_config: gomasprid/config.toml install_user
	install -o ${GOMASPRID_USER} -D -m 600 $< /etc/gomasprid/config.toml

install_service: gomasprid/gomasprid.service install_user
	install $< /etc/systemd/system/gomasprid.service
	systemctl daemon-reload

install: install_bin install_config install_service
