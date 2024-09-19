Overpush
--------

A self-hosted, drop-in replacement for [Pushover](https://pushover.net), that 
uses XMPP as delivery method and offers the same API for submitting messages, so 
that existing setups (e.g. Grafana) can continue working and only require 
changing the API URL.


## Build

```sh
$ go build
```

## Configure

Overpush will try to read the `overpush.toml` file from one of the following
paths:

- `/etc/overpush.toml`
- `$XDG_CONFIG_HOME/overpush.toml`
- `$HOME/.config/overpush.toml`
- `$HOME/overpush.toml`
- `$PWD/overpush.toml`

Every configuration key available in the example 
[`overpush.toml`](examples/etc/overpush.toml) can be exported as
environment variable, by separating scopes using `_` and prepend `OVERPUSH` to
it. 


## Run

All that's needed is a [configuration](#configure) and Overpush can be
launched:

```sh
$ overpush
```


### Supervisor

To run Overpush via `supervisord`, create a config like this inside
`/etc/supervisord.conf` or `/etc/supervisor/conf.d/overpush.conf`:

```ini
[program:overpush]
command=/path/to/binary/of/overpush
process_name=%(program_name)s
numprocs=1
directory=/home/overpush
autostart=true
autorestart=unexpected
startsecs=10
startretries=3
exitcodes=0
stopsignal=TERM
stopwaitsecs=10
user=overpush
redirect_stderr=false
stdout_logfile=/var/log/overpush.out.log
stdout_logfile_maxbytes=1MB
stdout_logfile_backups=10
stdout_capture_maxbytes=1MB
stdout_events_enabled=false
stderr_logfile=/var/log/overpush.err.log
stderr_logfile_maxbytes=1MB
stderr_logfile_backups=10
stderr_capture_maxbytes=1MB
stderr_events_enabled=false
```

**Note:** It is advisable to run Overpush under its own, dedicated daemon
user (`overpush` in this example), so make sure to either adjust `directory`
as well as `user` or create a user called `overpush`.


### OpenBSD rc

As before, create a configuration file under `/etc/overpush.toml`.

Then copy the [example rc.d script](examples/etc/rc.d/overpush) to
`/etc/rc.d/overpush` and copy the binary to e.g.
`/usr/local/bin/overpush`. Last but not least, update the `/etc/rc.conf.local`
file to contain the following line:

```conf
overpush_user="_overpush"
```

It is advisable to run overpush as a dedicated user, hence create the
`_overpush` daemon account or adjust the line above according to your setup.

You can now run Overpush by enabling and starting the service:

```sh
rcctl enable overpush
rcctl start overpush
```


### systemd

TODO


### Docker

TODO


### Kubernetes

TODO


### Aamazon Web Services Lambda Function

TODO


### Google Cloud Function

TODO


## Use 

### Pushover clients

The [official Pushover API documentation](https://pushover.net/api#messages) 
shows how to submit a message to the `/1/messages.json` endpoint. Replacing 
Pushover with Overpush requires your tooling to be able to change the endpoint 
URL you your own server's.

Please find an [example script 
here](https://github.com/mrusme/dotfiles/blob/master/usr/local/bin/pushover) 
that you can use as a command-line API client for both, Pushover and Overpush, 
to submit notifications. As Overpush does not yet have 100% feature parity, not 
all features might be available.


### Grafana

Overpush supports a `/grafana` endpoint, that lets use add it as Grafana 
*Contant point*. To do so, create a new contact point in your Grafana under 
`/alerting/notifications/receivers/new`, choose the *Webhook* integration add 
set your Overpush instance with the `/grafana` path as URL:

```
https://my.overpush.net/grafana?user=XXX&token=YYYY
```

Set the `user` and `token` parameters according to your Overpush configuration. 
They represent the same values as your Pushover client credentials.


## FAQ

### Why?

[That's why](https://xn--gckvb8fzb.com/goodbye-pushover-hello-overpush/).

