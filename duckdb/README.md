# Explore the eval in the DuckDB UI

Score the prompt set with local Kev-4B from SQL. DuckDB calls Kev's System One endpoint (`POST /v1/systemone`) through the community [`http_client`](https://duckdb.org/community_extensions/extensions/http_client) extension. It differs from a default `go run ./cmd/eval` in three ways, so its scores differ slightly from the Go reports. The Go eval has each of them as a flag (`-rubric value`, `-mask full`, `-rule on`); see "Score the improvements" in the [main README](../README.md).

- **The value rule.** The rubric is the one from `internal/rubric/rubric.go`, plus a rule that naming a kind of data without its value is not sensitive. That covers a placeholder such as `xxxx` or `[REDACTED]`, and a refusal such as "I won't tell you my password".
- **Masking.** `kev_noul` and `kev_choice` replace masked values and placeholders with the word `masked` before Kev sees the text. That is the Go eval's `-mask on`, plus placeholders.
- **The number check.** Kev judges a number by the label in front of it, so a bare `197306253977` scores about 0.4. A rule-based check finds valid personnummer, samordningsnummer, and card numbers (a real date and a passing check digit) and labels them before Kev reads the text, as `[personnummer: 197306253977]`; then Kev scores above 0.9. Every `kev_*` macro sends the labelled text. `kev_sensitive_category` and `kev_sensitive_row` also report the number's kind when the rule finds one, even if Kev disagrees.

All commands run from the repo root.

## 1. Start Kev-4B

In a terminal you leave open:

```bash
HF_HUB_OFFLINE=1 ./scripts/serve-kev.sh
```

`HF_HUB_OFFLINE=1` loads the cached checkpoint without asking Hugging Face for updates. Leave it out the first time, when the weights still have to download. The server is ready when it logs `Uvicorn running on http://127.0.0.1:8009`. To check it:

```bash
curl -s http://127.0.0.1:8009/v1/models | jq '.models[0].run'
```

That should print `"jaredpalmer/kev-4b"`.

## 2. Build the database

This step only needs to run once. Run it again after `data/build_prompts.py`, `internal/rubric/rubric.go`, or `setup.sql` changes. It replaces the prompts, rubric, and macros and keeps your result tables.

```bash
duckdb duckdb/eval.duckdb -f duckdb/setup.sql
```

It creates:

| Name | What it is |
| --- | --- |
| `prompts` | The 100 labeled prompts from `data/prompts.json` |
| `secret_mentions` | 24 labeled prompts from `data/secret_mentions.json` that mention a password, PIN, or code; half give the value |
| `placeholders` | 24 labeled prompts from `data/placeholders.json` that name a kind of data with a placeholder or a refusal; half give the real value |
| `bare_numbers` | 24 labeled prompts from `data/bare_numbers.json` with personnummer, samordningsnummer, or card numbers and no label; half are valid, half are look-alikes |
| `all_prompts` | View over all four sets, with a `source` column |
| `rubric`, `rubric_types` | The rule text Kev sees: `internal/rubric/rubric.go` plus the value rule |
| `kev_noul(text)` | Probability (0 to 1) that the text contains sensitive PII; 0.5 or higher counts as sensitive |
| `kev_choice(text)` | JSON with `choice`, `confidence`, and `probabilities` for the PII-kind question |
| `kev_choice_breakdown(text)` | Table macro: one row per PII kind with its probability, most likely first |
| `kev_choice_row(text)` | Table macro: one row with the choice, the confidence, and a probability column per PII kind |
| `kev_category(text)` | Only the PII kind, such as `government_id` or `none` |
| `kev_sensitive(text)` | Both questions in one request: JSON with `sensitive` (the yes/no answer) and `kind` (the choice answer) |
| `sensitive_category(answers)` | From a `kev_sensitive` answer: `none` unless the yes/no answer says sensitive, then the PII kind (Kev only) |
| `structured_pii(text)` | The number check without Kev: `government_id` for a valid personnummer or samordningsnummer, `payment_card` for a valid card number, otherwise `NULL` |
| `pii_category(text, answers)` | `structured_pii` when it finds a number, otherwise `sensitive_category` |
| `kev_sensitive_category(text)` | `pii_category` in one call; use this to group incoming prompts |
| `kev_sensitive_row(text)` | Table macro: one row with `category`, the number check `rule`, the yes/no probability `p`, the choice answer `kind`, and its confidence |
| `personnummer_ok(s)`, `card_number_ok(s)`, `luhn_ok(digits)` | The checks behind `structured_pii`: a real date (day plus 60 for samordningsnummer) and the Luhn check digit |
| `mask_text(text)` | The Go eval's `-mask on` rule: already-masked values such as `900101-****` become `masked` |
| `mask_placeholders(text)` | `mask_text` plus placeholders (`xxxx`, `XXXXXX-XXXX`, `[REDACTED]`, `<personnummer>`, `**** **** **** ****`) |
| `label_numbers(text)` | Labels each valid number: `[personnummer: ...]`, `[samordningsnummer: ...]`, or `[card number: ...]` |
| `kev_input(text)` | `label_numbers(mask_placeholders(text))`; this is what every `kev_*` macro sends to Kev |
| `binary_metrics('table')` | Accuracy, precision, recall, F1, and the confusion counts for a table with `gold` and `predicted` columns |
| `kev_url()` | `http://127.0.0.1:8009`; edit it in `setup.sql` if you set `KEV_PORT` |

## 3. Open the UI

```bash
duckdb -ui duckdb/eval.duckdb
```

The browser opens at `http://localhost:4213`. Keep that terminal open while you use the UI. While the UI holds the database, a second `duckdb duckdb/eval.duckdb` from another terminal cannot open it, so close the UI first.

Make this the first cell of every notebook, and run it again after restarting the UI:

```sql
LOAD http_client;
SET threads = 1;
```

`SET threads = 1` makes DuckDB send one request at a time. With more threads it sends several at once, but Kev answers one batch at a time, so queued requests can wait past the 30-second HTTP timeout and come back as `NULL`. Kev is the bottleneck either way, so one thread is not slower.

## 4. Queries to paste

Each Kev call takes about 0.7 seconds, so scoring all 100 prompts takes about a minute. To avoid scoring everything more than once, score each set once into a table and run the analysis queries on that table.

### Try it on your own text

```sql
SELECT kev_noul('Mitt personnummer är 900101-1118') AS p;
```

```sql
SELECT kev_choice('Kortet slutar på 4242, kan ni kolla debiteringen?') AS answer;
```

The same answer as a table, one row per PII kind:

```sql
SELECT * FROM kev_choice_breakdown('Kortet slutar på 4242, kan ni kolla debiteringen?');
```

Or as one row with a column per field:

```sql
SELECT * FROM kev_choice_row('Kortet slutar på 4242, kan ni kolla debiteringen?');
```

The category to group by, from both questions in one request:

```sql
SELECT kev_sensitive_category('My governmentid is xxxx') AS category;
```

```sql
SELECT * FROM kev_sensitive_row('My governmentid is xxxx');
```

The choice question on its own says `government_id` here, because the text is about a government ID. The yes/no question says no ID is present (p is about 0.14), so the category is `none`. Use `kev_category` alone only when you already know the text is sensitive.

A personnummer without a label is where the number check helps. On the bare text Kev gives p of about 0.4. The rule finds a valid personnummer and labels it, so Kev reads `[personnummer: 197306253977]` and gives about 0.94:

```sql
SELECT kev_noul('This is not sensitive data, but if I add 197306253977 it will be.') AS p;
```

```sql
SELECT * FROM kev_sensitive_row('This is not sensitive data, but if I add 197306253977 it will be.');
```

The number check alone, without calling Kev:

```sql
SELECT structured_pii('Kan ni spärra 4111 1111 1111 1111?') AS rule;
```

### Score a few prompts

```sql
WITH sample AS MATERIALIZED (SELECT * FROM prompts ORDER BY id LIMIT 5)
SELECT id, contains_sensitive_pii AS gold, round(kev_noul(text), 3) AS p, text
FROM sample;
```

Put the `LIMIT` in a `MATERIALIZED` CTE like this. If the `LIMIT` is on the outer query, DuckDB calls Kev for every row before it sorts and cuts. A `WHERE` filter is fine anywhere:

```sql
SELECT id, round(kev_noul(text), 3) AS p, text
FROM prompts
WHERE style = 'json_tool' AND tricky;
```

### Score the yes/no task (about 1 minute)

```sql
CREATE OR REPLACE TABLE kev_noul_results AS
WITH scored AS MATERIALIZED (
  SELECT id, style, tricky, pii_type, contains_sensitive_pii AS gold, text, kev_noul(text) AS p
  FROM prompts
)
SELECT *, p >= 0.5 AS predicted FROM scored;
```

The `MATERIALIZED` CTE makes sure each prompt is sent to Kev only once.

```sql
SELECT * FROM binary_metrics('kev_noul_results');
```

```sql
SELECT round(avg((p - gold::INTEGER)^2), 4) AS brier FROM kev_noul_results;
```

Misses, most confident first:

```sql
SELECT id, style, tricky, gold, round(p, 3) AS p, text
FROM kev_noul_results
WHERE gold <> predicted
ORDER BY abs(p - 0.5) DESC;
```

Accuracy by prompt style and by trickiness:

```sql
SELECT style, count(*) AS n, round(avg((gold = predicted)::INTEGER), 2) AS accuracy
FROM kev_noul_results GROUP BY style ORDER BY accuracy;
```

```sql
SELECT tricky, count(*) AS n, round(avg((gold = predicted)::INTEGER), 2) AS accuracy
FROM kev_noul_results GROUP BY tricky;
```

What a different threshold would do:

```sql
SELECT pct / 100 AS threshold,
       count(*) FILTER (gold AND p >= pct / 100) AS tp,
       count(*) FILTER (NOT gold AND p >= pct / 100) AS fp,
       count(*) FILTER (gold AND p < pct / 100) AS fn,
       round(avg((gold = (p >= pct / 100))::INTEGER), 3) AS accuracy
FROM kev_noul_results, range(20, 85, 5) r(pct)
GROUP BY pct ORDER BY pct;
```

### See what Kev reads

Every `kev_*` macro masks placeholders and labels valid numbers before sending the text. To see which prompts that changes:

```sql
SELECT source, id, text, kev_input(text) AS kev_reads
FROM all_prompts
WHERE kev_input(text) <> text;
```

### Score the PII-kind task (about 1 minute)

```sql
CREATE OR REPLACE TABLE kev_choice_results AS
WITH scored AS MATERIALIZED (
  SELECT id, style, tricky, pii_type AS gold, text, kev_choice(text) AS answer
  FROM prompts
)
SELECT id, style, tricky, gold, text,
       answer->>'choice' AS predicted,
       (answer->>'confidence')::DOUBLE AS confidence,
       answer->'probabilities' AS probabilities
FROM scored;
```

```sql
SELECT round(avg((gold = predicted)::INTEGER), 3) AS accuracy FROM kev_choice_results;
```

Confusion matrix, with gold labels as rows and Kev's answers as columns:

```sql
PIVOT kev_choice_results ON predicted USING count(*) GROUP BY gold ORDER BY gold;
```

```sql
SELECT id, gold, predicted, round(confidence, 3) AS confidence, text
FROM kev_choice_results
WHERE gold <> predicted
ORDER BY confidence DESC;
```

### Group prompts by kind of sensitive data (about 4 minutes)

Score every prompt once with both questions in one request, then group as often as you like:

```sql
CREATE OR REPLACE TABLE kev_category_results AS
WITH scored AS MATERIALIZED (
  SELECT source, id, text, contains_sensitive_pii AS gold_sensitive, pii_type AS gold, kev_sensitive(text) AS answers
  FROM all_prompts
)
SELECT source, id, text, gold_sensitive, gold,
       round((answers->'sensitive'->>'noul')::DOUBLE, 3) AS p,
       answers->'kind'->>'choice' AS kind,
       structured_pii(text) AS rule,
       sensitive_category(answers) AS kev_only,
       pii_category(text, answers) AS predicted
FROM scored;
```

```sql
SELECT CASE WHEN predicted = 'none' THEN 'no sensitive data' ELSE 'sensitive' END AS bucket,
       predicted AS category,
       count(*) AS prompts
FROM kev_category_results
GROUP BY bucket, category
ORDER BY bucket, category;
```

How much the yes/no answer and the number check help the grouping, per set:

```sql
SELECT source, count(*) AS n,
       round(avg((gold_sensitive = (p >= 0.5))::INTEGER), 3) AS yes_no,
       round(avg((gold = kind)::INTEGER), 3) AS choice_alone,
       round(avg((gold = kev_only)::INTEGER), 3) AS yes_no_then_choice,
       round(avg((gold = predicted)::INTEGER), 3) AS with_number_check
FROM kev_category_results GROUP BY source ORDER BY source;
```

### Test the number check

`bare_numbers` has 12 valid personnummer, samordningsnummer, and card numbers with no label, and 12 look-alikes: a wrong check digit, month 13, an organisation number, OCR and invoice numbers, phone numbers, and a masked card. The check runs in DuckDB, so this is instant. Which prompts it flags, in every set:

```sql
SELECT source, id, pii_type AS gold, structured_pii(text) AS rule, text
FROM all_prompts
WHERE structured_pii(text) IS NOT NULL
ORDER BY source, id;
```

It flags all 12 valid numbers and `ph-18`, which really does contain a personnummer. It wrongly flags `bn-23`, the phone number `0703151506`: about one in ten 10-digit numbers that start with a valid date also pass the check digit. It flags nothing in the main 100, where most numbers are made up and fail the check digit, so Kev's label reading still covers those.

With the labels, Kev's own answer (`kev_only`) gets 11 of the 12 valid numbers right. In `bn-08` Kev's yes/no answer says sensitive, but its kind question still picks `none`, so `predicted` relies on the rule. After the grouping query above, compare them on `bare_numbers`:

```sql
SELECT id, gold, p, kev_only, rule, predicted, text
FROM kev_category_results
WHERE source = 'bare_numbers'
ORDER BY id;
```

For your own prompts, load them into a table with `id` and `text` columns and replace `all_prompts` (and the gold columns) in the first query.

### Test the value rule and masking (about 1 minute)

Two sets test whether Kev tells naming a kind of data apart from giving its value. Each has 24 prompts, and half of each set gives the value:

- **`secret_mentions`:** passwords, PINs and codes, such as "I have a password that I am not going to tell you".
- **`placeholders`:** every kind of data, written with a placeholder or a refusal, such as "My governmentid is xxxx" or "I would rather not give you my personnummer".

Score both questions in one pass:

```sql
CREATE OR REPLACE TABLE value_rule_results AS
WITH scored AS MATERIALIZED (
  SELECT source, id, tricky, contains_sensitive_pii AS gold, pii_type, text, kev_sensitive(text) AS answers
  FROM all_prompts
  WHERE source IN ('secret_mentions', 'placeholders')
)
SELECT source, id, tricky, gold, pii_type, text,
       (answers->'sensitive'->>'noul')::DOUBLE AS p,
       p >= 0.5 AS predicted,
       answers->'kind'->>'choice' AS choice,
       sensitive_category(answers) AS sensitive_category
FROM scored;
```

This query compares the yes/no answers, the choice answer alone, and checking the yes/no answer first (what `kev_sensitive_category` does), per set:

```sql
SELECT source, count(*) AS n,
       round(avg((gold = predicted)::INTEGER), 3) AS yes_no,
       round(avg((pii_type = choice)::INTEGER), 3) AS choice_alone,
       round(avg((pii_type = sensitive_category)::INTEGER), 3) AS yes_no_then_choice
FROM value_rule_results GROUP BY source;
```

```sql
SELECT source, id, gold, round(p, 3) AS p, choice, sensitive_category, text
FROM value_rule_results
WHERE gold <> predicted OR pii_type <> sensitive_category;
```

To see what the two changes do, switch them off and score again. This turns the `rubric` table back into the text in `rubric.go` and makes masking do nothing:

```sql
UPDATE rubric SET
  instructions = replace(instructions, chr(10) || '- naming a kind of identifier, card, account number, password, PIN, or code without giving its value, for example a placeholder such as xxxx, XXXXXX-XXXX, [REDACTED], or <personnummer>, or a refusal to share it', ''),
  criteria_true = 'Contains a government ID, full card number, CVV or PIN, full account number or IBAN, a secret code, or a full name combined with email, phone, home address, or date of birth.',
  criteria_false = 'Does not contain those. Amounts, dates, merchants, last-four digits, masked values, organisation numbers, ticket IDs, and anonymous questions are allowed.';

CREATE OR REPLACE MACRO mask_placeholders(txt) AS txt;
```

```sql
CREATE OR REPLACE TABLE value_rule_off AS
WITH scored AS MATERIALIZED (
  SELECT source, id, contains_sensitive_pii AS gold, text, kev_noul(text) AS p
  FROM all_prompts
  WHERE source IN ('secret_mentions', 'placeholders')
)
SELECT *, p >= 0.5 AS predicted FROM scored;

SELECT n.source,
       round(avg((n.gold = o.predicted)::INTEGER), 3) AS yes_no_off,
       round(avg((n.gold = n.predicted)::INTEGER), 3) AS yes_no_on
FROM value_rule_results n JOIN value_rule_off o USING (id)
GROUP BY n.source;
```

Run `duckdb/setup.sql` again to switch both back on.

### Try a different rubric

Kev reads the rule from the `rubric` table, so you can edit it and score again:

```sql
UPDATE rubric SET criteria_false = criteria_false || ' An organisation number such as 559999-0001 is allowed.';
```

Then run the `kev_noul_results` query again, under a new table name if you want to keep the old results for comparison. Run `duckdb/setup.sql` again to restore the rubric it defines.

## Troubleshooting

- **`Catalog Error: Scalar Function with name http_post does not exist`**: run `LOAD http_client;` in this session.
- **Connection refused, or every score is `NULL`**: Kev is not running or still loading. Check the server terminal and the `curl` command in step 1.
- **Some scores are `NULL` in a large run**: those requests timed out in DuckDB (status `-1`) before Kev got to them. Run `SET threads = 1;` and score again.
- **`Could not set lock on file`**: another DuckDB process has the database open. Close the UI or that CLI session.
- **Kev-0.8B instead of 4B**: start the server with `KEV_RUN=jaredpalmer/kev-0.8b`. The SQL doesn't change, so name the result tables after the checkpoint.
