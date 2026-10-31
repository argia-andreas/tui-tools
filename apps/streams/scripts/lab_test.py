"""Safety contract tests; no Docker, broker or Amp process is started."""
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('streams_lab', Path(__file__).with_name('lab.py'))
lab = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lab)


class SafetyTests(unittest.TestCase):
    def test_private_state_idempotent_unknown_and_symlink_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory) / '.lab'
            with patch.object(lab, 'STATE', state):
                lab.owned(create=True)
                lab.owned(create=True)
                self.assertEqual(state.stat().st_mode & 0o777, 0o700)
                (state / 'owner').write_text('someone else')
                with self.assertRaises(RuntimeError):
                    lab.owned()
            link = Path(directory) / 'link'
            link.symlink_to(state)
            with patch.object(lab, 'STATE', link), self.assertRaises(RuntimeError):
                lab.owned()

    def test_remote_environment_rejected_before_docker_call(self):
        for key in ['DOCKER_HOST', 'DOCKER_CONTEXT']:
            with patch.dict(os.environ, {key: 'remote'}), patch.object(lab, 'run') as run:
                with self.assertRaises(RuntimeError):
                    lab.docker('stop', 'streams-lab-redpanda')
                run.assert_not_called()

    def test_active_remote_context_rejected(self):
        result = subprocess.CompletedProcess([], 0, stdout='[{"Endpoints":{"docker":{"Host":"ssh://remote"}}}]')
        with patch.dict(os.environ, {}, clear=True), patch.object(lab, 'run', return_value=result) as run:
            with self.assertRaises(RuntimeError):
                lab.docker('pull', 'anything')
            self.assertEqual(run.call_count, 1)

    def test_unknown_orb_service_not_stopped(self):
        result = subprocess.CompletedProcess([], 0, stdout='LoadState=loaded\nActiveState=active\nExecStart=/unknown/server\n')
        with patch.object(lab.subprocess, 'run', return_value=result), patch.object(lab, 'run') as run:
            with self.assertRaises(RuntimeError):
                lab.service('stop', 'streams-lab-redpanda')
            run.assert_not_called()

    def test_container_requires_owner_and_pin(self):
        for config in [{'Image':lab.PINS['redpanda'], 'Labels':{}},
                       {'Image':'unverified:latest', 'Labels':{'streams.lab.owner':str(lab.STATE)}}]:
            import json
            result = subprocess.CompletedProcess([], 0, stdout=json.dumps([{'Config':config}]))
            with patch.object(lab, 'docker', return_value=result), self.assertRaises(RuntimeError):
                lab.inspect('redpanda')


if __name__ == '__main__':
    unittest.main()
