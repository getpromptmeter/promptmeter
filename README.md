# Promptmeter

Cost intelligence platform for AI/LLM workloads. Track, attribute, and optimize LLM spending across models, teams, and features.

Self-hosted. No limits. No vendor lock-in.

> **Status: Active Development.** Ingestion pipeline and dashboard are working end-to-end. Alerting and cost explorer are next. Not yet ready for production use.

## Architecture

```
SDK (Python)           Ingestion API (Go)         NATS JetStream
  pm.track() -------> POST /v1/events ---------> durable queue
  pm.wrap(client)      validates, rate-limits      protobuf msgs
                                                       |
                                                       v
  Dashboard UI <----- Dashboard API (Go) <-----  Cost Worker (Go)
  (Next.js)            ClickHouse queries         batch writes, cost calc
                       PostgreSQL CRUD                 |
                                                       v
                                              ClickHouse  +  S3
                                              (analytics)   (prompt text)
```

Supporting services: PostgreSQL (state), Redis (cache, rate limiting), Caddy (reverse proxy, TLS).

Requires Docker and Docker Compose.

## Quick Start

```bash
git clone https://github.com/getpromptmeter/promptmeter.git
cd promptmeter
docker compose -f deploy/docker-compose.dev.yml --profile full up
```

An API key is printed in the dashboard-api logs on first startup. You can also create keys in the UI at Settings > API Keys.

The dashboard will be available at `http://localhost:3000`, the ingestion API at `http://localhost:8443`.

## SDK Usage

```python
from promptmeter import PromptMeter
from openai import OpenAI

pm = PromptMeter(api_key="pm_live_xxx", endpoint="http://localhost:8443")
client = pm.wrap(OpenAI())  # all calls are now tracked
```

Or track manually:

```python
pm.track(model="gpt-4o", provider="openai", prompt_tokens=100, completion_tokens=50)
```

Install with `pip install -e ./sdk/python`.

## What's Implemented

- **Ingestion API** -- validates events, publishes to NATS, rate limiting
- **Cost Worker** -- consumes from NATS, calculates costs from model price table, batch writes to ClickHouse, uploads prompt/response text to S3
- **Python SDK** -- `pm.track()`, OpenAI/Anthropic provider wrapping, client-side batching, retry with backoff
- **Dashboard API** -- cost overview, cost breakdown by model/feature, cost timeseries, API key CRUD, org settings, project selector
- **Dashboard UI** -- Next.js 16 with Overview page (KPI cards, cost charts, cost tables), Settings pages (General, API Keys), Login, Welcome screen
- **Auth** -- JWT + refresh tokens, OAuth (Google/GitHub), autologin for self-hosted
- **Storage layer** -- ClickHouse (analytics + materialized views), PostgreSQL (state), Redis (cache/rate limits), S3 (prompt text)
- **Dev environment** -- single `docker compose up` brings up everything

## Roadmap

- Cost explorer -- group-by toggle, drill-down into model/feature/project
- Events page -- event list, event detail, lazy-load prompt/response from S3
- Alert engine -- budget thresholds, cost spike detection, error rate alerts, Slack/email delivery
- OpenAI Usage API poller -- zero-code cost import, no SDK integration needed
- Projects CRUD -- create/edit/delete projects, per-project API keys and analytics

## License

Server: [FSL-1.1-MIT](LICENSE) | SDK: [MIT](sdk/python/LICENSE)
