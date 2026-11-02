#!/usr/bin/env python3
"""Prepare historical coding tasks and grade candidate patches independently."""
import argparse
from contextlib import contextmanager
import json
from pathlib import Path
import re
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def git(*args, cwd=ROOT, data=None):
    return subprocess.check_output(["git", *args], cwd=cwd, input=data, text=True)


def tasks():
    return json.loads((ROOT / "evals/tasks.json").read_text())["tasks"]


def check():
    catalog = tasks()
    assert len(catalog) >= 50
    assert len({t["id"] for t in catalog}) == len(catalog)
    for task in catalog:
        assert git("rev-parse", task["reference_revision"] + "^").strip() == task["base_revision"]
        assert task["oracle_tests"] and task["allowed_paths"]
        for oracle in task["oracle_tests"]:
            source = git("show", task["reference_revision"] + ":" + oracle["file"])
            before = git("diff", "--unified=0", task["base_revision"], task["reference_revision"], "--", oracle["file"])
            for name in oracle["names"]:
                assert re.search(r"^func " + re.escape(name) + r"\(", source, re.M)
                assert "+func " + name + "(" in before
            constraint = re.search(r"^//go:build (.+)$", source, re.M)
            assert oracle["tags"] == (constraint.group(1) if constraint else "")
        assert not any(p.endswith("_test.go") for p in task["allowed_paths"])
    print(f"Validated {len(catalog)} candidate definitions; no behavioral calibration or agent execution")


def prompt(task):
    return task["prompt"] + "\n\nAcceptance scenarios:\n" + "\n".join(
        "- " + re.sub(r"(?<=[a-z0-9])(?=[A-Z])", " ", n.removeprefix("Test")).replace("_", " ")
        for n in task["acceptance_scenarios"])


@contextmanager
def checkout(revision):
    with tempfile.TemporaryDirectory(prefix="convoy-agent-grade-") as directory:
        path = Path(directory) / "checkout"
        git("worktree", "add", "--detach", str(path), revision)
        try:
            yield path
        finally:
            git("worktree", "remove", "--force", str(path))


def passed_events(output, required, returncode):
    passed = set()
    for line in output.splitlines():
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if event.get("Action") == "pass":
            passed.add(event.get("Test"))
    return returncode == 0 and set(required) <= passed


def grade(task, candidate, reference=False, baseline=False):
    revision = task["reference_revision"] if reference else task["base_revision"]
    with checkout(revision) as path:
        if candidate:
            candidate = Path(candidate).resolve()
            assert git("rev-parse", "HEAD", cwd=candidate).strip() == task["base_revision"], "Candidate HEAD must remain at the task base; leave edits uncommitted"
            # Include untracked files in scope checks rather than silently ignoring them.
            untracked = git("ls-files", "--others", "--exclude-standard", cwd=candidate).splitlines()
            assert not untracked, "Stage new files (git add) before grading; do not commit"
            changed = git("diff", "HEAD", "--name-only", cwd=candidate).splitlines()
            implementation = [p for p in changed if not p.endswith("_test.go")]
            assert set(implementation) <= set(task["allowed_paths"]), "Implementation edits exceed frozen task scope"
            # Candidate-authored tests may be present but are never used as the oracle.
            patch = git("diff", "--binary", "HEAD", "--", *implementation, cwd=candidate) if implementation else ""
            if patch:
                git("apply", "--check", "-", cwd=path, data=patch)
                git("apply", "-", cwd=path, data=patch)
        if not reference:
            oracle_files = [o["file"] for o in task["oracle_tests"]]
            patch = git("diff", "--binary", task["base_revision"], task["reference_revision"], "--", *oracle_files)
            git("apply", "--check", "-", cwd=path, data=patch)
            git("apply", "-", cwd=path, data=patch)
        results = []
        for oracle in task["oracle_tests"]:
            command = ["go", "test", "-count=1", "-json", "-timeout=5m"]
            if oracle["tags"]:
                command.append("-tags=" + oracle["tags"])
            command += ["./" + str(Path(oracle["file"]).parent), "-run", "^(" + "|".join(oracle["names"]) + ")$"]
            try:
                result = subprocess.run(command, cwd=path, text=True, stdout=subprocess.PIPE,
                                        stderr=subprocess.STDOUT, timeout=360)
                print(result.stdout, flush=True)
                ok = passed_events(result.stdout, oracle["names"], result.returncode)
                # A test-level failure still needs review: it can be a setup error.
                has_failure_event = any('"Action":"fail"' in line and '"Test":' in line for line in result.stdout.splitlines())
                results.append({"file": oracle["file"], "passed": ok,
                                "test_failure_event": has_failure_event})
            except subprocess.TimeoutExpired:
                results.append({"file": oracle["file"], "passed": False,
                                "test_failure_event": False, "error": "timeout"})
        passed = all(r["passed"] for r in results)
        print(json.dumps({"task": task["id"], "mode": "reference" if reference else "baseline" if baseline else "candidate",
                          "passed": passed, "oracles": results}))
        return passed


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("check")
    sub.add_parser("list")
    for name in ("prepare", "grade", "reference", "baseline"):
        p = sub.add_parser(name)
        p.add_argument("task")
        if name in ("prepare", "grade"):
            p.add_argument("workspace", type=Path)
    args = parser.parse_args()
    if args.command == "check":
        check()
        return
    if args.command == "list":
        for task in tasks():
            print(task["id"], task["status"], task["title"])
        return
    task = next((t for t in tasks() if t["id"] == args.task), None)
    if task is None:
        parser.error("Unknown task ID")
    if args.command == "prepare":
        git("worktree", "add", "--detach", str(args.workspace.resolve()), task["base_revision"])
        print(prompt(task))
        print("\nAllowed implementation paths:\n" + "\n".join(task["allowed_paths"]))
        return
    ok = grade(task, args.workspace if args.command == "grade" else None,
               reference=args.command == "reference", baseline=args.command == "baseline")
    raise SystemExit(0 if ok else 1)


if __name__ == "__main__":
    main()
