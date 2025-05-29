## Overpush

![Overpush](.README.md/overpush.png)

[![Static 
Badge](https://img.shields.io/badge/Join_on_Matrix-green?style=for-the-badge&logo=element&logoColor=%23ffffff&label=Chat&labelColor=%23333&color=%230DBD8B&link=https%3A%2F%2Fmatrix.to%2F%23%2F%2521PHlbgZTdrhjkCJrfVY%253Amatrix.org)](https://matrix.to/#/%21PHlbgZTdrhjkCJrfVY%3Amatrix.org)

**Overpush** is a self-hosted, drop-in replacement for
[Pushover](https://pushover.net) that can use XMPP, as well as a wide variety of
other services ([see below](#apprise)) as the delivery method while maintaining
full compatibility with the Pushover API and also offering a flexible HTTP
webhooks endpoint. This allows existing setups to continue functioning with
minimal changes, while allowing more complex setups further down the road.

Think of Overpush as a _bridge_ between HTTP requests (_webhooks_) and various
_target_ services, such as XMPP, Matrix, Signal, and others:

```
                    ┌────────────────┐                    
                    │                │                    
                    │  HTTP REQUEST  │                    
                    │                │                    
                    └────────┬───────┘                    
                             │                            
                             │                            
                             │                            
                    ┌────────▼───────┐                    
                    │                │                    
                    │  OVERPUSH API  │                    
                    │                │                    
                    │────────────────│                    
                    │     REDIS      │                    
                    │────────────────│                    
                    │                │                    
                    │     WORKER     │                    
                    │                │                    
                    └────────┬───────┘                    
                             │                            
        ┌────────────────────┼──────────────────┐         
        │                    │                  │         
┌───────▼────────┐  ┌────────▼───────┐  ┌───────▼────────┐
│                │  │                │  │                │
│     SIGNAL     │  │      XMPP      │  │     MATRIX     │
│                │  │                │  │                │
└────────────────┘  └────────────────┘  └────────────────┘
```

The Overpush API accepts HTTP `POST` requests in multiple formats, including:

- `application/x-www-form-urlencoded`
- `multipart/form-data`
- `application/json`
- `application/xml`/`text/xml`

The [configuration file](examples/etc/overpush.toml) allows you to define custom
templates using Go's `text/template` syntax to extract and transform data from
webhook payloads.

Internally, Overpush uses [`asynq`](https://github.com/hibiken/asynq) to queue
messages for processing by a background worker. This requires a Redis-compatible
server as a backend. You can run your own instance (e.g. Redis, Valkey, KeyDB,
DragonflyDB) or use the free _Essentials_ plan from
[Redis Cloud](https://redis.io/pricing/), which is reliable and sufficient for
most use cases.

The Overpush worker has native support for XMPP as a _target_. For other
services, it integrates with [Apprise](https://github.com/caronc/apprise),
requiring the `apprise` CLI tool to be available on the same host.

## Build

To build Overpush, simply clone this repository and run the following command
inside your local copy:

```sh
$ go build
```

## Configure

The Overpush configuration is organized into four main sections:

- `[Server]`: Specifies server settings, like the IP address and port on which
  Overpush should run.
- `[Redis]`: Configures the connection to a Redis server or cluster.
- `[Users]`: Defines individual users and their settings.
- `[Targets]`: Sets up message targets (or _destinations_) for each user.

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

### Sources

The Overpush API accepts requests to the legacy `/1/messages.json` endpoint used
by Pushover, which allows you to simply flip the host/domain in your
configurations to drop in Overpush instead of Pushover. In addition, Overpush
offers a flexible HTTP `POST` endpoint on `/:token` (where `:token` is an
`Application`'s `Token`), which can be used to retrieve various different
webhook formats.

#### Pushover clients

The [official Pushover API documentation](https://pushover.net/api#messages)
shows how to submit a message to the `/1/messages.json` endpoint. Replacing
Pushover with Overpush only requires your tooling to support changing the
endpoint URL to your own server.

You can find an
[example script here](https://github.com/mrusme/dotfiles/blob/master/usr/local/bin/overpush)
that serves as a command-line API client for both Pushover and Overpush to
submit notifications. Since Overpush does not yet offer 100% feature parity with
Pushover, some features are not available.

#### Custom HTTP Webhooks

Overpush can handle a wide variety of custom webhooks by configuring dedicated
`Applications` in [its config](examples/etc/overpush.toml). Here are some
examples:

##### CrowdSec

Add the following application to your Overpush config:

```toml
[[Users.Applications]]
Token = "XXX"
Name = "CrowdSec"
IconPath = ""
Target = "your_target"
Format = "custom"
CustomFormat.Message = '{{ webhook "body.alerts.0.message" }}'
CustomFormat.Title = 'CrowdSec: {{ webhook "body.alerts.0.scenario" }}'
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

##### Grafana

Add the following application to your Overpush config:

```toml
[[Users.Applications]]
Token = "XXX"
Name = "Grafana"
IconPath = ""
Target = "your_target"
Format = "custom"
CustomFormat.Message = '{{ webhook "body.message" }}'
CustomFormat.Title = '{{ webhook "body.title" }}'
CustomFormat.URL = '{{ webhook "body.externalURL" }}'
```

Create a new contact point in your Grafana under
`/alerting/notifications/receivers/new`, choose the _Webhook_ integration add
set your Overpush instance:

```
https://my.overpush.net/XXX
```

Set `XXX` to the unique token of the Overpush application.

### Targets

_Targets_ are the target services that messages should be routed to. Overpush
supports XMPP out of the box, but can use Apprise to forward messages to a wide
range of different services.

Each `Application` must specify a `Target`. Multiple `Application`
configurations might use the same target.

#### XMPP (built-in)

Overpush supports XMPP (without OTR/OMEMO) out of the box, without any
additional software. The configuration for the XMPP target might look like this:

```toml
[[Targets]]
ID = "your_target"
Type = "xmpp"

  [Targets.Args]
  server = "conversations.im"
  tls = "true"
  username = "my_bot@conversations.im"
  password = "hunter2"
  destination = "your-user@your-xmpp.im"
```

To use this target, specify its ID inside an `Application` configuration:

```toml
...
Target = "your_target"
...
```

#### Apprise

Overpush supports the following platforms via
[Apprise](https://github.com/caronc/apprise):

- [AWS SES](https://github.com/caronc/apprise/wiki/Notify_ses)
- [Bark](https://github.com/caronc/apprise/wiki/Notify_bark)
- [BlueSky](https://github.com/caronc/apprise/wiki/Notify_bluesky)
- [Chanify](https://github.com/caronc/apprise/wiki/Notify_chanify)
- [Discord](https://github.com/caronc/apprise/wiki/Notify_discord)
- [Emby](https://github.com/caronc/apprise/wiki/Notify_emby)
- [Enigma2](https://github.com/caronc/apprise/wiki/Notify_enigma2)
- [FCM](https://github.com/caronc/apprise/wiki/Notify_fcm)
- [Feishu](https://github.com/caronc/apprise/wiki/Notify_feishu)
- [Flock](https://github.com/caronc/apprise/wiki/Notify_flock)
- [Google Chat](https://github.com/caronc/apprise/wiki/Notify_googlechat)
- [Gotify](https://github.com/caronc/apprise/wiki/Notify_gotify)
- [Growl](https://github.com/caronc/apprise/wiki/Notify_growl)
- [Guilded](https://github.com/caronc/apprise/wiki/Notify_guilded)
- [Home Assistant](https://github.com/caronc/apprise/wiki/Notify_homeassistant)
- [IFTTT](https://github.com/caronc/apprise/wiki/Notify_ifttt)
- [Join](https://github.com/caronc/apprise/wiki/Notify_join)
- [KODI](https://github.com/caronc/apprise/wiki/Notify_kodi)
- [Kumulos](https://github.com/caronc/apprise/wiki/Notify_kumulos)
- [LaMetric Time](https://github.com/caronc/apprise/wiki/Notify_lametric)
- [Line](https://github.com/caronc/apprise/wiki/Notify_line)
- [LunaSea](https://github.com/caronc/apprise/wiki/Notify_lunasea)
- [Mailgun](https://github.com/caronc/apprise/wiki/Notify_mailgun)
- [Mastodon](https://github.com/caronc/apprise/wiki/Notify_mastodon)
- [Matrix](https://github.com/caronc/apprise/wiki/Notify_matrix)
- [Mattermost](https://github.com/caronc/apprise/wiki/Notify_mattermost)
- [Microsoft Power Automate / Workflows (MSTeams)](https://github.com/caronc/apprise/wiki/Notify_workflows)
- [Microsoft Teams](https://github.com/caronc/apprise/wiki/Notify_msteams)
- [Misskey](https://github.com/caronc/apprise/wiki/Notify_misskey)
- [MQTT](https://github.com/caronc/apprise/wiki/Notify_mqtt)
- [Nextcloud](https://github.com/caronc/apprise/wiki/Notify_nextcloud)
- [NextcloudTalk](https://github.com/caronc/apprise/wiki/Notify_nextcloudtalk)
- [Notica](https://github.com/caronc/apprise/wiki/Notify_notica)
- [Notifiarr](https://github.com/caronc/apprise/wiki/Notify_notifiarr)
- [Notifico](https://github.com/caronc/apprise/wiki/Notify_notifico)
- [ntfy](https://github.com/caronc/apprise/wiki/Notify_ntfy)
- [Office 365](https://github.com/caronc/apprise/wiki/Notify_office365)
- [OneSignal](https://github.com/caronc/apprise/wiki/Notify_onesignal)
- [Opsgenie](https://github.com/caronc/apprise/wiki/Notify_opsgenie)
- [PagerDuty](https://github.com/caronc/apprise/wiki/Notify_pagerduty)
- [PagerTree](https://github.com/caronc/apprise/wiki/Notify_pagertree)
- [ParsePlatform](https://github.com/caronc/apprise/wiki/Notify_parseplatform)
- [PopcornNotify](https://github.com/caronc/apprise/wiki/Notify_popcornnotify)
- [Prowl](https://github.com/caronc/apprise/wiki/Notify_prowl)
- [PushBullet](https://github.com/caronc/apprise/wiki/Notify_pushbullet)
- [Pushjet](https://github.com/caronc/apprise/wiki/Notify_pushjet)
- [Push (Techulus)](https://github.com/caronc/apprise/wiki/Notify_techulus)
- [Pushed](https://github.com/caronc/apprise/wiki/Notify_pushed)
- [PushMe](https://github.com/caronc/apprise/wiki/Notify_pushme)
- [Pushover](https://github.com/caronc/apprise/wiki/Notify_pushover)
- [PushSafer](https://github.com/caronc/apprise/wiki/Notify_pushsafer)
- [Pushy](https://github.com/caronc/apprise/wiki/Notify_pushy)
- [PushDeer](https://github.com/caronc/apprise/wiki/Notify_pushdeer)
- [Reddit](https://github.com/caronc/apprise/wiki/Notify_reddit)
- [Resend](https://github.com/caronc/apprise/wiki/Notify_resend)
- [Revolt](https://github.com/caronc/apprise/wiki/Notify_Revolt)
- [Rocket.Chat](https://github.com/caronc/apprise/wiki/Notify_rocketchat)
- [RSyslog](https://github.com/caronc/apprise/wiki/Notify_rsyslog)
- [Ryver](https://github.com/caronc/apprise/wiki/Notify_ryver)
- [SendGrid](https://github.com/caronc/apprise/wiki/Notify_sendgrid)
- [ServerChan](https://github.com/caronc/apprise/wiki/Notify_serverchan)
- [Signal API](https://github.com/caronc/apprise/wiki/Notify_signal)
- [SimplePush](https://github.com/caronc/apprise/wiki/Notify_simplepush)
- [Slack](https://github.com/caronc/apprise/wiki/Notify_slack)
- [SMTP2Go](https://github.com/caronc/apprise/wiki/Notify_smtp2go)
- [SparkPost](https://github.com/caronc/apprise/wiki/Notify_sparkpost)
- [Splunk](https://github.com/caronc/apprise/wiki/Notify_splunk)
- [Streamlabs](https://github.com/caronc/apprise/wiki/Notify_streamlabs)
- [Synology Chat](https://github.com/caronc/apprise/wiki/Notify_synology_chat)
- [Syslog](https://github.com/caronc/apprise/wiki/Notify_syslog)
- [Telegram](https://github.com/caronc/apprise/wiki/Notify_telegram)
- [Twitter](https://github.com/caronc/apprise/wiki/Notify_twitter)
- [Twist](https://github.com/caronc/apprise/wiki/Notify_twist)
- [Webex Teams (Cisco)](https://github.com/caronc/apprise/wiki/Notify_wxteams)
- [WeCom Bot](https://github.com/caronc/apprise/wiki/Notify_wecombot)
- [WhatsApp](https://github.com/caronc/apprise/wiki/Notify_whatsapp)
- [WxPusher](https://github.com/caronc/apprise/wiki/Notify_wxpusher)
- [XBMC](https://github.com/caronc/apprise/wiki/Notify_xbmc)
- [Zulip Chat](https://github.com/caronc/apprise/wiki/Notify_zulip)
- [Africas Talking](https://github.com/caronc/apprise/wiki/Notify_africas_talking)
- [Automated Packet Reporting System (ARPS)](https://github.com/caronc/apprise/wiki/Notify_aprs)
- [AWS SNS](https://github.com/caronc/apprise/wiki/Notify_sns)
- [BulkSMS](https://github.com/caronc/apprise/wiki/Notify_bulksms)
- [BulkVS](https://github.com/caronc/apprise/wiki/Notify_bulkvs)
- [Burst SMS](https://github.com/caronc/apprise/wiki/Notify_burst_sms)
- [ClickSend](https://github.com/caronc/apprise/wiki/Notify_clicksend)
- [DAPNET](https://github.com/caronc/apprise/wiki/Notify_dapnet)
- [D7 Networks](https://github.com/caronc/apprise/wiki/Notify_d7networks)
- [DingTalk](https://github.com/caronc/apprise/wiki/Notify_dingtalk)
- [Free-Mobile](https://github.com/caronc/apprise/wiki/Notify_freemobile)
- [httpSMS](https://github.com/caronc/apprise/wiki/Notify_httpsms)
- [Kavenegar](https://github.com/caronc/apprise/wiki/Notify_kavenegar)
- [MessageBird](https://github.com/caronc/apprise/wiki/Notify_messagebird)
- [MSG91](https://github.com/caronc/apprise/wiki/Notify_msg91)
- [Plivo](https://github.com/caronc/apprise/wiki/Notify_plivo)
- [Seven](https://github.com/caronc/apprise/wiki/Notify_seven)
- [Société Française du Radiotéléphone (SFR)](https://github.com/caronc/apprise/wiki/Notify_sfr)
- [Signal API](https://github.com/caronc/apprise/wiki/Notify_signal)
- [Sinch](https://github.com/caronc/apprise/wiki/Notify_sinch)
- [SMSEagle](https://github.com/caronc/apprise/wiki/Notify_smseagle)
- [SMS Manager](https://github.com/caronc/apprise/wiki/Notify_sms_manager)
- [Threema Gateway](https://github.com/caronc/apprise/wiki/Notify_threema)
- [Twilio](https://github.com/caronc/apprise/wiki/Notify_twilio)
- [Voipms](https://github.com/caronc/apprise/wiki/Notify_voipms)
- [Vonage](https://github.com/caronc/apprise/wiki/Notify_nexmo) (formerly Nexmo)
- [E-Mail](https://github.com/caronc/apprise/wiki/Notify_email)
- ... and many more

The configuration for the XMPP target might look like this:

```toml
[[Targets]]
ID = "your_target"
Type = "apprise"

  [Targets.Args]
  apprise = "/home/you/.local/share/pyenv/bin/apprise"
  connection = "matrixs://your_bot:hunter2@matrix.org:443/!xXxXxXxX:matrix.org"
```

To use this target, specify its ID inside an `Application` configuration:

```toml
...
Target = "your_target"
...
```

## Run

All that's needed is a [configuration](#configure) and Overpush can be launched:

```sh
$ overpush
```

### Supervisor

To run Overpush via `supervisord`, create a config like this inside
`/etc/supervisord.conf` or `/etc/supervisor/conf.d/overpush.ini`. See
[this example](/examples/etc/supervisor.d/overpush.ini).

**Note:** It is advisable to run Overpush under its own, dedicated daemon user.
Make sure to either adjust `directory` as well as `user`.

### OpenBSD rc

As before, create a configuration file under `/etc/overpush.toml`.

Then copy the [example rc.d script](examples/etc/rc.d/overpush) to
`/etc/rc.d/overpush` and copy the binary to e.g. `/usr/local/bin/overpush`. Last
but not least, update the `/etc/rc.conf.local` file to contain the following
line:

```conf
overpush_user="_overpush"
```

It is advisable to run Overpush as a dedicated user, hence create the
`_overpush` daemon account or adjust the line above according to your setup.

You can now run Overpush by enabling and starting the service:

```sh
$ rcctl enable overpush
$ rcctl start overpush
```

### systemd

As before, create a configuration file under `/etc/overpush.toml`.

Then copy the
[example systemd service](examples/etc/systemd/system/overpush.service) to
`/etc/systemd/system/overpush.service` and copy the binary to e.g.
`/usr/local/bin/overpush`. Ensure the `overpush` user and group exist, or change
them to something appropriate.

Then, as root, reload systemd and enable the service:

```sh
# systemctl daemon-reload
# systemctl enable --now overpush
```

If you'd rather prefer running Overpush as a user-level service, put the service
file in `~/.config/systemd/user/overpush.service` and reload and enable the
service as user:

```sh
$ systemctl --user daemon-reload
$ systemctl --user enable --now overpush
```

### Podman

```sh
$ podman run \
  -d \
  --name overpush \
  --network podman \
  -v "$(pwd)/overpush.toml:/etc/overpush.toml:ro" \
  -p 8080:8080 \
  ghcr.io/mrusme/overpush:latest
```

**Info:** `apprise` inside the Docker container is available at
`/usr/bin/apprise`; Make sure to set the `apprise = "/usr/bin/apprise"` flag
inside `apprise` targets.

### Docker

```sh
$ docker run \
  -d \
  --name overpush \
  -v "$(pwd)/overpush.toml:/etc/overpush.toml:ro" \
  -p 8080:8080 \
  ghcr.io/mrusme/overpush:latest
```

**Info:** `apprise` inside the Docker container is available at
`/usr/bin/apprise`; Make sure to set the `apprise = "/usr/bin/apprise"` flag
inside `apprise` targets.

### Kubernetes

TODO

### Amazon Web Services Lambda Function

TODO

### Google Cloud Function

TODO

## FAQ

### Why?

[That's why](https://xn--gckvb8fzb.com/goodbye-pushover-hello-overpush/).

### XMPP? Without OTR? Or OMEMO?

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

#### Workaround

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

### But Matrix is E2EE, right?

[Nope](https://github.com/caronc/apprise/issues/305). Use the aforementioned
[workaround](#workaround).
