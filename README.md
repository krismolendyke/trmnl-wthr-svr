# TRMNL WTHR SVR

An [Ambient Weather Network](https://ambientweather.net/) webhook server for [trmnl](https://usetrmnl.com/) devices.

Run locally with the 1Password CLI, `op`:

```sh
go run . --debug serve \
    --application-key $(op read "op://Private/AmbientWeather/Tidbyt Secrets/Application Key") \
    --api-key $(op read "op://Private/AmbientWeather/Tidbyt Secrets/API Key")
```
