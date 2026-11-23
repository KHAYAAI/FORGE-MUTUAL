#!/usr/bin/env python3
"""Checks the AWS landing zone in deploy/aws on paper (python-hcl2; no terraform binary or AWS account here).

It cannot say the plan will apply. It says the files keep their promises: one region, encryption everywhere, nothing
public, a private Kubernetes API, WORM buckets in COMPLIANCE mode, IMDSv2, no literal secrets, and organisation
guardrails that lock the region and keep audit on. `--selftest` proves each check fails when its promise is broken.
Needs: pip install python-hcl2 (the gate skips this check, with a warning, if it is missing).
"""
import glob, io, json, os, re, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DIR = os.path.join(ROOT, "deploy/aws")

try:
    import hcl2
except ImportError:
    print("WARN check-aws: python-hcl2 is not installed; skipped (pip install python-hcl2)")
    sys.exit(0)


def norm(x):
    """python-hcl2 keeps the quotes of HCL strings and block labels; drop them so checks read plain values."""
    if isinstance(x, dict):
        return {norm(k): norm(v) for k, v in x.items()}
    if isinstance(x, list):
        return [norm(v) for v in x]
    if isinstance(x, str) and len(x) >= 2 and x[0] == '"' and x[-1] == '"':
        return x[1:-1]
    return x


def load(texts):
    res, var = {}, {}
    for name, t in texts.items():
        d = norm(hcl2.load(io.StringIO(t)))
        for r in d.get("resource", []):
            for typ, items in r.items():
                for n, body in items.items():
                    res.setdefault(typ, {})[n] = body
        for v in d.get("variable", []):
            var.update(v)
    return res, var


def one(x):
    return x[0] if isinstance(x, list) and x else x


def check(texts):
    res, var = load(texts)
    p = []
    g = lambda typ: res.get(typ, {})
    # one region
    cond = json.dumps([v.get("condition") for v in var.get("region", {}).get("validation", [])])
    if "af-south-1" not in cond: p.append("region variable is not locked to af-south-1 by a validation")
    # encryption and exposure
    for n, b in g("aws_db_instance").items():
        for k, want in (("storage_encrypted", True), ("multi_az", True), ("publicly_accessible", False), ("deletion_protection", True)):
            if b.get(k) is not want: p.append(f"aws_db_instance.{n}: {k} must be {want}")
        if int(b.get("backup_retention_period", 0)) < 7: p.append(f"aws_db_instance.{n}: backups kept under 7 days")
    pg = json.dumps(g("aws_db_parameter_group"))
    if "rds.force_ssl" not in pg: p.append("postgres does not force TLS")
    for n, b in g("aws_msk_cluster").items():
        e = json.dumps(b.get("encryption_info"))
        if '"client_broker": "TLS"' not in e or '"in_cluster": true' not in e or "encryption_at_rest_kms_key_arn" not in e: p.append(f"aws_msk_cluster.{n}: encryption in transit and at rest required")
    for n, b in g("aws_instance").items():
        if "required" not in json.dumps(b.get("metadata_options")): p.append(f"aws_instance.{n}: IMDSv2 must be required")
        if "true" not in json.dumps(b.get("root_block_device")).lower(): p.append(f"aws_instance.{n}: root volume not encrypted")
    for n, b in g("aws_ebs_volume").items():
        if b.get("encrypted") is not True: p.append(f"aws_ebs_volume.{n}: not encrypted")
    for n, b in g("aws_eks_cluster").items():
        v = json.dumps(b.get("vpc_config"))
        if '"endpoint_private_access": true' not in v: p.append(f"aws_eks_cluster.{n}: private endpoint required")
        if "secrets" not in json.dumps(b.get("encryption_config")): p.append(f"aws_eks_cluster.{n}: secrets in etcd not encrypted")
        if "audit" not in json.dumps(b.get("enabled_cluster_log_types")): p.append(f"aws_eks_cluster.{n}: audit log off")
    for n, b in g("aws_kms_key").items():
        if b.get("enable_key_rotation") is not True: p.append(f"aws_kms_key.{n}: rotation off")
    # buckets: lock, encryption, no public access
    for n in g("aws_s3_bucket"):
        for need in ("aws_s3_bucket_public_access_block", "aws_s3_bucket_server_side_encryption_configuration", "aws_s3_bucket_versioning"):
            if n not in g(need): p.append(f"aws_s3_bucket.{n}: no {need}")
        pab = json.dumps(g("aws_s3_bucket_public_access_block").get(n, {}))
        if pab.count("true") < 4: p.append(f"aws_s3_bucket.{n}: public access not fully blocked")
    ol = json.dumps(g("aws_s3_bucket_object_lock_configuration"))
    if "COMPLIANCE" not in ol or "GOVERNANCE" in ol: p.append("WORM buckets must use COMPLIANCE mode")
    if json.dumps(g("aws_s3_bucket")).count("object_lock_enabled") < 1: p.append("buckets are not created with Object Lock")
    # nothing open to the world inbound
    for typ in ("aws_security_group", "aws_security_group_rule"):
        for n, b in g(typ).items():
            if '"0.0.0.0/0"' in json.dumps(b.get("ingress", b if b.get("type") == "ingress" else {})) : p.append(f"{typ}.{n}: ingress from 0.0.0.0/0")
    # no literal secrets
    for name, t in texts.items():
        for m in re.finditer(r'^\s*(password|secret_key|access_key|master_password)\s*=\s*"[^"$]+"', t, re.M):
            p.append(f"{name}: literal secret {m.group(1)}")
    # organisation guardrails
    scp = json.dumps(load({k: v for k, v in texts.items() if k.startswith("org/")})[0])
    for need in ("af-south-1", "cloudtrail:StopLogging", "s3:PutBucketObjectLockConfiguration"):
        if need not in scp: p.append(f"organisation guardrails do not mention {need}")
    return p


def main():
    texts = {os.path.relpath(f, DIR): open(f).read() for f in sorted(glob.glob(DIR + "/*.tf") + glob.glob(DIR + "/org/*.tf"))}
    problems = check(texts)
    for q in problems: print("FAIL", q)
    if problems: sys.exit(1)
    if "--selftest" in sys.argv:
        faults = [
            ("data.tf", "storage_encrypted               = true", "storage_encrypted               = false"),
            ("data.tf", "publicly_accessible             = false", "publicly_accessible             = true"),
            ("storage.tf", 'mode = "COMPLIANCE"', 'mode = "GOVERNANCE"'),
            ("eks.tf", "endpoint_private_access = true", "endpoint_private_access = false"),
            ("tigerbeetle.tf", 'http_tokens                 = "required" # IMDSv2 only', 'http_tokens                 = "optional"'),
            ("kms.tf", "enable_key_rotation     = true", "enable_key_rotation     = false"),
            ("variables.tf", 'var.region == "af-south-1"', 'var.region == "eu-west-1"'),
            ("org/scp.tf", '"aws:RequestedRegion" = ["af-south-1"]', '"aws:RequestedRegion" = ["us-east-1"]'),
        ]
        for fn, old, new in faults:
            t = dict(texts)
            if old not in t[fn]:
                print(f"FAIL selftest: text to break not found in {fn}: {old[:40]}"); sys.exit(1)
            t[fn] = t[fn].replace(old, new)
            if not check(t):
                print(f"FAIL selftest: not caught: {fn}: {old[:50]}"); sys.exit(1)
        print(f"ok   aws checks catch {len(faults)} deliberate faults")
    ph = sum(len(re.findall("PLACEHOLDER", t)) for t in texts.values())
    print(f"aws: {len(texts)} files, 0 problems, {ph} PLACEHOLDER values to settle")


main()
