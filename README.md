## Overpush

![Overpush](.README.md/overpush.png)

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
[`overpush.toml`](examples/etc/overpush.toml) can be exported as environment
variable, by separating scopes using `_` and prepend `OVERPUSH` to it.

## Run

All that's needed is a [configuration](#configure) and Overpush can be launched:

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

**Note:** It is advisable to run Overpush under its own, dedicated daemon user
(`overpush` in this example), so make sure to either adjust `directory` as well
as `user` or create a user called `overpush`.

### OpenBSD rc

As before, create a configuration file under `/etc/overpush.toml`.

Then copy the [example rc.d script](examples/etc/rc.d/overpush) to
`/etc/rc.d/overpush` and copy the binary to e.g. `/usr/local/bin/overpush`. Last
but not least, update the `/etc/rc.conf.local` file to contain the following
line:

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

Please find an
[example script 
here](https://github.com/mrusme/dotfiles/blob/master/usr/local/bin/overpush)
that you can use as a command-line API client for both, Pushover and Overpush,
to submit notifications. As Overpush does not yet have 100% feature parity, not
all features might be available.

### Custom

Overpush can handle all sorts of custom messages by configuring dedicated 
applications in [its config](examples/etc/overpush.toml).

Here are some examples:

#### CrowdSec

Add the following application to your Overpush config:

```toml
  [[Users.Applications]]
  Token = "XXX"
  Name = "CrowdSec"
  IconPath = ""
  Target = "your_target"
  Format = "custom"
  CustomFormat.Message = "body.alerts.0.message"
  CustomFormat.Title = "body.alerts.0.scenario"
```

Edit the CrowdSec config `notifications/http.yaml` (under 
`/etc/crowdsec/notifications/http.yaml` or 
`/usr/local/etc/crowdsec/notifications/http.yaml`) as follows:

```yaml
type: http
name: http_default

log_level: info

format: |
  {"alerts":{{.|toJson}}}

url: https://my.overpush.net/XXX

method: POST

headers:
  Content-Type: application/json

# skip_tls_verification:  true 
```

Set `XXX` to the unique token of the Overpush application.

#### Grafana

Add the following application to your Overpush config:

```toml
  [[Users.Applications]]
  Token = "XXX"
  Name = "Grafana"
  IconPath = ""
  Target = "your_target"
  Format = "custom"
  CustomFormat.Message = "body.message"
  CustomFormat.Title = "body.title"
  CustomFormat.URL = "body.externalURL"
```

Create a new contact point in your Grafana under
`/alerting/notifications/receivers/new`, choose the _Webhook_ integration add
set your Overpush instance:

```
https://my.overpush.net/XXX
```

Set `XXX` to the unique token of the Overpush application.

## FAQ

### Why?

[That's why](https://xn--gckvb8fzb.com/goodbye-pushover-hello-overpush/).

### No OTR? No OMEMO?

Nope, none of those. The XMPP ecosystem is a bit of a can of worms in this
regard. First, when using _modern_ languages like Go, there are very few XMPP
libraries available. The ones that do exist generally don't support OTR or
OMEMO. Adding support would require either implementing these protocols from
scratch or interfacing with a low-level C library.

Even if someone were willing to go through that effort, they'd run into the
second major issue with XMPP: fragmentation. For example, if someone were to
implement OMEMO in Go today, they would likely choose
[OMEMO 0.8.3 or newer](https://xmpp.org/extensions/xep-0384.html).
Unfortunately, many
[XMPP clients are still stuck on version 0.3.0](https://xmpp.org/extensions/#xep-0384-implementations),
which uses AES-128-GCM -- an encryption algorithm considered weaker by modern
standards (e.g., compared to what Matrix.org or Signal use). As a result, most
implementations would have to fall back to a significantly older and less secure
version of OMEMO.

### Workaround

For notifications that require content encryption, good old GPG can be used:

```sh
curl -s \
  --form-string "token=$OP_TOKEN" \
  --form-string "user=$OP_USER" \
  --form-string "message=$(gpg -e -r KEY_ID --armor -o - file_to_encrypt)" \
  "$OP_URL"
```

On the receiving end, for example, Android running
[Conversations](https://f-droid.org/en/packages/eu.siacs.conversations/), the
message can be shared to
[OpenKeychain](https://f-droid.org/en/packages/org.sufficientlysecure.keychain/)
using the standard Android sharing popup. OpenKeychain will then decrypt and
display the message content.

This is the simplest way to enable encrypted notifications without relying on
XMPP clients to support modern encryption standards.
