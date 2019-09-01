# GoMailSprinkler

This project aims to solve the following problem:
1. A sender sends a mail to a pre-setup mailing address
2. A programm listens to the incoming mails under this address and
3. Forwards the mail to a list of recipients

Since I was not able to find a quick solution for this, I wrote a short implementation in go.

**WORK IN PROGRESS**

## Intallation

```bash
go get github.com/cxkoda/gomaspri
sudo go build -o /bin/gomaspri github.com/cxkoda/gomaspri/main
```

Add service user and config
```bash
sudo useradd -r -s /bin/false gomaspri

sudo mkdir -p /etc/gomaspri
sudo cp main/config.toml /etc/gomaspri/.
sudo chmod -R gomaspri /etc/gomaspri
sudo chmod o-r /etc/gomaspri

sudo cp main/gomaspri.service /lib/systemd/system/.
sudo chmod 755 /lib/systemd/system/gomaspri.service
```

Automatic startup
```bash
sudo systemctl enable gomaspri
```


```bash
sudo systemctl start gomaspri
sudo journalctl -f -u gomaspri
```