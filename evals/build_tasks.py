#!/usr/bin/env python3
"""Generate historical task candidates; no models or Go tests are executed."""
import json
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[1]
ANCHOR = "379a18c5"


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT, text=True).strip()


def main():
    tasks = []
    history = git("log", ANCHOR, "-600", "--format=%H %s", "--no-merges")
    for line in history.splitlines():
        fix, subject = line.split(" ", 1)
        if not subject.startswith("fix("):
            continue
        base = git("rev-parse", fix + "^")
        files = git("diff", "--name-only", base, fix).splitlines()
        tests = []
        for path in files:
            if not path.endswith("_test.go"):
                continue
            diff = git("diff", "--unified=0", base, fix, "--", path)
            names = re.findall(r"^\+func (Test\w+)\(\w+ \*testing\.T\)", diff, re.M)
            if not names:
                continue
            source = git("show", fix + ":" + path)
            constraint = re.search(r"^//go:build (.+)$", source, re.M)
            tests.append({"file": path, "names": names,
                          "tags": constraint.group(1) if constraint else ""})
        if not tests:
            continue
        title = re.sub(r" \(#\d+\)$", "", subject.split(": ", 1)[1])
        # Public technical identifiers only; never export traces or user data.
        tasks.append({
            "id": f"convoy-agent-{len(tasks) + 1:03d}",
            "title": title,
            "base_revision": base,
            "reference_revision": fix,
            "status": "candidate-uncalibrated",
            "prompt": f"Fix this Convoy behavior: {title}. Work from the supplied checkout. "
                      "Preserve unrelated behavior and public compatibility. Add your own regression tests. "
                      "The acceptance scenarios listed below must work without weakening validation or authorization. "
                      "Explain the change and report only checks actually executed.",
            "acceptance_scenarios": [n for t in tests for n in t["names"]],
            "oracle_tests": tests,
            "allowed_paths": [p for p in files if not p.endswith("_test.go")],
            "prerequisites": "Inspect historical CI and test setup; use disposable services. E2E cases require Docker. Never use production data.",
        })
        if len(tasks) == 60:
            break
    if len(tasks) != 60:
        raise SystemExit(f"Expected 60 tasks, got {len(tasks)}")
    (ROOT / "evals/tasks.json").write_text(json.dumps({
        "kind": "historical-coding-agent-task-candidates",
        "created_on": "2026-10-20", "tasks": tasks,
    }, indent=2) + "\n")
    print(f"Generated {len(tasks)} historical coding task candidates")


if __name__ == "__main__":
    main()
