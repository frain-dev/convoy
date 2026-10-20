"""Regression checks for release and auto-approval boundaries."""

import copy
import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch
from urllib.error import HTTPError

SPEC = importlib.util.spec_from_file_location("publish_homebrew", Path(__file__).with_name("publish-homebrew.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class FormulaTests(unittest.TestCase):
    def setUp(self):
        self.release = {"tag_name": "v26.9.2", "draft": False, "prerelease": False, "assets": []}
        for platform in ("darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"):
            name = f"convoy_26.9.2_{platform}.tar.gz"
            self.release["assets"].append({
                "name": name, "digest": "sha256:" + "a" * 64, "size": 100,
                "state": "uploaded",
                "browser_download_url": f"https://github.com/frain-dev/convoy/releases/download/v26.9.2/{name}",
            })

    def test_formula_has_only_expected_platform_downloads(self):
        result = MODULE.formula("v26.9.2", self.release)
        self.assertEqual(result.count('      url "https://github.com/frain-dev/convoy/'), 4)
        self.assertEqual(result.count('bin.install "convoy"'), 4)
        self.assertNotIn("system", result)

    def test_rejects_unpublished_and_prerelease(self):
        for field in ("draft", "prerelease"):
            release = copy.deepcopy(self.release)
            release[field] = True
            with self.subTest(field=field), self.assertRaises(ValueError):
                MODULE.formula("v26.9.2", release)

    def test_rejects_malicious_or_nonstable_tag(self):
        for tag in ('v26.9.2-rc1', 'v26.9.2";system("curl")', '../../main'):
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                MODULE.formula(tag, self.release)

    def test_rejects_invalid_archive(self):
        for field, value in (("digest", ""), ("digest", "sha256:short"),
                             ("browser_download_url", "https://example.org/archive"),
                             ("state", "new"), ("size", 0)):
            release = copy.deepcopy(self.release)
            release["assets"][0][field] = value
            with self.subTest(field=field, value=value), self.assertRaises(ValueError):
                MODULE.formula("v26.9.2", release)


class ReviewTests(unittest.TestCase):
    def setUp(self):
        self.pr = {
            "state": "open", "draft": False, "user": {"login": MODULE.AUTHOR},
            "base": {"ref": "main", "sha": "base", "repo": {"full_name": MODULE.TAP}},
            "head": {"sha": "head", "repo": {"full_name": MODULE.TAP}}, "changed_files": 1,
        }
        self.files = [{"filename": "convoy.rb", "status": "modified"}]

    def test_expected_bot_formula_is_allowed(self):
        MODULE.validate_pr(self.pr, self.files, "head", "base")

    def test_changed_head_or_base_is_rejected(self):
        for head, base in (("new-head", "base"), ("head", "new-base")):
            with self.subTest(head=head, base=base), self.assertRaises(ValueError):
                MODULE.validate_pr(self.pr, self.files, head, base)

    def test_other_authors_forks_and_bases_are_rejected(self):
        for section, field, value in (("user", "login", "someone"),
                                      ("base", "ref", "other")):
            pr = copy.deepcopy(self.pr)
            pr[section][field] = value
            with self.subTest(section=section), self.assertRaises(ValueError):
                MODULE.validate_pr(pr, self.files, "head", "base")
        pr = copy.deepcopy(self.pr)
        pr["head"]["repo"]["full_name"] = "other/homebrew-tools"
        with self.assertRaises(ValueError):
            MODULE.validate_pr(pr, self.files, "head", "base")

    def test_extra_files_renames_and_nonformula_are_rejected(self):
        for files in ([], self.files * 2, [{"filename": "convoy.rb", "status": "renamed"}],
                      [{"filename": ".github/workflows/pwn.yml", "status": "modified"}]):
            with self.subTest(files=files), self.assertRaises(ValueError):
                MODULE.validate_pr(self.pr, files, "head", "base")


class ResumeTests(unittest.TestCase):
    def existing(self, head="head", author=MODULE.AUTHOR):
        return [HTTPError("https://api.github.com", 422, "exists", {}, None),
                {"object": {"sha": head}},
                {"author": {"login": author}, "parents": [{"sha": "base"}],
                 "files": [{"filename": "convoy.rb", "status": "modified"}]}]

    def test_fresh_branch_commits_once(self):
        with patch.object(MODULE, "api", side_effect=[{}, {"commit": {"sha": "head"}}]) as api:
            self.assertEqual(MODULE.release_branch("branch", "base", "blob", "formula", "v1.2.3", "token"), "head")
            self.assertEqual(api.call_count, 2)

    def test_resume_after_branch_creation(self):
        replies = self.existing(head="base")[:2] + [{"commit": {"sha": "head"}}]
        with patch.object(MODULE, "api", side_effect=replies):
            self.assertEqual(MODULE.release_branch("branch", "base", "blob", "formula", "v1.2.3", "token"), "head")

    def test_resume_after_formula_commit_does_not_rewrite(self):
        with patch.object(MODULE, "api", side_effect=self.existing()) as api, patch.object(MODULE, "content", return_value=("formula", "blob")):
            self.assertEqual(MODULE.release_branch("branch", "base", "blob", "formula", "v1.2.3", "token"), "head")
            self.assertEqual(api.call_count, 3)

    def test_foreign_branch_is_never_overwritten(self):
        with patch.object(MODULE, "api", side_effect=self.existing(author="someone")) as api:
            with self.assertRaises(ValueError):
                MODULE.release_branch("branch", "base", "blob", "formula", "v1.2.3", "token")
            self.assertEqual(api.call_count, 3)

    def test_altered_formula_is_never_resumed(self):
        with patch.object(MODULE, "api", side_effect=self.existing()), patch.object(MODULE, "content", return_value=("malicious", "blob")):
            with self.assertRaises(ValueError):
                MODULE.release_branch("branch", "base", "blob", "formula", "v1.2.3", "token")


if __name__ == "__main__":
    unittest.main()
