"""Stdlib-only safety tests; these never start, reset or seed a broker."""
import copy
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
import subprocess
from unittest.mock import MagicMock, patch

import lab


class LabTests(unittest.TestCase):
    def test_mutations_require_confirmation_before_any_subprocess(self):
        with patch.object(lab, "run") as run:
            for command in ("down", "reset", "seed", "send", "dead-letter", "drain"):
                with self.subTest(command=command), self.assertRaisesRegex(ValueError, "confirm-disposable"):
                    lab.main([command])
            run.assert_not_called()

    def test_eula_not_inferred_from_environment(self):
        with patch.dict(os.environ, {"SBX_ACCEPT_EULA": "Y"}), patch.object(lab, "run") as run:
            for args in (["up"], ["--confirm-disposable", "reset"], ["--confirm-disposable", "seed"]):
                with self.subTest(args=args), self.assertRaisesRegex(ValueError, "accept-eula"):
                    lab.main(args)
            run.assert_not_called()

    def test_lifecycle_rejects_endpoint_flags(self):
        with patch.object(lab, "run") as run:
            for command in ("status", "up", "reset", "down"):
                with self.subTest(command=command), self.assertRaisesRegex(ValueError, "extra arguments"):
                    lab.main(["--accept-eula", "--confirm-disposable", command, "--endpoint", "cloud.invalid"])
            run.assert_not_called()

    def test_remote_and_compose_overrides_rejected_before_docker(self):
        args = type("Args", (), {"sudo": False})()
        with patch.object(lab, "run") as run:
            for key in ("DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH", "COMPOSE_FILE", "COMPOSE_PROJECT_NAME"):
                with self.subTest(key=key), patch.dict(os.environ, {key: "unsafe"}, clear=True):
                    with self.assertRaisesRegex(ValueError, "overrides"):
                        lab.local_docker(args)
            run.assert_not_called()
        result = type("Result", (), {"stdout": json.dumps([{"Endpoints": {"docker": {"Host": "tcp://cloud.invalid:2376"}}}])})()
        with patch.dict(os.environ, {}, clear=True), patch.object(lab, "run", return_value=result):
            with self.assertRaisesRegex(ValueError, "local Docker"):
                lab.local_docker(args)

    def test_container_guard_requires_project_image_and_exact_loopback_ports(self):
        container = {"Config": {"Image": lab.IMAGE, "Labels": {
            "com.docker.compose.project": lab.PROJECT, "com.docker.compose.service": "emulator"}},
            "HostConfig": {"PortBindings": {
                "5672/tcp": [{"HostIp": "127.0.0.1", "HostPort": "5673"}],
                "5300/tcp": [{"HostIp": "127.0.0.1", "HostPort": "5301"}]}}}
        lab.validate_container(container)
        cases = []
        altered = copy.deepcopy(container)
        altered["Config"]["Labels"]["com.docker.compose.project"] = "servicebus-explorer-spike"
        cases.append(altered)
        altered = copy.deepcopy(container)
        altered["Config"]["Image"] = "emulator:latest"
        cases.append(altered)
        for host, port in (("0.0.0.0", "5673"), ("127.0.0.1", "5672"), ("::", "5673")):
            altered = copy.deepcopy(container)
            altered["HostConfig"]["PortBindings"]["5672/tcp"] = [{"HostIp": host, "HostPort": port}]
            cases.append(altered)
        for altered in cases:
            with self.subTest(container=altered), self.assertRaises(ValueError):
                lab.validate_container(altered)

    def test_credentials_private_stable_and_outside_git(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(Path, "home", return_value=Path(tmp)):
            path = lab.credential_file()
            original = path.read_bytes()
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertEqual(path.parent.stat().st_mode & 0o777, 0o700)
            self.assertNotIn(str(lab.ROOT), str(path))
            self.assertIn(b"SBX_ACCEPT_EULA=Y", original)
            self.assertEqual(lab.credential_file().read_bytes(), original)
            path.unlink()
            path.symlink_to(Path(tmp) / "elsewhere")
            with self.assertRaisesRegex(ValueError, "symlink"):
                lab.credential_file()

    def test_config_is_exact_fixture_topology(self):
        scenario = json.loads((lab.ROOT / "internal/fixture/scenario.json").read_text())
        config = json.loads((lab.ROOT / "lab/config.json").read_text())["UserConfig"]
        self.assertEqual(len(config["Namespaces"]), 1)
        namespace = config["Namespaces"][0]
        self.assertEqual({q["Name"] for q in namespace["Queues"]}, {s["queue"] for s in scenario if "queue" in s})
        self.assertEqual(len(namespace["Queues"]), 12)
        self.assertEqual([t["Name"] for t in namespace["Topics"]], ["events"])
        self.assertEqual({s["Name"] for s in namespace["Topics"][0]["Subscriptions"]}, {"billing", "shipping"})

    def test_health_uses_fixed_endpoint_and_structured_healthy_status(self):
        for status in ("healthy", "unhealthy"):
            response = MagicMock()
            response.status = 200
            response.read.return_value = json.dumps({"status": status, "message": "Service is healthy."}).encode()
            opener = MagicMock()
            opener.open.return_value.__enter__.return_value = response
            with patch.object(lab.urllib.request, "build_opener", return_value=opener):
                self.assertEqual(lab.health(), status == "healthy")
                opener.open.assert_called_once_with("http://127.0.0.1:5301/health", timeout=2)

    def test_orb_up_uses_supervision_not_detach_and_has_no_secret_in_argv(self):
        args = type("Args", (), {"accept_eula": True, "sudo": True, "orb": True})()
        with patch.object(lab, "local_docker"), patch.object(lab, "compose", return_value=["sudo", "docker", "compose"]), patch.object(lab, "guarded"), patch.object(lab, "run") as run, patch("sys.stdout", new_callable=io.StringIO):
            lab.up(args)
            command = run.call_args.args[0]
            self.assertEqual(command[:5], ["amp", "orb", "service", "start", lab.SERVICE])
            self.assertNotIn("--detach", command[-1])
            self.assertNotIn("SBX_SQL_PASSWORD=", command[-1])
            self.assertIn("docker compose up", command[-1])

    def test_first_reset_down_skips_only_known_missing_service(self):
        args = type("Args", (), {"sudo": True, "orb": True})()
        for present in (False, True):
            listing = f"amp-svc-{lab.SERVICE}.service loaded active running lab\n" if present else "amp-svc-servicebus-lab.service loaded active running spike\n"
            result = subprocess.CompletedProcess([], 0, stdout=listing)
            with patch.object(lab, "local_docker"), patch.object(lab, "compose", return_value=["sudo", "docker", "compose"]), patch.object(lab, "run", return_value=result) as run:
                lab.down(args)
                stop = ["amp", "orb", "service", "stop", lab.SERVICE]
                self.assertEqual(any(call.args[0] == stop for call in run.call_args_list), present)
                self.assertEqual(run.call_args.args[0], ["sudo", "docker", "compose", "down", "--volumes", "--remove-orphans"])
        with patch.object(lab, "local_docker"), patch.object(lab, "run", side_effect=subprocess.CalledProcessError(1, ["amp"])) as run:
            with self.assertRaises(subprocess.CalledProcessError):
                lab.down(args)
            self.assertEqual(run.call_count, 1)  # inventory failure must not delete containers


if __name__ == "__main__":
    unittest.main()
