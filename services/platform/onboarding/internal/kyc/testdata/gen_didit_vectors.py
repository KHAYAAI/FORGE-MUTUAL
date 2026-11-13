#!/usr/bin/env python3
"""Regenerates didit_v2_vectors.json for didit_test.go.

Each vector is a JSON document as Didit might send it, with the canonical form and the X-Signature-V2 value that
Didit's own reference code produces for it. The Go tests must reproduce both exactly.

The reference functions below are copied from Didit's webhook documentation, "Python / FastAPI (recommended:
X-Signature-V2)", https://docs.didit.me/integration/webhooks, as read on 2026-11-13. If that page changes, update them.

    python3 gen_didit_vectors.py > didit_v2_vectors.json
"""
import hashlib
import hmac
import json
import random
import sys

SECRET = "whsec_vectors_only_not_a_real_secret_0123456789"


# ---- Didit's reference, verbatim in behaviour --------------------------------------------------------------------
def shorten_floats(data):
    """Match Didit: whole-valued floats serialise as ints."""
    if isinstance(data, dict):
        return {k: shorten_floats(v) for k, v in data.items()}
    if isinstance(data, list):
        return [shorten_floats(x) for x in data]
    if isinstance(data, float) and data.is_integer():
        return int(data)
    return data


def canonical(body_json):
    return json.dumps(
        shorten_floats(body_json),
        sort_keys=True,
        separators=(",", ":"),
        ensure_ascii=False,  # unescaped Unicode -- matches Didit's V2 canonical form
    )


def sign_v2(body_json, secret):
    return hmac.new(secret.encode("utf-8"), canonical(body_json).encode("utf-8"), hashlib.sha256).hexdigest()
# ---- end of the reference -------------------------------------------------------------------------------------------


STRINGS = [
    "", "plain", "José Ramírez", "Thandi Mokoena", "日本語のテキスト", "emoji 😀 and 𝒳", "quote \" and slash \\",
    "tab\tnewline\ncarriage\rreturn", "bell \u0007 and nul-adjacent \u0001\u001f", "del \u007f", "ls   ps  ",
    "html <b>&amp;</b>", "slash / and unicode escape lookalike \\u00e9", "mixed ÀÉÎÕÜ ß ñ", "é vs é",
]


def rand_string(rng):
    return rng.choice(STRINGS) if rng.random() < 0.5 else "".join(
        rng.choice("abcXYZ019 _-éß日😀\"\\\n\t") for _ in range(rng.randint(0, 12))
    )


def rand_number(rng):
    kind = rng.randint(0, 9)
    if kind == 0:
        return rng.randint(-1000, 1000)
    if kind == 1:
        return rng.randint(10**18, 10**30) * rng.choice([-1, 1])  # beyond int64
    if kind == 2:
        return float(rng.randint(-1000, 1000))  # whole float
    if kind == 3:
        return rng.uniform(-1000, 1000)
    if kind == 4:
        return rng.random()
    if kind == 5:
        return rng.uniform(1, 10) * 10.0 ** rng.randint(-12, -5)  # tiny: Python writes an exponent
    if kind == 6:
        return rng.uniform(1, 10) * 10.0 ** rng.randint(-4, -1)  # small but plain
    if kind == 7:
        return 10.0 ** rng.randint(15, 25)  # whole, large
    if kind == 8:
        return round(rng.uniform(0, 1), rng.randint(1, 6))  # scores such as 0.97
    return rng.uniform(1, 9) * 10 ** rng.randint(3, 14)  # large, may be whole or not


def rand_value(rng, depth):
    kinds = ["str", "num", "bool", "null"] + (["obj", "list"] * 2 if depth < 4 else [])
    kind = rng.choice(kinds)
    if kind == "str":
        return rand_string(rng)
    if kind == "num":
        return rand_number(rng)
    if kind == "bool":
        return rng.random() < 0.5
    if kind == "null":
        return None
    if kind == "list":
        return [rand_value(rng, depth + 1) for _ in range(rng.randint(0, 5))]
    return {rand_string(rng): rand_value(rng, depth + 1) for _ in range(rng.randint(0, 6))}


HANDCRAFTED = {
    "status updated, approved (shape from Didit's docs)": {
        "event_id": "9c0c8b8a-1111-4222-9333-444444444444", "webhook_type": "status.updated", "timestamp": 1774970000,
        "created_at": 1774969994, "application_id": "11111111-2222-3333-4444-555555555555", "environment": "live",
        "session_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "Approved",
        "workflow_id": "22222222-3333-4444-5555-666666666666", "workflow_version": 4, "vendor_data": "user-123",
        "metadata": {"onboarding_id": "o-1", "subject_party_id": "user-123"},
        "decision": {"session_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "Approved",
                     "id_verifications": [{"node_id": "ocr_1", "status": "Approved", "full_name": "José Ramírez",
                                           "date_of_birth": "1990-04-02", "confidence": 0.9876}],
                     "liveness_checks": [{"node_id": "live_1", "status": "Approved", "score": 98.0}],
                     "face_matches": [{"node_id": "fm_1", "status": "Approved", "score": 87.5}], "reviews": []},
    },
    "status updated, in progress (not a verdict)": {
        "event_id": "1f0e0d0c-aaaa-4bbb-8ccc-dddddddddddd", "webhook_type": "status.updated", "timestamp": 1774970000,
        "environment": "live", "session_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "In Progress",
        "vendor_data": "user-123", "metadata": {"onboarding_id": "o-1", "subject_party_id": "user-123"},
    },
    "status updated, declined": {
        "event_id": "2f0e0d0c-aaaa-4bbb-8ccc-dddddddddddd", "webhook_type": "status.updated", "timestamp": 1774970000,
        "environment": "live", "session_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "Declined",
        "vendor_data": "user-123", "metadata": {"onboarding_id": "o-1", "subject_party_id": "user-123"},
        "decision": {"status": "Declined", "aml_screenings": [{"status": "Declined", "total_hits": 2, "hits": [{"score": 0.93}]}]},
    },
    "whole floats and integers": {"a": 1.0, "b": 100.0, "c": -3.0, "d": 7, "e": 1e3, "f": 0.0, "g": -0.0, "h": 2.50},
    "tiny and plain floats": {"a": 0.0001, "b": 0.00001, "c": 1.5e-7, "d": -2.5e-6, "e": 0.30000000000000004, "f": 123456.789},
    "very large": {"a": 12345678901234567890123, "b": 1e20, "c": 1e16, "d": 123456789012345680.0, "e": -1e25},
    "unicode keys and sorting": {"é": 1, "z": 2, "A": 3, "a": 4, "日本": 5, "😀": 6, "Z": 7, "": 8},
    "string escapes": {"s": "q\" b\\ n\n r\r t\t bs\b ff\f c\u0001 e\u001f d\u007f ls  ps  <&>"},
    "empty containers and nulls": {"o": {}, "l": [], "n": None, "t": True, "f": False, "nested": [[], {}, [None]]},
    "top level array": [1, 2.0, "x", {"k": [3.5]}],
    "top level string": "just a string",
    "top level number": 42.0,
}


def vector(name, doc, style):
    if style == 0:
        body = json.dumps(doc, ensure_ascii=True)  # how Didit transmits it
    elif style == 1:
        body = json.dumps(doc, indent=2, ensure_ascii=False)
    else:
        body = json.dumps(doc, sort_keys=False, separators=(", ", ": "), ensure_ascii=True)
    return {"name": name, "body": body, "canonical": canonical(doc), "signature": sign_v2(doc, SECRET)}


def main():
    rng = random.Random(20261113)
    vectors = []
    for name, doc in HANDCRAFTED.items():
        for style in (0, 1, 2):
            vectors.append(vector(f"{name} [style {style}]", doc, style))
    for i in range(150):
        doc = {rand_string(rng): rand_value(rng, 1) for _ in range(rng.randint(1, 6))}
        vectors.append(vector(f"random {i}", doc, i % 3))
    json.dump({"secret": SECRET, "vectors": vectors}, sys.stdout, ensure_ascii=True, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
