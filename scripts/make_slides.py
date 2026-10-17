"""Builds results/slides/model-comparison.pptx from the compare-improved.sh results.

Run from the repo root: uv run --with python-pptx python scripts/make_slides.py
"""
from pptx import Presentation
from pptx.dml.color import RGBColor
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from pptx.util import Inches, Pt

NAVY = RGBColor(0x1F, 0x2A, 0x44)
GREY = RGBColor(0x6B, 0x72, 0x80)
LIGHT = RGBColor(0xF3, 0xF4, 0xF6)
GREEN = RGBColor(0x0F, 0x7B, 0x4A)
GREEN_BG = RGBColor(0xE3, 0xF4, 0xEA)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)

# model, (before, after) for yes/no, gate, kind, yes/no then kind; accuracy on 172 prompts
ROWS = [
    ("Kev-4B", (0.808, 0.948), (0.814, 0.843), (0.797, 0.837), (0.831, 0.953)),
    ("Decider-4B", (0.808, 0.913), (0.808, 0.860), (0.791, 0.849), (0.808, 0.913)),
    ("Strands Decider 2B", (0.529, 0.767), (0.686, 0.756), (0.686, 0.756), (0.535, 0.762)),
    ("Kev-0.8B", (0.570, 0.686), (0.669, 0.744), (0.669, 0.744), (0.570, 0.686)),
    ("GLiNER2.5-Decide", (0.500, 0.587), (0.651, 0.727), (0.645, 0.721), (0.500, 0.581)),
    ("GLiNER2.5-multi-Decide", (0.535, 0.547), (0.698, 0.721), (0.593, 0.640), (0.547, 0.552)),
    ("Laya multilingual", (0.523, 0.529), (0.640, 0.680), (0.552, 0.616), (0.570, 0.634)),
]
HEADERS = ["Model", "Yes/no", "Gate\n(kind → yes/no)", "Kind", "Yes/no, then kind"]


def pct(x):
    return f"{int(x * 100 + 0.5 + 1e-9)}%"


def text(slide, left, top, width, height, paragraphs, size=12, color=NAVY):
    """paragraphs: list of str or list of (text, bold, color) runs."""
    box = slide.shapes.add_textbox(Inches(left), Inches(top), Inches(width), Inches(height))
    tf = box.text_frame
    tf.word_wrap = True
    for i, para in enumerate(paragraphs):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        runs = [(para, False, color)] if isinstance(para, str) else para
        for t, bold, c in runs:
            r = p.add_run()
            r.text = t
            r.font.size = Pt(size)
            r.font.bold = bold
            r.font.color.rgb = c
        p.space_after = Pt(6)
    return box


def cell_fill(cell, rgb):
    cell.fill.solid()
    cell.fill.fore_color.rgb = rgb


prs = Presentation()
prs.slide_width, prs.slide_height = Inches(13.333), Inches(7.5)
blank = prs.slide_layouts[6]

# Slide 1: results
s = prs.slides.add_slide(blank)
text(s, 0.5, 0.3, 12.3, 0.8, [[("Seven local models tested: small fixes lift the best to 95% accuracy", True, NAVY)]], size=28)
text(s, 0.5, 1.0, 12.3, 0.5, ["Accuracy on 172 labelled prompts (100 standard + 72 hard cases), before → after optimisation. 50% is guessing."], size=14, color=GREY)

col_w = [2.4, 1.35, 1.6, 1.35, 1.6]
tbl = s.shapes.add_table(len(ROWS) + 1, 5, Inches(0.5), Inches(1.65), Inches(sum(col_w)), Inches(4.7)).table
for i, w in enumerate(col_w):
    tbl.columns[i].width = Inches(w)
best = [max(r[c][1] for r in ROWS) for c in range(1, 5)]
for c, h in enumerate(HEADERS):
    cell = tbl.cell(0, c)
    cell_fill(cell, NAVY)
    cell.text = h
    for p in cell.text_frame.paragraphs:
        p.alignment = PP_ALIGN.LEFT if c == 0 else PP_ALIGN.CENTER
        for r in p.runs:
            r.font.size, r.font.bold, r.font.color.rgb = Pt(12), True, WHITE
    cell.vertical_anchor = MSO_ANCHOR.MIDDLE
for ri, row in enumerate(ROWS, start=1):
    for c in range(5):
        cell = tbl.cell(ri, c)
        cell_fill(cell, WHITE if ri % 2 else LIGHT)
        cell.vertical_anchor = MSO_ANCHOR.MIDDLE
        p = cell.text_frame.paragraphs[0]
        if c == 0:
            r = p.add_run()
            r.text = row[0]
            r.font.size, r.font.bold, r.font.color.rgb = Pt(12), True, NAVY
            continue
        before, after = row[c]
        p.alignment = PP_ALIGN.CENTER
        r = p.add_run()
        r.text = f"{pct(before)} → "
        r.font.size, r.font.color.rgb = Pt(11), GREY
        r = p.add_run()
        r.text = pct(after)
        r.font.size, r.font.bold, r.font.color.rgb = Pt(14), True, GREEN if after == best[c - 1] else NAVY
        if after == best[c - 1]:
            cell_fill(cell, GREEN_BG)

text(s, 9.0, 1.6, 3.9, 5.0, [
    [("No retraining needed. ", True, NAVY), ("The models are unchanged; we clarified the rule, cleaned the input, and added a checksum test for ID and card numbers.", False, NAVY)],
    [("Two models stand out. ", True, NAVY), ("Kev-4B 81% → 95% and Decider-4B 81% → 91%. The smaller models gain less, and much of their gain comes from the checksum test.", False, NAVY)],
    [("The gate helps weak models, not strong ones. ", True, NAVY), ("Asking “which kind?” and treating anything but “none” as sensitive lifts Laya and GLiNER, but cannot tell “my ID is xxxx” from a real ID.", False, NAVY)],
    [("Best setup: yes/no first, then the kind. ", True, GREEN), ("Kev-4B classifies 95% correctly, including the kind of data. It only works for models whose yes/no answer is already good.", False, NAVY)],
], size=13)

text(s, 0.5, 6.55, 12.3, 0.7, [
    "Before: original rule, already-masked values hidden. After: rule says naming data without its value is fine; placeholders (xxxx, [REDACTED]) hidden; valid personnummer and card numbers labelled and always flagged. "
    "Prompts are synthetic; all models run locally behind the same System One API.",
], size=10, color=GREY)

# Slide 2: how to read it
s = prs.slides.add_slide(blank)
text(s, 0.5, 0.3, 12.3, 0.8, [[("How to read the results", True, NAVY)]], size=28)

cards = [
    ("Yes/no", "One question: does the text contain sensitive personal data? The model returns a probability; 50% or more counts as yes. Scored as right or wrong."),
    ("Gate (kind → yes/no)", "One question: which kind of sensitive data is in the text (ID, card, account, secret, name + contact, or none)? Anything but “none” counts as yes. Scored as yes/no."),
    ("Kind", "The same question, but the exact kind must be right. A real ID filed as “card” counts as wrong."),
    ("Yes/no, then kind", "Ask yes/no first. Only if yes, ask which kind; otherwise the answer is “none”. Scored on the exact kind. Same idea as the DuckDB category query."),
]
for i, (title, body) in enumerate(cards):
    left = 0.5 + i * 3.1
    box = s.shapes.add_shape(1, Inches(left), Inches(1.2), Inches(2.9), Inches(2.6))
    box.fill.solid()
    box.fill.fore_color.rgb = LIGHT
    box.line.fill.background()
    text(s, left + 0.15, 1.3, 2.6, 2.4, [[(title, True, GREEN if i == 3 else NAVY)], body], size=12)

text(s, 0.5, 4.05, 6.0, 3.0, [
    [("What we changed (no retraining)", True, NAVY)],
    "• Rule: naming a kind of data without its value is not sensitive (“My ID is xxxx”, “I won’t tell you my password”).",
    "• Input: placeholders such as xxxx, [REDACTED], <personnummer> become “masked”; valid numbers get a label, e.g. [personnummer: 197306253977].",
    "• Checksum test: a real date plus a passing Luhn check digit means a personnummer, samordningsnummer, or card number is always flagged.",
], size=12)

text(s, 6.9, 4.05, 6.0, 3.0, [
    [("Test set and caveats", True, NAVY)],
    "• 100 standard prompts plus 72 hard cases: password mentions without a value, placeholders and refusals, and bare numbers with look-alikes. Half are sensitive.",
    "• The rule only changes the yes/no question, so the gate and kind columns gain less; their gains come from cleaner input and the checksum test.",
    "• “Yes/no, then kind” loses true positives when the yes/no answer misses them, so it is worse for Kev-0.8B and GLiNER. About one in ten bare phone numbers passes the checksum test.",
], size=12)

prs.save("results/slides/model-comparison.pptx")

print("saved")
