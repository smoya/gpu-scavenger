# gpu-scavenger

[![MIT License](https://img.shields.io/apm/l/atomic-design-ui.svg?)](https://github.com/tterb/atomic-design-ui/blob/master/LICENSEs) [![Go Report Card](https://goreportcard.com/badge/github.com/smoya/gpu-scavenger)](https://goreportcard.com/report/github.com/smoya/gpu-scavenger)

This Bot listens to some GPU retailer sites and notifies via Telegram in case there is stock available.

## Default Retailers
- [Ldlc.com](https://www.ldlc.com)
- [Coolmod.com](https://www.coolmod.com)
- [VsGamers.es](https://www.vsgamers.es)
- [Neobyte.es](https://www.neobyte.es)

## Run
### Docker
```bash
docker run -e GPUSCAVENGER_TELEGRAM_BOT_TOKEN=<telegram-bot-token> -e GPUSCAVENGER_TELEGRAM_NOTIFICATION_CHAT=<telegram-notification-chat> smoya/gpu-scavenger:latest
```

## Config

To run this project, you will need to specify *some* of the following environment variables:

| Name                                    | Description                                                                                     | Required | Default |
|-----------------------------------------|-------------------------------------------------------------------------------------------------|:--------:|:-------:|
| GPUSCAVENGER_TELEGRAM_BOT_TOKEN         | Telegram Bot Token. Create one with https://t.me/botfather.                                     |     ✓    |         |
| GPUSCAVENGER_TELEGRAM_NOTIFICATION_CHAT | Telegram Chat ID. Easy: Invite https://t.me/GetIDsBot to the chat between you and your own bot. |     ✓    |         |
| GPUSCAVENGER_TIMEOUT                    | HTTP Client requests timeout (applies for each website)                                         |          |    4s   |
| GPUSCAVENGER_MIN_TIME                   | Min amount of time to wait between each cycle of requests. Used for avoid bans.                 |          |   10s   |
| GPUSCAVENGER_MAX_TIME                   | Max amount of time to wait between each cycle of requests. Used for avoid bans.                 |          |   20s   |
| GPUSCAVENGER_RENOTIFY_AFTER             | Articles won't be sent again if they are in stock for this period.                              |          |   10m   |
| GPUSCAVENGER_DEBUG                      | Enables debug log level which increases verbosity.                                              |          |  false  |

## License

[MIT](https://choosealicense.com/licenses/mit/)

  