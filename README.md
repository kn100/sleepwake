# sleepwake
Some garbage to make my pc do stuff when it goes into and out of suspension.

If you too need your PC to do stuff when it goes to sleep, and then do other
more different stuff when it wakes up, you can define whatever code you like
in the `onSleep()` and `onWake()` funcs, make the binary, copy it somewhere 
(I suggest /opt), and then add a systemd unit for it. Start the service, et
voila, you too get my garbage.

Your tasks should take less than 5 seconds, since that's how long you get 
by default from Systemd to do whatever it is you need to do before the system
goes to sleep. If you need longer, look into configuring `InhibitDelayMaxSec`
(logind.conf).



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