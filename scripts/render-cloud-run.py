#!/usr/bin/env python3
"""Render Cloud Run templates without materializing any sensitive values."""

from __future__ import annotations

import argparse
import os
import re
import tempfile
from pathlib import Path
from urllib.parse import urlsplit

PLACEHOLDER = re.compile(r"__[A-Z0-9_]+__")
DIGEST = re.compile(r"^[a-z0-9][a-z0-9._/-]*@sha256:[a-f0-9]{64}$")
SAFE_NAME = re.compile(r"^[a-z][a-z0-9-]{0,48}$")
REGION = re.compile(r"^[a-z]+[a-z0-9-]*[a-z0-9]$")
SERVICE_ACCOUNT = re.compile(r"^[a-z][a-z0-9-]{4,28}[a-z0-9]@[a-z][a-z0-9-]{4,28}[a-z0-9]\.iam\.gserviceaccount\.com$")
SECRET_NAME = re.compile(r"^[A-Za-z][A-Za-z0-9_-]{0,254}$")
SECRET_VERSION = re.compile(r"^[1-9][0-9]*$")
VERSION = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$")
GITHUB_LOGIN = re.compile(r"^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$")
BUCKET = re.compile(r"^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$")


def required(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if not value:
        raise SystemExit(f"{name} is required")
    if "\n" in value or "\r" in value:
        raise SystemExit(f"{name} cannot contain line breaks")
    return value


def render(template: Path, output: Path, values: dict[str, str]) -> None:
    text = template.read_text(encoding="utf-8")
    for key, value in values.items():
        text = text.replace(f"__{key}__", value)
    unresolved = sorted(set(PLACEHOLDER.findall(text)))
    if unresolved:
        raise SystemExit(f"unresolved placeholders in {template}: {', '.join(unresolved)}")
    output.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary_name = tempfile.mkstemp(prefix=f".{output.name}-", dir=output.parent)
    temporary = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(text)
        os.chmod(temporary, 0o600)
        os.replace(temporary, output)
    finally:
        temporary.unlink(missing_ok=True)


def validate_name(name: str, value: str) -> str:
    if not SAFE_NAME.fullmatch(value) or value.endswith("-"):
        raise SystemExit(f"{name} must be a lowercase Cloud Run resource name with at most 49 characters")
    return value


def validate_digest(name: str, value: str) -> str:
    if not DIGEST.fullmatch(value):
        raise SystemExit(f"{name} must use an immutable @sha256 image digest")
    return value


def validate_origin(name: str, value: str) -> str:
    parsed = urlsplit(value)
    if parsed.scheme != "https" or not parsed.netloc or parsed.username or parsed.password:
        raise SystemExit(f"{name} must be an HTTPS origin")
    if parsed.path not in {"", "/"} or parsed.query or parsed.fragment:
        raise SystemExit(f"{name} must not contain a path, query, or fragment")
    return f"https://{parsed.netloc.lower()}"


def validate_logins(name: str, value: str) -> str:
    logins = [item.strip() for item in value.split(",")]
    if not logins or any(not GITHUB_LOGIN.fullmatch(item) for item in logins):
        raise SystemExit(f"{name} must be a comma-separated list of GitHub logins")
    if len({item.lower() for item in logins}) != len(logins):
        raise SystemExit(f"{name} contains duplicate GitHub logins")
    return ",".join(logins)


def validate_positive_integer(name: str, value: str, maximum: int) -> str:
    try:
        parsed = int(value)
    except ValueError as exc:
        raise SystemExit(f"{name} must be an integer") from exc
    if parsed < 1 or parsed > maximum:
        raise SystemExit(f"{name} must be between 1 and {maximum}")
    return str(parsed)


def secret_reference(prefix: str) -> dict[str, str]:
    name_key = f"{prefix}_SECRET"
    version_key = f"{prefix}_SECRET_VERSION"
    name = required(name_key)
    version = required(version_key)
    if not SECRET_NAME.fullmatch(name):
        raise SystemExit(f"{name_key} has an invalid Secret Manager name")
    if not SECRET_VERSION.fullmatch(version):
        raise SystemExit(f"{version_key} must be a pinned numeric version")
    return {name_key: name, version_key: version}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output-directory", type=Path, default=Path(".tmp/cloud-run"))
    arguments = parser.parse_args()

    version = required("RELEASE_VERSION")
    if not VERSION.fullmatch(version):
        raise SystemExit("RELEASE_VERSION has an invalid format")
    region = required("GCP_REGION")
    if not REGION.fullmatch(region):
        raise SystemExit("GCP_REGION has an invalid format")
    runtime_account = required("RUNTIME_SERVICE_ACCOUNT")
    if not SERVICE_ACCOUNT.fullmatch(runtime_account):
        raise SystemExit("RUNTIME_SERVICE_ACCOUNT must be a Google service-account email")
    application_url = validate_origin("APPLICATION_URL", required("APPLICATION_URL"))
    api_base_url = validate_origin("API_BASE_URL", required("API_BASE_URL"))
    allowed_logins = validate_logins("ALLOWED_GITHUB_LOGINS", required("ALLOWED_GITHUB_LOGINS"))
    superadmin = required("SUPERADMIN_GITHUB_LOGIN")
    if not GITHUB_LOGIN.fullmatch(superadmin):
        raise SystemExit("SUPERADMIN_GITHUB_LOGIN must be one GitHub login")
    if superadmin.lower() not in {item.lower() for item in allowed_logins.split(",")}:
        raise SystemExit("SUPERADMIN_GITHUB_LOGIN must be present in ALLOWED_GITHUB_LOGINS")
    r2_endpoint = validate_origin("R2_ENDPOINT", required("R2_ENDPOINT"))
    r2_bucket = required("R2_BUCKET")
    if not BUCKET.fullmatch(r2_bucket):
        raise SystemExit("R2_BUCKET must be a lowercase 3-63 character bucket name")

    shared = {
        "REGION": region,
        "RELEASE_VERSION": version,
        "RUNTIME_SERVICE_ACCOUNT": runtime_account,
        "APPLICATION_URL": application_url,
        "API_BASE_URL": api_base_url,
        "ALLOWED_GITHUB_LOGINS": allowed_logins,
        "SUPERADMIN_GITHUB_LOGIN": superadmin,
        "R2_ENDPOINT": r2_endpoint,
        "R2_BUCKET": r2_bucket,
    }
    for prefix in (
        "DATABASE_URL",
        "GITHUB_OAUTH_CLIENT_ID",
        "GITHUB_OAUTH_CLIENT_SECRET",
        "R2_ACCESS_KEY_ID",
        "R2_SECRET_ACCESS_KEY",
    ):
        shared.update(secret_reference(prefix))
    api_values = {
        **shared,
        "API_SERVICE_NAME": validate_name("API_SERVICE_NAME", required("API_SERVICE_NAME")),
        "API_MAX_INSTANCES": validate_positive_integer(
            "API_MAX_INSTANCES", required("API_MAX_INSTANCES"), 100
        ),
        "API_IMAGE_DIGEST": validate_digest("API_IMAGE_DIGEST", required("API_IMAGE_DIGEST")),
    }
    worker_values = {
        **shared,
        "WORKER_JOB_NAME": validate_name("WORKER_JOB_NAME", required("WORKER_JOB_NAME")),
        "WORKER_IMAGE_DIGEST": validate_digest("WORKER_IMAGE_DIGEST", required("WORKER_IMAGE_DIGEST")),
    }
    migration_values = {
        **shared,
        "MIGRATION_JOB_NAME": validate_name("MIGRATION_JOB_NAME", required("MIGRATION_JOB_NAME")),
        "MIGRATION_IMAGE_DIGEST": validate_digest(
            "MIGRATION_IMAGE_DIGEST", required("MIGRATION_IMAGE_DIGEST")
        ),
    }
    root = Path(__file__).resolve().parents[1]
    render(
        root / "deploy/cloud-run/api.service.yaml.tmpl",
        arguments.output_directory / "api.service.yaml",
        api_values,
    )
    render(
        root / "deploy/cloud-run/worker.job.yaml.tmpl",
        arguments.output_directory / "worker.job.yaml",
        worker_values,
    )
    render(
        root / "deploy/cloud-run/migrate.job.yaml.tmpl",
        arguments.output_directory / "migrate.job.yaml",
        migration_values,
    )


if __name__ == "__main__":
    main()
