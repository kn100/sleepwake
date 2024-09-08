# sleepwake
some garbage to make my pc do stuff when it goes into and out of suspension.

Example systemd unit
```
[Unit]
Description=Run SleepWake on Startup

[Service]
ExecStart=/opt/sleepwake
WorkingDirectory=/opt
Restart=on-failure

[Install]
WantedBy=multi-user.target
```