# picolytics: lightweight, high-performance web analytics
Picolytics web analytics: self-hosted, privacy-first, with support for bare metal, docker, and Kubernetes environments. Powered by Postgres, Go, and Grafana.

## Features:
* **:feather: Lightweight Tracking Script:** Super-small Javascript tracking script weighs in at under 1000 bytes.
* **:chart_with_upwards_trend: Bring your own dashboards:** Everything is in Postgres - build custom dashboards in Grafana/Superset/Tableau/etc. Works great with Supabase. Sample Grafana dashboard provided out of the box.
* **:see_no_evil: Privacy friendly:** ***GDPR-Easy***. No cookies! Track sessions and locations without storing the user's IP address.
* **:muscle: Performant and Scalable:** Low-overhead, horizontally-scalable server. Sensible defaults with plenty of options to tune.
* **:ship: Easy to Deploy:** Single binary with docker-compose and Kubernetes Helm options. Only dependency is Postgres.
* **:unlock: Control your Own Data:** Open Source (Apache) and self-hosted.

# Quickstart
1. Clone this repo: `git clone https://github.com/nmcclain/picolytics.git`
2. Use an insecure config for testing: `cd docker && cp .env-sample .env`
2. Start Picolytics and Postgres: `docker compose up`
3. Load the [etc/index.html](etc/index.html) file in your browser to create a sample visit.
4. Visit the sample Grafana dashboard at: http://localhost:3000/d/picolytics-47a42be-8950-55eebc62a0e1/picolytics

# Deployment

## Kubernetes
There's a Helm chart in the `k8s` directory. Configuration options are in `k8s/helm/values.yaml`.

1. `cd k8s`
2. `helm dependencies update`
3. Edit values.example.yaml
  1. Or start with `values.external.yaml` if using an external database.
4. Create a namespace: `kubectl create ns picolytics`
5. Create secrets with the following commands - be sure to change `SECURE_PASSWORD`.
```
	kubectl create secret -n picolytics generic picolytics-grafana \
		--from-literal=user=admin \
		--from-literal=password='SECURE_PASSWORD'
	kubectl create secret -n picolytics generic picolytics-db \
		--from-literal=user=picolytics \
		--from-literal=password='SECURE_PASSWORD'
	kubectl create secret -n picolytics generic picolytics-db-grafana \
		--from-literal=PICOLYTICS_DB_USER=picolyticsgrafana \
		--from-literal=PICOLYTICS_DB_PASSWORD='SECURE_PASSWORD'
```
6. `helm upgrade --install -f values.example.yaml -n picolytics picolytics ./helm`

## Docker Compose
1. Switch to the docker directory: `cd docker`
2. Copy `.env-sample` to `.env` and create secure passwords for: `POSTGRES_PASSWORD`, `POSTGRES_PICOLYTICS_PASSWORD`, and `POSTGRES_GRAFANA_PASSWORD`. One way to do it is with this command:
```
cat .env-sample | \
  sed "s|REPLACE_PG_PASSWORD|$(openssl rand -base64 18)|g" | \
  sed "s|REPLACE_PL_PASSWORD|$(openssl rand -base64 18)|g" | \
  sed "s|REPLACE_GF_PASSWORD|$(openssl rand -base64 18)|g" > .env
```
3. Start Picolytics and supporting containers: `docker compose up`
  1. Supporting containers are: postgres, victoriametrics, grafana. Feel free to disable if you're using external services for any of these.

> [!NOTE]
> Package link: https://github.com/nmcclain/picolytics/pkgs/container/picolytics

## Linux
1. Download and uncompress the latest release: https://github.com/nmcclain/picolytics/releases
2. Download DBIP database: `curl -L https://download.db-ip.com/free/dbip-city-lite-2023-12.mmdb.gz | gunzip -c > geoip.mmdb`
  1. or fetch a commercial one and save it at `geoip.mmdb`
3. Seutp a Postgres DB and configure the connection: `export set PGCONNSTRING=postgres://user:password@host:5432/picolytics`.
4. Launch Picolytics: `./picolytics`

## Postgres Setup
You must configure `PGHOST`, `PGDATABASE`, `PGUSER`, and `PGPASSWORD` OR `PGCONNSTRING`.

Picolytics will create tables by running migrations at startup.

Optionally, see the [Docker Postgres init script](docker/postgres-initdb.d/initdb-grafana-user.sh) for an example of using a custom database and Postgres role.

Works great with Supabase!

## Grafana Setup
The Helm chart and docker-compose deployment methods include a Grafana instance with Datasources and a sample Dashboard. If you're using an existing Grafana instance, you'll need to:
1. Configure a Grafana `postgres` Datasource to connect to your Picolytics database.
2. Optionally configure Prometheus or VictoriaMetrics to scrape Picolytics metrics.
  1. Then create a Grafana Datasource to connect to your metrics server.
3. Import the [sample dashboard](k8s/helm/dashboards/picolytics.json).

# Configuration
Picolytics uses the [spf13/viper](https://github.com/spf13/viper) library and supports configuration via environment variables and/or a config file. Environment variables > config file > defaults. 

## Required configuration
| Environment Variable | Config File Key   | Description                     | Default |
| -------------------- | ----------------- | ------------------------------- | ------- |
| `PGCONNSTRING`       | `pgConnString`    | PostgreSQL connection string    | ""      |

Example: `export PGCONNSTRING="postgres://USER:PASSWORD@HOST:PORT/DATABASE?sslmode=SSLMODE"`

### OR (useful with k8s Secrets)
| Environment Variable | Config File Key   | Description                     | Default |
| -------------------- | ----------------- | ------------------------------- | ------- |
| `PGHOST`             | `pgHost`          | PostgreSQL server host and port | ""      |
| `PGDATABASE`         | `pgDatabase`      | PostgreSQL database name        | ""      |
| `PGUSER`             | `pgUser`          | PostgreSQL user                 | ""      |
| `PGPASSWORD`         | `pgPassword`      | PostgreSQL password             | ""      |

## Optional configuration

### HTTP server
| Environment Variable   | Config File Key       | Default Value  | Description                                 |
| ---------------------- | --------------------- | -------------- | ------------------------------------------- |
| `LISTEN_ADDR`          | `listenAddr`          | :8080          | Server listen address                       |
| `LOG_FORMAT`           | `logFormat`           | json           | Pick "json" or "text"                       |
| `ROOT_REDIRECT`        | `rootRedirect`        | ""             | URL to redirect root (/) requests           |
| `STATIC_DIR`           | `staticDir`           | static         | Directory with static files if found. Will be used instead of embedded files. This is an easy way to customize the tracking JS. |
| `STATIC_CACHE_MAX_AGE` | `staticCacheMaxAge`   | 3600 [1 hour]  | Static files max age (seconds) to send to browser.  |
| `REQUEST_RATE_LIMIT`   | `requestRateLimit`    | 10             | Request rate limit (events per second per IP per instance). Note: the rate limiter uses local, not shared state. See: [middleware docs](https://echo.labstack.com/docs/middleware/rate-limiter)|

### Automatic TLS (LetsEncrypt)
Auto TLS is useful for deploying a single instance that is directly exposed to the internet; not useful for k8s or other deployments behind a reverse proxy or load balancer. For this to work, your server must be accessible from the internet on ports 80 and 443 at the `AUTOTLS_HOST` DNS name.
| Environment Variable   | Config File Key       | Default Value  | Description                                 |
| ---------------------- | --------------------- | -------------- | ------------------------------------------- |
| `AUTOTLS_ENABLED`          | `autotlsEnabled`          | false          | Enable Auto TLS. Host must be internet-accessible on ports 80 and 443.                       |
| `AUTOTLS_HOST`          | `autotlsHost`          | ""          | Required if Auto TLS enabled. Must be a "real" DNS name that routes to the host.                       |
| `AUTOTLS_STAGING`          | `autotlsStaging`          | true           | Use LetsEncrypt staging. Recommended to start with staging to avoid rate limits while testing.                          |

### Event handler
| Environment Variable   | Config File Key       | Default Value  | Description                                 |
| ---------------------- | --------------------- | -------------- | ------------------------------------------- |
| `BODY_MAX_SIZE`        | `bodyMaxSize`         | 2048 [2KB]   | Max request body size in bytes              |
| `VALID_EVENT_NAMES`    | `validEventNames`     | default if empty: "load,visible,hidden,hashchange,ping" | CSV list of valid event types |

### Database settings
| Environment Variable   | Config File Key       | Default Value  | Description                                 |
| ---------------------- | --------------------- | -------------- | ------------------------------------------- |
| `PGPORT`             | `pgPort`          | PostgreSQL port                 | 5432      |
| `PGSSLMODE`            | `pgSslMode`           | prefer         | PostgreSQL SSL mode\n Pick one of: "disable", "allow", "prefer", "require", "verify-ca", "verify-full".  |
| `PGCONNATTEMPTS`             | `pgConnAttempts`          | Number of DB connection attempts before failing. Useful if started in docker-compose.                 | 5      |
| `SKIP_MIGRATIONS`             | `skipMigrations`          | Skip application of DB migrations.                 | false      |

### Proxy settings
| Environment Variable   | Config File Key       | Default Value  | Description                                 |
| ---------------------- | --------------------- | -------------- | ------------------------------------------- |
| `IP_EXTRACTOR`         | `ipExtractor`         | direct         | Pick "direct", "xff", or "realip". See: [Echo docs](https://echo.labstack.com/docs/ip-address)           |
| `TRUSTED_PROXIES`      | `trustedProxies`   | Internal IP addresses (loopback, link-local unicast, private-use and unique local address from RFC6890, RFC4291 and RFC4193): IP ranges starting with `127.`, `169.254.`, `10.`, or `192.168.` | List of CIDRs to trust if `IP_EXTRACTOR` is set to `xff` or `realip`. See: [Echo trust option docs](https://pkg.go.dev/github.com/labstack/echo#TrustOption) |

### Privacy settings
| Environment Variable   | Config File Key       | Default Value    | Description                                      |
| ---------------------- | --------------------- | ---------------- | ------------------------------------------------ |
| `GEO_IP_FILE`          | `geoIpFile`           | geoip.mmdb       | Specify an alternate location for Geo MMDB file. |
| `PRUNE_DAYS`           | `pruneDays`           | 0 [keep forever] | Number of days to retain events and sessions in DB. |
| `PRUNE_CHECK_HOURS`    | `pruneCheckHours`     | 24               | Frequency in hours to check for pruneable data. |
| `SESSION_TIMEOUT_MIN`  | `sessionTimeoutMin`   | 30               | Idle minutes before a visit is considered a new session. |

### Admin/health/metrics server
The admin server runs on a different port, to help avoid exposing it to the internet. It provides `/healthz`, `/ready`, and Prometheus-compatible `/metrics` endpoints. It also provides pprof endpoints (`/debug/pprof/goroutine`, `/debug/pprof/heap`, etc.) if `DEBUG` is set to `true`.

`DISABLE_HOST_METRICS` is `true` by default. Setting it to `false` can be useful if you're deploying to an environment that doesn't have `node_exporter` or the like.

| Environment Variable   | Config File Key       | Default Value  | Description                                      |
| ---------------------- | --------------------- | -------------- | ------------------------------------------------ |
| `ADMIN_LISTEN`         | `adminListen`         | ""             | Disabled unless specified.    |
| `DISABLE_HOST_METRICS` | `disableHostMetrics`  | true           | Enable host CPU/memory metrics.    |
| `DEBUG`                | `debug`               | false          | Enable debug logging and pprof endpoints.    |

### Performance tuning
The default settings below perform well for up to 1000 requests/second.  You can decrease queue size to use less memory, or increase batch and queue size to handle more traffic and larger spikes.
| Environment Variable   | Config File Key       | Default Value  | Description                                 |
| ---------------------- | --------------------- | -------------- | ------------------------------------------- |
| `QUEUE_SIZE`           | `queueSize`           | 640000         | Processing queue size                       |
| `BATCH_MAX_SIZE`       | `batchMaxSize`        | 6400           | Maximum batch size                          |
| `BATCH_MAX_MSEC`       | `batchMaxMsec`        | 500            | Max time (ms) per batch process             |

### Configuration file
| Environment Variable   | Config File Key       | Default Value  | Description                                 |
| ---------------------- | --------------------- | -------------- | ------------------------------------------- |
| `CONFIG_NAME`          | `configName`          | config         | Config file name                            |
| `CONFIG_PATH`          | `configPath`          | .              | Config file path                            |

You can use a config file instead of environment variables. You must either use the `-c <configfile>` flag, or set `CONFIG_NAME` and `CONFIG_PATH` environment variables. Note that picolytics does not yet support hot config reloads.

See the [sample config.yaml](config.yaml) or use the following command to write a default config.yaml file:
```
picolytics --write-default-config
```

# Javascript Tracker
Picolytics provides a lightweight tracking script at `/pico.js`.

Add this code to your page `<head>` section, replacing `example.com` with your domain:
```
<script defer src="https://example.com/pico.js"></script>
```

You can customize the Javascript by setting `STATIC_DIR` and mounting a custom directory as a ConfigMap or Docker volume.

# Privacy
Picolytics is compliant with GDPR. It follows [Plausible Analytics' approach](https://plausible.io/data-policy) privacy approach. In brief:
* **No Personal Data Collection:** No personally identifiable information (PII) is stored. All data is aggregated and contains no personal information. Visitor data cannot be related back to any individual.
* **No Device-Persistent Identifier:** No cookies, browser cache, or local storage.
* **Self-Hosted:** You fully control your data and can host in the EU as desired.

This generates a random string of letters and numbers that is used to calculate unique visitor numbers for the day. The raw data IP address and User-Agent are never stored in our logs, databases or anywhere on disk at all.

> [!CAUTION]
> With no practical way to reverse the "visitor ID" to IP, User Agent, or other identifable data, the Picolytics database and logs fall outisde the scope of most privacy standards. To minimize auditing scope, you may choose to limit data retention with the `PRUNE_DAYS` setting.
> Using Picolytics does not guarantee compliance. Among other things, you'll need to either make sure your web server/apps don't log IPs, or have GDPR-compliant disclosure, discovery, and right-to-be-forgotten procedures in place.

## Geolocation
The Docker container includes db-ip's [free IP to City Lite geolocation database](https://db-ip.com/db/download/ip-to-city-lite). You'll need to download in order to run Picolytics outside a container.

The licensing terms of this database require attribution in any user front-end/UI:
```
<a href='https://db-ip.com'>IP Geolocation by DB-IP</a>
```

You can use any MaxMind database-compatible `mmdb` file, including a licensed one for better accuracy. Set `GEO_IP_FILE` to point to your uncompressed `mmdb` file. Using the Docker container, you can mount your custom `mmdb` file at `/app/geo.mmdb`. 

# Production
* A single Picolytics instance can support up to 1000 req/sec on a DigitalOcean `s-2vcpu-4gb-amd` droplet. This includes running a local Postgres database - you should be able to support much more traffic with an external DB.
* There is a configurable rate limiter controlling requests/second per IP. You can set this to `0` to disable rate limiting (for example, for load testing). Note: the rate limiter uses local, not shared state. See: https://echo.labstack.com/docs/middleware/rate-limiter
| `REQUEST_RATE_LIMIT`   | `requestRateLimit`    | 10             | Request rate limit (events per second per IP per instance). |
* TimescaleDB is not supported at this time, mostly due to the current migrations setup.

# What picolytics is not
* **Single Page App?** You should be using a full-stack tracing library/service, like [honeycomb.io](https://www.honeycomb.io/), [Sentry](https://sentry.io/), or [Tempo](https://grafana.com/docs/tempo/latest/).
* **Tons of Traffic?** You'll want a ClickHouse backend, not Postgres. Try [Plausible Analytics](https://plausible.io/docs/self-hosting) or [PostHog](https://github.com/posthog/posthog).
* **Session Recording?** Consider [Highlight](https://github.com/highlight/highlight) or [PostHog](https://github.com/posthog/posthog).
* **Similar OSS Alternatives?** Check out [Plausible Analytics](https://plausible.io/docs/self-hosting), [Shynet](https://github.com/milesmcc/shynet), or [Matomo](https://github.com/matomo-org/matomo).

# Development

You'll need to [install the sqlc CLI](https://docs.sqlc.dev/en/stable/overview/install.html) so you can generate the db.go files from the schema and query.sql.

The Makefile has some useful commands for developers:
* Download DBIP database: `make dbip`
* Build binaries: `make build`
* Build docker image: `make docker`
* Minify tracker: `make tracker` (requires: npm install uglify-js -g)
* Code coverage: `make cover`
* Local load test: `make load` (requires Vegeta: https://github.com/tsenart/vegeta/)

> [!TIP]
> I'm very open to contributors under the Apache License. We can formalize things with your first commit.
