-- Loads the prompt set and defines SQL macros that call the local Kev server.
-- Run from the repo root: duckdb duckdb/eval.duckdb -f duckdb/setup.sql
-- Safe to re-run; it replaces the prompts, rubric, and macros but keeps result tables.
-- Each session needs LOAD http_client and SET threads = 1. With more threads DuckDB sends requests in parallel,
-- Kev answers them one batch at a time, and queued requests time out as status -1 (NULL answers).

INSTALL http_client FROM community;
LOAD http_client;
INSTALL ui;

CREATE OR REPLACE TABLE prompts AS
SELECT * FROM read_json('data/prompts.json');

-- 24 prompts that mention a password, PIN, or code, half with the value and half without.
CREATE OR REPLACE TABLE secret_mentions AS
SELECT * FROM read_json('data/secret_mentions.json');

-- 24 prompts that name a kind of data with a placeholder or a refusal, half with the real value instead.
CREATE OR REPLACE TABLE placeholders AS
SELECT * FROM read_json('data/placeholders.json');

-- 24 prompts with personnummer, samordningsnummer, or card numbers and no label, half valid and half look-alikes.
CREATE OR REPLACE TABLE bare_numbers AS
SELECT * FROM read_json('data/bare_numbers.json');

CREATE OR REPLACE VIEW all_prompts AS
SELECT 'main' AS source, * FROM prompts
UNION ALL BY NAME SELECT 'secret_mentions' AS source, * FROM secret_mentions
UNION ALL BY NAME SELECT 'placeholders' AS source, * FROM placeholders
UNION ALL BY NAME SELECT 'bare_numbers' AS source, * FROM bare_numbers;

-- Matches internal/rubric/rubric.go except for the value rule: the last "allowed" line in instructions
-- and the "actual value" / "without its value" wording in both criteria. Go eval reports use the rubric without it.
CREATE OR REPLACE TABLE rubric AS
SELECT
  $$Decide whether the text contains sensitive personal data that must not be sent to a general-purpose LLM.

Answer true if the text includes any of the following:
- a government identifier, including a Swedish personnummer or samordningsnummer, a passport number, or a driver's license number
- a full payment card number
- a card CVV, CVC, or PIN
- a full bank account number or IBAN
- a password, one-time code, or account recovery code
- a customer's full name together with their email address, phone number, home address, or date of birth

Answer false if the text does not include those. The following are allowed and are not sensitive on their own:
- amounts, currencies, due dates, and merchant names
- the last four digits of a card or account
- masked values such as 900101-**** or **** **** **** 4242
- public company facts and Swedish organisation numbers
- ticket IDs, invoice numbers, and anonymous product questions
- a first name alone, without a government ID, a full account number, or the combinations above
- naming a kind of identifier, card, account number, password, PIN, or code without giving its value, for example a placeholder such as xxxx, XXXXXX-XXXX, [REDACTED], or <personnummer>, or a refusal to share it

Ignore how the text is phrased. JSON, email, Swedish, and English all follow the same rule.$$ AS instructions,
  $$Contains the actual value of a government ID, full card number, CVV or PIN, full account number or IBAN, password, or one-time code, or a full name combined with email, phone, home address, or date of birth.$$ AS criteria_true,
  $$Does not contain those. Amounts, dates, merchants, last-four digits, masked values, organisation numbers, ticket IDs, and anonymous questions are allowed. Naming one of those without its value, such as a placeholder like xxxx or [REDACTED], is allowed.$$ AS criteria_false,
  $$Choose the one kind of sensitive personal data in the text.

Use none when the text does not contain sensitive personal data. A last-four, a masked value, an organisation number, a ticket id, an amount, a date, or a first name alone is none.

Use government_id for a Swedish personnummer or samordningsnummer, a passport number, or a driver's license number.
Use payment_card for a full payment card number or a card CVV or CVC.
Use bank_account for a full bank account number or an IBAN.
Use secret for a password, PIN, one-time code, or account recovery code.
Use named_contact for a customer's full name together with an email address, phone number, home address, or date of birth.$$ AS type_instructions;

CREATE OR REPLACE TABLE rubric_types (pos INTEGER, name VARCHAR, description VARCHAR);
INSERT INTO rubric_types VALUES
  (1, 'none', $$No sensitive personal data. Amounts, dates, merchants, last-four digits, masked values, organisation numbers, ticket ids, and a first name alone belong here.$$),
  (2, 'government_id', $$A Swedish personnummer or samordningsnummer, a passport number, or a driver's license number.$$),
  (3, 'payment_card', $$A full payment card number, or a card CVV or CVC.$$),
  (4, 'bank_account', $$A full bank account number, a clearing number plus account, or an IBAN.$$),
  (5, 'secret', $$A password, PIN, one-time code, or account recovery code.$$),
  (6, 'named_contact', $$A customer's full name together with an email address, phone number, home address, or date of birth.$$);

CREATE OR REPLACE MACRO kev_url() AS 'http://127.0.0.1:8009';

-- Same patterns as internal/masking/masking.go.
CREATE OR REPLACE MACRO mask_text(txt) AS
  regexp_replace(
    regexp_replace(txt, '\b(?:\d{2})?\d{6}[-+]?[*xX•]{4}', 'masked', 'g'),
    '(?:[*xX•]{4}[ -]?)+\d{2,4}\b', 'masked', 'g');

-- mask_text plus placeholders that stand in for a value: [REDACTED], <personnummer>, xxxx, XXXXXX-XXXX,
-- **** **** **** ****. Kev otherwise reads the label in front of them ("personnummer", "IBAN") as the value.
CREATE OR REPLACE MACRO mask_placeholders(txt) AS
  regexp_replace(regexp_replace(regexp_replace(regexp_replace(mask_text(txt),
    '\[(?i:redacted|removed|masked|hidden)\]', 'masked', 'g'),
    '<[A-Za-zåäöÅÄÖ_ ]+>', 'masked', 'g'),
    '\b[xX]{3,}(?:[- ][xX]{2,})*\b', 'masked', 'g'),
    '\*{3,}(?:[ -]\*{2,})*', 'masked', 'g');

-- Rule-based check for numbers Kev misses without a label in front. Kev judges by context, so a bare
-- 197306253977 scores low; these macros check the number itself: a real date and a passing check digit.
CREATE OR REPLACE MACRO digits_only(s) AS regexp_replace(s, '\D', '', 'g');

-- Luhn check digit, used by personnummer (on the 10-digit form) and card numbers.
CREATE OR REPLACE MACRO luhn_ok(digits) AS
  list_sum(list_transform(range(length(digits)), lambda i:
    CASE WHEN i % 2 = 0 THEN substr(reverse(digits), i + 1, 1)::INTEGER
         WHEN 2 * substr(reverse(digits), i + 1, 1)::INTEGER > 9 THEN 2 * substr(reverse(digits), i + 1, 1)::INTEGER - 9
         ELSE 2 * substr(reverse(digits), i + 1, 1)::INTEGER END)) % 10 = 0;

-- Month and day of a 10-digit personnummer; samordningsnummer add 60 to the day.
CREATE OR REPLACE MACRO personnummer_mmdd(ten) AS
  substr(ten, 3, 2) || lpad((substr(ten, 5, 2)::INTEGER % 60)::VARCHAR, 2, '0');

-- YYMMDD-NNNN, YYMMDDNNNN, YYYYMMDD-NNNN, or YYYYMMDDNNNN with a real date and a passing check digit.
CREATE OR REPLACE MACRO personnummer_ok(candidate) AS
  luhn_ok(right(digits_only(candidate), 10)) AND CASE
    WHEN length(digits_only(candidate)) = 12 THEN
      left(digits_only(candidate), 2) IN ('18', '19', '20')
      AND try_strptime(left(digits_only(candidate), 4) || personnummer_mmdd(right(digits_only(candidate), 10)), '%Y%m%d') IS NOT NULL
    ELSE
      try_strptime('19' || left(digits_only(candidate), 2) || personnummer_mmdd(digits_only(candidate)), '%Y%m%d') IS NOT NULL
      OR try_strptime('20' || left(digits_only(candidate), 2) || personnummer_mmdd(digits_only(candidate)), '%Y%m%d') IS NOT NULL
  END;

-- 13 to 19 digits, optionally grouped with spaces or dashes, starting 2-6, with a passing check digit.
CREATE OR REPLACE MACRO card_number_ok(candidate) AS
  length(digits_only(candidate)) BETWEEN 13 AND 19
  AND left(digits_only(candidate), 1) IN ('2', '3', '4', '5', '6')
  AND luhn_ok(digits_only(candidate));

-- The numbers in the text that pass the checks, as written.
CREATE OR REPLACE MACRO personnummer_matches(txt) AS
  list_distinct(list_filter(regexp_extract_all(txt, '\b(?:\d{2})?\d{6}[-+]?\d{4}\b'), lambda c: personnummer_ok(c)));

CREATE OR REPLACE MACRO card_number_matches(txt) AS
  list_distinct(list_filter(regexp_extract_all(txt, '\b\d(?:[ -]?\d){12,18}\b'), lambda c: card_number_ok(c)));

-- government_id, payment_card, or NULL when no number passes. About one in ten random 10-digit numbers that
-- start with a date also pass, so a bare phone number such as 0703151506 can match.
CREATE OR REPLACE MACRO structured_pii(txt) AS CASE
  WHEN len(personnummer_matches(txt)) > 0 THEN 'government_id'
  WHEN len(card_number_matches(txt)) > 0 THEN 'payment_card'
END;

CREATE OR REPLACE MACRO label_matches(txt, matches, label) AS CASE
  WHEN len(matches) = 0 THEN txt
  ELSE regexp_replace(txt, '\b(?:' || array_to_string(list_transform(matches, lambda m: replace(m, '+', '\+')), '|') || ')\b',
                      '[' || label || ': \0]', 'g')
END;

-- Puts a label on each number that passes, such as [personnummer: 197306253977]. Kev scores a bare valid
-- personnummer around 0.4 and the labelled one above 0.9.
CREATE OR REPLACE MACRO label_numbers(txt) AS
  label_matches(label_matches(label_matches(txt,
    list_filter(personnummer_matches(txt), lambda c: substr(right(digits_only(c), 10), 5, 2)::INTEGER <= 60), 'personnummer'),
    list_filter(personnummer_matches(txt), lambda c: substr(right(digits_only(c), 10), 5, 2)::INTEGER > 60), 'samordningsnummer'),
    card_number_matches(txt), 'card number');

-- The text every kev_* macro sends: placeholders masked, valid numbers labelled.
CREATE OR REPLACE MACRO kev_input(txt) AS label_numbers(mask_placeholders(txt));

-- The yes/no question: is there sensitive data?
CREATE OR REPLACE MACRO kev_noul_question() AS json_object(
  'type', 'noul',
  'instructions', (SELECT instructions FROM rubric),
  'criteria', json_object(
    'true', (SELECT criteria_true FROM rubric),
    'false', (SELECT criteria_false FROM rubric)));

-- The choice question: which kind of data?
CREATE OR REPLACE MACRO kev_choice_question() AS json_object(
  'type', 'choice',
  'instructions', (SELECT type_instructions FROM rubric),
  'criteria', (SELECT to_json(map_from_entries(list({'k': name, 'v': description} ORDER BY pos))) FROM rubric_types));

-- Every request sends kev_input, so all kev_* macros mask and label automatically.
CREATE OR REPLACE MACRO kev_request(txt, questions) AS json_object(
  'state', kev_input(txt),
  'model', 'kev-latest',
  'questions', questions);

CREATE OR REPLACE MACRO kev_noul_request(txt) AS
  kev_request(txt, json_object('pii', kev_noul_question()));

CREATE OR REPLACE MACRO kev_choice_request(txt) AS
  kev_request(txt, json_object('pii', kev_choice_question()));

-- Both questions in one request; Kev reads the text once and answers each question separately.
CREATE OR REPLACE MACRO kev_sensitive_request(txt) AS
  kev_request(txt, json_object('sensitive', kev_noul_question(), 'kind', kev_choice_question()));

-- Full Kev response body as JSON, or NULL with the error in the result if the call failed.
CREATE OR REPLACE MACRO kev_post(body) AS (
  WITH r AS (
    SELECT http_post(kev_url() || '/v1/systemone', MAP {'content-type': 'application/json'}, body) AS res
  )
  SELECT CASE WHEN (res->>'status')::INTEGER = 200
              THEN (res->>'body')::JSON
              ELSE json_object('error', res->>'status', 'detail', res->>'body') END
  FROM r
);

-- Probability (0..1) that the text contains sensitive PII. The Go eval treats >= 0.5 as sensitive.
CREATE OR REPLACE MACRO kev_noul(txt) AS
  (kev_post(kev_noul_request(txt))->'answers'->'pii'->>'noul')::DOUBLE;

-- {"choice": ..., "confidence": ..., "probabilities": {...}} for the PII-kind question.
CREATE OR REPLACE MACRO kev_choice(txt) AS
  kev_post(kev_choice_request(txt))->'answers'->'pii';

-- One row per PII kind with its probability, most likely first: SELECT * FROM kev_choice_breakdown('...');
CREATE OR REPLACE MACRO kev_choice_breakdown(txt) AS TABLE
  WITH a AS MATERIALIZED (SELECT kev_choice(txt) AS answer)
  SELECT p.key AS category,
         round(p.value::DOUBLE, 4) AS probability,
         p.key = (a.answer->>'choice') AS chosen
  FROM a, json_each(a.answer->'probabilities') p
  ORDER BY probability DESC;

-- One row with a column per field: SELECT * FROM kev_choice_row('...');
-- The probability fields must list the same names as rubric_types.
CREATE OR REPLACE MACRO kev_choice_row(txt) AS TABLE
  WITH a AS MATERIALIZED (SELECT kev_choice(txt) AS answer)
  SELECT unnest(json_transform(answer, '{"choice": "VARCHAR", "confidence": "DOUBLE", "probabilities": {"none": "DOUBLE", "government_id": "DOUBLE", "payment_card": "DOUBLE", "bank_account": "DOUBLE", "secret": "DOUBLE", "named_contact": "DOUBLE"}}'), recursive := true)
  FROM a;

-- The PII kind alone: none, government_id, payment_card, bank_account, secret, or named_contact.
CREATE OR REPLACE MACRO kev_category(txt) AS
  kev_choice(txt)->>'choice';

-- Both answers from one call: {"sensitive": {"noul": ...}, "kind": {"choice": ..., "confidence": ..., "probabilities": {...}}}.
CREATE OR REPLACE MACRO kev_sensitive(txt) AS
  kev_post(kev_sensitive_request(txt))->'answers';

-- 'none' unless the noul answer says sensitive, then the choice answer; NULL if the call failed. The choice question
-- alone files a mention of a kind of data without its value under that kind; the noul question does not.
CREATE OR REPLACE MACRO sensitive_category(answers) AS
  CASE WHEN (answers->'sensitive'->>'noul') IS NULL THEN NULL
       WHEN (answers->'sensitive'->>'noul')::DOUBLE >= 0.5 THEN answers->'kind'->>'choice'
       ELSE 'none' END;

-- The rule's category when a valid personnummer or card number is in the text, otherwise Kev's.
CREATE OR REPLACE MACRO pii_category(txt, answers) AS
  coalesce(structured_pii(txt), sensitive_category(answers));

-- One call. The MATERIALIZED CTE keeps DuckDB from sending the request once per reference to the answers.
CREATE OR REPLACE MACRO kev_sensitive_category(txt) AS (
  WITH a AS MATERIALIZED (SELECT kev_sensitive(txt) AS answers)
  SELECT pii_category(txt, answers) FROM a
);

-- One call, one row: SELECT * FROM kev_sensitive_row('...');
-- category combines the rule and Kev; rule, p, and kind show each part on its own.
CREATE OR REPLACE MACRO kev_sensitive_row(txt) AS TABLE
  WITH a AS MATERIALIZED (SELECT kev_sensitive(txt) AS answers)
  SELECT pii_category(txt, answers) AS category,
         structured_pii(txt) AS rule,
         round((answers->'sensitive'->>'noul')::DOUBLE, 4) AS p,
         answers->'kind'->>'choice' AS kind,
         round((answers->'kind'->>'confidence')::DOUBLE, 4) AS kind_confidence
  FROM a;

-- Metrics for a result table with boolean columns gold and predicted.
CREATE OR REPLACE MACRO binary_metrics(tbl) AS TABLE
  SELECT
    count(*) AS n,
    count(*) FILTER (gold AND predicted) AS tp,
    count(*) FILTER (NOT gold AND predicted) AS fp,
    count(*) FILTER (gold AND NOT predicted) AS fn,
    count(*) FILTER (NOT gold AND NOT predicted) AS tn,
    round(avg((gold = predicted)::INTEGER), 3) AS accuracy,
    round(tp / nullif(tp + fp, 0), 3) AS precision,
    round(tp / nullif(tp + fn, 0), 3) AS recall,
    round(2 * tp / nullif(2 * tp + fp + fn, 0), 3) AS f1
  FROM query_table(tbl);
