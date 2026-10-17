# Prompt evaluation: Kev vs Bedrock

Scores 100 labeled financial-agent prompts for sensitive PII. Local [Kev](https://github.com/jaredpalmer/kev), local [Laya](https://github.com/NandhaKishorM/laya), local [Strands Decider](https://github.com/strands-labs/strands-decider), local [GLiNER2.5-Decide](https://huggingface.co/fastino/GLiNER2.5-Decide), local [Decider-4B](https://huggingface.co/Mapika/decider-4b), and Claude Haiku 4.5 on Amazon Bedrock see the same rule. The labels are known because the prompts were written for this set.

Kev is the local stand-in for Jev. Jev itself is a hosted API.

## Run Kev

Requires Python 3.13 and [uv](https://docs.astral.sh/uv/). The script serves Kev-4B. Set `KEV_RUN=jaredpalmer/kev-0.8b` for the smaller, faster checkpoint. The first start downloads that checkpoint.

```bash
./scripts/serve-kev.sh
```

The server listens on `http://127.0.0.1:8009`.

## Run Laya

The script serves the multilingual checkpoint on `http://127.0.0.1:8010`, using Metal (`LAYA_DEVICE=mps`). The first start downloads that checkpoint.

```bash
./scripts/serve-laya.sh
```

```bash
go run ./cmd/eval -backends laya
go run ./cmd/eval -task choice -backends laya -out results/laya-choice.json
```

## Run Strands Decider

The script serves Strands Decider 2B on `http://127.0.0.1:8011`, using Metal (`STRANDS_DEVICE=mps`). The first start downloads that checkpoint.

```bash
./scripts/serve-strands.sh
```

```bash
go run ./cmd/eval -backends strands -out results/strands-noul.json
go run ./cmd/eval -task choice -backends strands -out results/strands-choice.json
```

## Run GLiNER2.5-Decide

The script serves [GLiNER2.5-Decide](https://huggingface.co/fastino/GLiNER2.5-Decide) on `http://127.0.0.1:8012`, using Metal (`GLINER_DEVICE=mps`). `scripts/gliner_server.py` puts the model behind the same System One API as Kev. The first start downloads the 2 GB checkpoint.

The checkpoint is English-only. About half the prompts are Swedish. Set `GLINER_RUN=fastino/GLiNER2.5-multi-Decide` to serve the multilingual checkpoint instead.

After the first download, set `HF_HUB_OFFLINE=1` to load from the local cache. Without it, the server asks Hugging Face for updates on every start and waits through timeouts on networks that block it.

```bash
./scripts/serve-gliner.sh
```

```bash
go run ./cmd/eval -backends gliner -out results/gliner-noul.json
go run ./cmd/eval -task choice -backends gliner -out results/gliner-choice.json
```

## Run Decider-4B

The script serves [Mapika's Decider-4B](https://huggingface.co/Mapika/decider-4b) on `http://127.0.0.1:8013`, using Metal (`DECIDER_DEVICE=mps`). Set `DECIDER_RUN=Mapika/decider-2b` for the smaller checkpoint. The first start downloads the 8.4 GB checkpoint.

```bash
./scripts/serve-decider.sh
```

```bash
go run ./cmd/eval -backends decider -out results/decider-noul.json
go run ./cmd/eval -task choice -backends decider -out results/decider-choice.json
```

## Score the prompts

Uses the AWS profile `ai-dev` in `eu-north-1` and the EU inference profile `eu.anthropic.claude-haiku-4-5-20251001-v1:0`.

```bash
go run ./cmd/eval
```

The report is printed and written to `results/latest.json`. Kev treats a noul probability of 0.5 or higher as sensitive PII. Haiku must answer `true` or `false`.

Flags: `-backends kev,laya,strands,gliner,decider,bedrock`, `-task noul`, `-task choice`, `-task gate` or `-task combined`, `-limit N`, `-mask off|on|full|both`, `-rubric original|value`, `-rule off|on`, `-extra`, `-kev-url`, `-laya-url`, `-strands-url`, `-gliner-url`, `-decider-url`, `-data`, `-out`.

`-task choice` asks which kind of PII is present: `none`, `government_id`, `payment_card`, `bank_account`, `secret`, or `named_contact`.

`-task combined` scores the same PII kinds, but asks the yes/no question first. When it says not sensitive, the answer is `none` and the choice question is not asked; otherwise the answer is the choice question's. The choice question alone files a mention such as "My governmentid is xxxx" under `government_id`; with `-rubric value` the yes/no question does not, so this is the task the value rule helps. It is `kev_sensitive_category` in [duckdb/README.md](duckdb/README.md), with two requests instead of one.

`-task gate` scores the yes/no task with that choice question instead: any answer other than `none` counts as sensitive, and the probability is 1 minus the probability of `none`. It is still one question and one request per prompt. The report is in the same format as `-task noul`, with `-gate` added to each backend name.

`-mask on` replaces already-masked values such as `900101-****` or `**** **** **** 4242` with `masked` before the prompt reaches the model, so a model does not mistake them for real PII. Full values are left alone. `-mask both` runs every backend with and without masking and prints a "model alone -> model + masking" comparison.

To compare masking for every local model, one server at a time:

```sh
./scripts/compare-mask.sh                 # all models
./scripts/compare-mask.sh kev-4b decider-4b
```

Reports go to `results/mask/<model>-<task>.json` and server logs to `results/mask/logs/`.

## Score the improvements

Three flags bring in what [duckdb/README.md](duckdb/README.md) adds on top of the rubric. All are off by default, so the reports above stay comparable:

- `-rubric value` adds a rule that naming a kind of data without its value is not sensitive, such as "My governmentid is xxxx" or "I won't tell you my password". It changes only the yes/no question, so it affects `-task noul` and `-task combined`, not `gate` or `choice`.
- `-mask full` is `-mask on` plus placeholders (`xxxx`, `XXXXXX-XXXX`, `[REDACTED]`, `<personnummer>`, `**** **** **** ****`) replaced with `masked`, and a label on every valid personnummer, samordningsnummer, or card number, such as `[personnummer: 197306253977]`. A number is valid when it has a real date (day plus 60 for samordningsnummer) and a passing Luhn check digit.
- `-rule on` counts a text with a valid number as sensitive, with probability 1, and answers `government_id` or `payment_card` in the choice and combined tasks, whatever the model says.

`-extra` scores more labeled sets after the main 100, and the report adds the accuracy per set. The three extra sets come from `data/build_prompts.py`: `secret_mentions` (a password, PIN, or code with or without its value), `placeholders` (a kind of data named with a placeholder or a refusal), and `bare_numbers` (personnummer and card numbers without a label, and look-alikes).

```bash
go run ./cmd/eval -backends kev -rubric value -mask full -rule on \
  -extra data/secret_mentions.json,data/placeholders.json,data/bare_numbers.json
```

To score every local model before and after, one server at a time:

```sh
./scripts/compare-improved.sh                 # all models, tasks noul, gate, choice and combined
./scripts/compare-improved.sh kev-4b decider-4b
TASKS="noul" ./scripts/compare-improved.sh
```

Before is `-mask on`; after is `-rubric value -mask full -rule on`. Both score all four sets. Reports go to `results/improved/<model>-<task>-before.json` and `-after.json`, and logs to `results/improved/logs/`.

## Explore in DuckDB

To score prompts with Kev from SQL in the DuckDB UI, follow [duckdb/README.md](duckdb/README.md).
