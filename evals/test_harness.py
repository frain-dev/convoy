"""Fast evaluator checks; these do not run an agent or historical Go tests."""
import json
import unittest

from harness import passed_events, prompt


class GradingTests(unittest.TestCase):
    def event(self, name, action):
        return json.dumps({"Test": name, "Action": action})

    def test_exact_test_pass_is_required(self):
        self.assertTrue(passed_events(self.event("TestFix", "pass"), ["TestFix"], 0))

    def test_skip_and_no_tests_do_not_pass(self):
        self.assertFalse(passed_events(self.event("TestFix", "skip"), ["TestFix"], 0))
        self.assertFalse(passed_events("no tests to run", ["TestFix"], 0))

    def test_package_failure_overrides_test_pass(self):
        self.assertFalse(passed_events(self.event("TestFix", "pass"), ["TestFix"], 1))

    def test_subtest_alone_does_not_pass_parent(self):
        self.assertFalse(passed_events(self.event("TestFix/one", "pass"), ["TestFix"], 0))

    def test_all_required_tests_must_pass(self):
        self.assertFalse(passed_events(self.event("TestFix", "pass"), ["TestFix", "TestOther"], 0))

    def test_prompt_does_not_export_reference_revision(self):
        task = {"prompt": "Repair a synthetic routing failure.",
                "reference_revision": "hidden-reference", "acceptance_scenarios": ["TestRouteFailure"]}
        value = prompt(task)
        self.assertNotIn("hidden-reference", value)
        self.assertIn("Route Failure", value)


if __name__ == "__main__":
    unittest.main()
