# Promptmeter

Cost intelligence platform for AI/LLM workloads. Track, attribute, and optimize LLM spending across models, teams, and features.

Self-hosted. No limits. No vendor lock-in.

> **Status: Active Development.** Core ingestion pipeline is working. Dashboard and alerting are in progress. Not yet ready for production use.

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

## Quick Start

```bash
git clone https://github.com/getpromptmeter/promptmeter.git
cd promptmeter
docker compose -f deploy/docker-compose.dev.yml up
```

The ingestion API will be available at `https://localhost`.

## SDK Usage

```python
from promptmeter import PromptMeter
from openai import OpenAI

pm = PromptMeter(api_key="pm_live_xxx")
client = pm.wrap(OpenAI())  # all calls are now tracked
```

Or track manually:

```python
pm.track(model="gpt-4o", provider="openai", prompt_tokens=100, completion_tokens=50)
```

Install with `pip install promptmeter`.

## What's Implemented

- **Ingestion API** -- validates events, publishes to NATS, rate limiting
- **Cost Worker** -- consumes from NATS, calculates costs from model price table, batch writes to ClickHouse, uploads prompt/response text to S3
- **Python SDK** -- `pm.track()`, OpenAI/Anthropic provider wrapping, client-side batching, retry with backoff
- **Storage layer** -- ClickHouse (analytics), PostgreSQL (state), Redis (cache/rate limits), S3 (prompt text)
- **Dev environment** -- single `docker compose up` brings up everything

## Roadmap

- Dashboard API -- cost analytics queries, alerts CRUD, OAuth authentication
- Dashboard UI -- cost overview, cost explorer, event viewer
- Alert engine -- budget thresholds, cost spike detection, error rate alerts, Slack/email delivery
- OpenAI Usage API poller -- zero-code cost import, no SDK integration needed
- Projects -- per-app API keys, budgets, isolated analytics

## License

Server: [FSL-1.1-MIT](LICENSE) | SDK: [MIT](sdk/python/LICENSE)
