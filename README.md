# GoMailSprinkler

This project aims to solve the following problem:
1. A sender sends a mail to a pre-setup mailing address
2. A programm listens to the incoming mails under this address and
3. Forwards the mail to a list of recipients

Since I was not able to find a quick solution for this, I wrote a short implementation in go.

**WORK IN PROGRESS**

## Intallation
To build and install gomasprid, together with a configuration file, the systemd service and a dedicated service user execute
```bash
sudo make install
```

To enable autostart type
```bash
sudo systemctl enable gomasprid
```

Other useful commands
```bash
sudo systemctl start gomasprid
sudo journalctl -f -u gomasprid
```