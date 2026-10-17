"""Serve a GLiNER2.5 Decide checkpoint behind Kev's System One contract.

GET  /health        -> {"status": "ok", "model": "<checkpoint>"}
POST /v1/systemone  -> {"answers": {"<id>": {"type": "noul", "noul": p}}}
                    or {"answers": {"<id>": {"type": "choice", "choice": ..., "confidence": ..., "probabilities": {...}}}}

A noul question becomes a single-label task over "true" and "false" with the
criteria as label descriptions. Single-label tasks are softmax, so noul is P(true).
"""
from __future__ import annotations

import json
import os

import uvicorn
from fastapi import FastAPI, HTTPException
from gliner2.classification import ClassificationSchema, Classifier, SchemaError
from huggingface_hub import snapshot_download
from pydantic import BaseModel

RUN = os.environ.get("GLINER_RUN", "fastino/GLiNER2.5-Decide")
DEVICE = os.environ.get("GLINER_DEVICE", "mps")


def local_checkpoint(run: str) -> str:
    """Return a directory transformers 4.x can load.

    GLiNER2.5-multi-Decide stores extra_special_tokens as a list, the
    transformers 5 format. gliner2 pins transformers<5, which expects a dict
    there, so the tokenizer fails to load. The 4.x spelling of a list of added
    tokens is additional_special_tokens.
    """
    src = snapshot_download(run)
    with open(os.path.join(src, "tokenizer_config.json")) as f:
        cfg = json.load(f)
    if not isinstance(cfg.get("extra_special_tokens"), list):
        return src
    dst = os.path.join(os.getcwd(), "models", run.rsplit("/", 1)[-1])
    os.makedirs(dst, exist_ok=True)
    for name in os.listdir(src):
        link = os.path.join(dst, name)
        if name != "tokenizer_config.json" and not os.path.lexists(link):
            os.symlink(os.path.realpath(os.path.join(src, name)), link)
    cfg["additional_special_tokens"] = cfg.pop("extra_special_tokens")
    with open(os.path.join(dst, "tokenizer_config.json"), "w") as f:
        json.dump(cfg, f, indent=2)
    return dst


classifier = Classifier.from_pretrained(local_checkpoint(RUN)).to(DEVICE).eval()
app = FastAPI()


class Question(BaseModel):
    type: str
    instructions: str | None = None
    criteria: dict[str, str]


class Request(BaseModel):
    state: str
    model: str | None = None
    questions: dict[str, Question]


@app.get("/health")
def health():
    return {"status": "ok", "model": RUN.rsplit("/", 1)[-1]}


@app.post("/v1/systemone")
def systemone(req: Request):
    try:
        schema = ClassificationSchema()
        for qid, q in req.questions.items():
            if q.type == "noul":
                labels = {"true": q.criteria["true"], "false": q.criteria["false"]}
            elif q.type == "choice":
                labels = q.criteria
            else:
                raise HTTPException(400, f"question {qid!r}: unsupported type {q.type!r}")
            schema.single(qid, labels, instruction=q.instructions)
        result = classifier.classify(req.state, schema)
    except (SchemaError, KeyError) as exc:
        raise HTTPException(400, str(exc)) from exc

    answers = {}
    for qid, q in req.questions.items():
        task = result[qid]
        probs = dict(task.probabilities)
        if q.type == "noul":
            answers[qid] = {"type": "noul", "noul": probs["true"]}
        else:
            answers[qid] = {
                "type": "choice",
                "choice": task.label,
                "confidence": task.confidence,
                "probabilities": probs,
            }
    return {"answers": answers}


if __name__ == "__main__":
    uvicorn.run(
        app,
        host="127.0.0.1",
        port=int(os.environ.get("GLINER_PORT", "8012")),
        log_level="warning",
    )
