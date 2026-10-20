#!/usr/bin/env python3
"""Grade the original payload issue only; never calls a model or installs deps."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import signal
import subprocess
import tarfile
import tempfile

BASELINE = '001a0e3e0cb78548382e8e95a8e5a7a3a67c79d0'
DASHBOARD = Path('web/ui/dashboard')
ALLOWED = [
    'src/app/services/general/general.service.ts',
    *['src/app/private/components/prism/prism.component.' + ext for ext in ('ts', 'html', 'scss')],
    'src/app/private/components/prism/prism.module.ts',
    *['src/app/components/copy-button/copy-button.component.' + ext for ext in ('ts', 'html', 'scss')],
]
EXPECTED = {
    'Issue2886 independent oracle preserves parseable payload JSON',
    'Issue2886 independent oracle renders all JSON keys and escaped values',
    'Issue2886 independent oracle copies the entire collapsed payload through a real button',
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('candidate', type=Path, help='candidate checkout; only fixed allowed product files are copied')
    parser.add_argument('--node-modules', required=True, type=Path, help='preinstalled trusted baseline-compatible dashboard dependencies')
    parser.add_argument('--baseline', action='store_true', help='grade frozen baseline without candidate changes')
    parser.add_argument('--timeout', type=int, default=360)
    args = parser.parse_args()
    candidate = args.candidate.resolve()
    deps = args.node_modules.resolve()
    if not (deps / '@angular/cli/bin/ng.js').is_file():
        parser.error('trusted Angular CLI dependencies are required')
    here = Path(__file__).resolve().parent
    oracle = (here / 'payload.oracle.spec.ts').read_bytes()
    # Keep evidence on failure as well as success; evaluator removes it explicitly.
    target = Path(tempfile.mkdtemp(prefix='convoy-payload-eval-'))
    print('Evidence directory:', target, flush=True)
    archive = subprocess.check_output(['git', '-C', str(candidate), 'archive', BASELINE])
    with tarfile.open(fileobj=io.BytesIO(archive)) as source:
        source.extractall(target, filter='data')
    dashboard = target / DASHBOARD
    overlay_hashes = {}
    if not args.baseline:
        for relative in ALLOWED:
            source = candidate / DASHBOARD / relative
            if not source.is_file() or source.is_symlink():
                raise RuntimeError('missing or symlinked permitted implementation file: ' + relative)
            content = source.read_bytes()
            (dashboard / relative).write_bytes(content)
            overlay_hashes[relative] = hashlib.sha256(content).hexdigest()
    # Exclude ALL baseline/candidate tests; only the fixed evaluator oracle compiles.
    for spec in (dashboard / 'src').rglob('*.spec.ts'):
        spec.unlink()
    (dashboard / 'src/payload.oracle.spec.ts').write_bytes(oracle)
    (dashboard / 'node_modules').symlink_to(deps, target_is_directory=True)
    config = dashboard / 'karma.oracle.cjs'
    config.write_text("""
const fs = require('fs');
module.exports = config => {
  require('./karma.conf.js')(config);
  const results = [];
  function OracleReporter() {
    this.onSpecComplete = (_browser, result) => results.push({
      name: [...result.suite, result.description].join(' '),
      success: result.success, skipped: result.skipped, diagnostics: result.log
    });
    this.onRunComplete = () => fs.writeFileSync('oracle-results.json', JSON.stringify(results));
  }
  config.set({
    reporters: ['oracle'], plugins: [...config.plugins, {'reporter:oracle': ['type', OracleReporter]}],
    browsers: ['ChromeHeadless'], singleRun: true, autoWatch: false,
    browserNoActivityTimeout: 60000
  });
};
""")
    command = ['node', str(deps / '@angular/cli/bin/ng.js'), 'test', '--watch=false',
               '--karma-config=karma.oracle.cjs', '--include=src/payload.oracle.spec.ts']
    status = None
    timed_out = False
    with (target / 'run.log').open('wb') as log:
        process = subprocess.Popen(command, cwd=dashboard, stdout=log, stderr=subprocess.STDOUT,
                                   start_new_session=True)
        try:
            status = process.wait(timeout=args.timeout)
        except subprocess.TimeoutExpired:
            timed_out = True
            os.killpg(process.pid, signal.SIGKILL)
            status = process.wait()
    results_path = dashboard / 'oracle-results.json'
    results = json.loads(results_path.read_text()) if results_path.exists() else []
    passed = (status == 0 and not timed_out and len(results) == len(EXPECTED)
              and {item['name'] for item in results} == EXPECTED
              and all(item.get('success') is True and not item.get('skipped') for item in results))
    summary = dict(baseline=BASELINE, baseline_run=args.baseline,
                   oracle_sha256=hashlib.sha256(oracle).hexdigest(),
                   implementation_sha256=overlay_hashes, command=command,
                   exit_status=status, timed_out=timed_out, passed=passed, results=results)
    (target / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    print('PASS' if passed else 'FAIL (inspect run.log and summary.json; setup failures are not behavioral evidence)')
    return 0 if passed else 1


if __name__ == '__main__':
    raise SystemExit(main())
