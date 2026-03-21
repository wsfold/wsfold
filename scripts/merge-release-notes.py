#!/usr/bin/env python3
import json
import sys
from pathlib import Path


def read_json(path: str) -> dict:
    return json.loads(Path(path).read_text(encoding="utf-8"))


def clean(text: str | None) -> str:
    return (text or "").strip()


def main() -> int:
    if len(sys.argv) != 4:
        print(
            "usage: merge-release-notes.py <current-release.json> <generated-notes.json> <output.json>",
            file=sys.stderr,
        )
        return 1

    current = read_json(sys.argv[1])
    generated = read_json(sys.argv[2])
    output_path = Path(sys.argv[3])

    current_body = clean(current.get("body"))
    generated_body = clean(generated.get("body"))

    if current_body and generated_body:
        merged_body = (
            f"{generated_body}\n\n---\n\n## Commit Changelog\n\n{current_body}"
        )
    else:
        merged_body = generated_body or current_body

    payload = {
        "name": generated.get("name") or current.get("name"),
        "body": merged_body,
    }
    output_path.write_text(json.dumps(payload), encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
