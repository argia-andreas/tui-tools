#!/usr/bin/env python3
"""Explicit, private, loopback-only streams broker lab (never run by builds)."""
import argparse
import json
import os
from pathlib import Path
import secrets
import shlex
import socket
import stat
import subprocess
import sys
import time

APP = Path(__file__).resolve().parents[1]
STATE = APP / '.lab'
MARKER = 'streams-local-lab-v1\n'
SERVICE = 'streams-lab-docker'
SOCKET = STATE / 'docker.sock'
PINS = {
    'redpanda': 'docker.redpanda.com/redpandadata/redpanda@sha256:badca2d958f81da675c3055a9ecb6ebb590325582bcf16936148ffd65e96e920',
    'kafka': 'apache/kafka@sha256:852a0d49fc4e656b022e7f23785411802c9a088c78a7c5e39f33969de9c227a7',
}
PORTS = {'redpanda': [19092, 19644, 13345], 'kafka': [29092, 29093]}

def run(*args, **kw):
    return subprocess.run(args, check=True, text=True, **kw)

def service(action, name, command=None):
    info = subprocess.run(['systemctl', 'show', f'amp-svc-{name}.service',
                           '--property=LoadState,ActiveState,ExecStart'],
                          text=True, capture_output=True)
    fields = dict(line.split('=', 1) for line in info.stdout.splitlines() if '=' in line)
    if fields.get('LoadState') == 'loaded' and str(STATE) not in fields.get('ExecStart', ''):
        raise RuntimeError('refusing unknown existing orb service')
    active = fields.get('ActiveState') in ['active', 'activating']
    if (action == 'stop' and not active) or (action == 'start' and active):
        return
    args = ['amp', 'orb', 'service', action, name]
    if command:
        args += ['--command', command, '--cwd', str(APP)]
    run(*args)

def owned(create=False):
    if STATE.is_symlink():
        raise RuntimeError('refusing symlink lab state')
    if not STATE.exists():
        if not create:
            raise RuntimeError('run install first')
        STATE.mkdir(mode=0o700)
        (STATE / 'owner').write_text(MARKER)
        (STATE / '.gitignore').write_text('*\n')
    if (STATE.stat().st_uid != os.getuid() or
            stat.S_IMODE(STATE.stat().st_mode) != 0o700 or
            (STATE / 'owner').is_symlink() or
            (STATE / 'owner').read_text() != MARKER):
        raise RuntimeError('unknown or insecure lab ownership')
    if not (STATE / '.gitignore').exists():
        (STATE / '.gitignore').write_text('*\n')

def docker(*args, **kw):
    # An explicit Unix socket bypasses contexts and cannot contact a remote daemon.
    if os.environ.get('DOCKER_HOST') or os.environ.get('DOCKER_CONTEXT'):
        raise RuntimeError('unset DOCKER_HOST/DOCKER_CONTEXT for the isolated lab')
    context = json.loads(run('docker', 'context', 'inspect', capture_output=True).stdout)[0]
    if not context['Endpoints']['docker']['Host'].startswith('unix://'):
        raise RuntimeError('refusing active remote Docker context')
    if SOCKET.is_symlink():
        raise RuntimeError('refusing symlink daemon socket')
    if args[0] != 'info':
        root = run('sudo', '-n', 'docker', '--host', f'unix://{SOCKET}',
                   'info', '--format', '{{.DockerRootDir}}', capture_output=True).stdout.strip()
        if root != str(STATE / 'docker'):
            raise RuntimeError('private socket belongs to an unknown Docker daemon')
    return run('sudo', '-n', 'docker', '--host', f'unix://{SOCKET}', *args, **kw)

def daemon():
    owned()
    if not SOCKET.exists():
        cmd = (f'sudo -n dockerd --host unix://{SOCKET} --data-root {STATE}/docker '
               f'--exec-root {STATE}/exec --pidfile {STATE}/docker.pid '
               '--storage-driver vfs --bridge none --iptables=false --ip6tables=false '
               '--ip-forward=false --ip-masq=false')
        service('start', SERVICE, cmd)
    for _ in range(60):
        try:
            docker('info', stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            return
        except subprocess.CalledProcessError:
            time.sleep(1)
    raise RuntimeError('private Docker daemon failed; inspect amp orb service logs streams-lab-docker')

def private(path, value):
    if path.is_symlink():
        raise RuntimeError('refusing symlink private file')
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, 'w') as f:
        f.write(value)

def install(broker):
    daemon()
    docker('pull', PINS[broker])
    if not (STATE / 'ca.crt').exists():
        run('openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '365',
            '-subj', '/CN=streams-local-lab', '-keyout', str(STATE / 'ca.key'),
            '-out', str(STATE / 'ca.crt'), stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        run('openssl', 'req', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=localhost',
            '-keyout', str(STATE / 'server.key'), '-out', str(STATE / 'server.csr'),
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        private(STATE / 'extensions', 'subjectAltName=DNS:localhost,IP:127.0.0.1\n')
        run('openssl', 'x509', '-req', '-in', str(STATE / 'server.csr'), '-CA', str(STATE / 'ca.crt'),
            '-CAkey', str(STATE / 'ca.key'), '-CAcreateserial', '-days', '365', '-extfile',
            str(STATE / 'extensions'), '-out', str(STATE / 'server.crt'),
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        for path in STATE.glob('*.key'):
            path.chmod(0o600)
    # Viewer profiles require private referenced files, including trust roots.
    for path in STATE.glob('*.crt'):
        if path.is_symlink() or not path.is_file():
            raise RuntimeError('refusing unsafe lab certificate')
        path.chmod(0o600)
    for user in ['admin', 'viewer']:
        if not (STATE / f'{user}.password').exists():
            private(STATE / f'{user}.password', secrets.token_hex(24))
    if broker == 'redpanda':
        private(STATE / 'redpanda.yaml', '''redpanda:
  data_directory: /var/lib/redpanda/data
  node_id: 0
  seed_servers: []
  rpc_server: {address: 127.0.0.1, port: 13345}
  advertised_rpc_api: {address: 127.0.0.1, port: 13345}
  kafka_api:
    - {address: 127.0.0.1, port: 19092, authentication_method: sasl}
  advertised_kafka_api:
    - {address: 127.0.0.1, port: 19092}
  kafka_api_tls:
    - enabled: true
      cert_file: /lab/server.crt
      key_file: /lab/server.key
      truststore_file: /lab/ca.crt
  admin: [{address: 127.0.0.1, port: 19644}]
  developer_mode: true
  enable_sasl: true
  enable_metrics_reporter: false
  superusers: [lab-admin]
  auto_create_topics_enabled: false
  default_topic_replications: 1
  transaction_coordinator_replication: 1
  transaction_max_timeout_ms: 86400000
  id_allocator_replication: 1
  default_topic_partitions: 3
  log_segment_size_min: 1048576
''')
        private(STATE / 'redpanda.env', 'RP_BOOTSTRAP_USER=lab-admin:' +
                (STATE / 'admin.password').read_text() + '\n')
    else:
        private(STATE / 'kafka.env', '''KAFKA_NODE_ID=1
KAFKA_PROCESS_ROLES=broker,controller
KAFKA_LISTENERS=PLAINTEXT://127.0.0.1:29092,CONTROLLER://127.0.0.1:29093
KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://127.0.0.1:29092
KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER
KAFKA_CONTROLLER_QUORUM_VOTERS=1@127.0.0.1:29093
KAFKA_INTER_BROKER_LISTENER_NAME=PLAINTEXT
KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1
KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1
KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1
KAFKA_TRANSACTION_MAX_TIMEOUT_MS=86400000
KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS=0
KAFKA_AUTO_CREATE_TOPICS_ENABLE=false
KAFKA_LOG_DIRS=/var/lib/kafka/data
KAFKA_HEAP_OPTS=-Xms256m -Xmx512m
CLUSTER_ID=MkU3OEVBNTcwNTJENDM2Qk
''')
    configs = []
    if (STATE / 'redpanda.env').exists():
        for user in ['viewer', 'admin']:
            configs.append(dict(name=f'redpanda-{user}', brokers=['127.0.0.1:19092'], tls=True,
                                local_plaintext=False, ca_file='ca.crt', sasl='SCRAM-SHA-256',
                                username=f'lab-{user}', password_file=f'{user}.password'))
    if (STATE / 'kafka.env').exists():
        configs.append(dict(name='kafka-local', brokers=['127.0.0.1:29092'], tls=False,
                            local_plaintext=True))
    # Never put the administrator credential in the viewer's config.
    private(STATE / 'connections.json', json.dumps({'connections': [c for c in configs if c['name'] != 'redpanda-admin']}, indent=2))
    private(STATE / 'admin.json', json.dumps({'connections': configs}, indent=2))

def inspect(broker):
    result = docker('inspect', f'streams-lab-{broker}', capture_output=True)
    obj = json.loads(result.stdout)[0]
    if obj['Config'].get('Labels', {}).get('streams.lab.owner') != str(STATE):
        raise RuntimeError('refusing unowned container')
    if obj['Config']['Image'] != PINS[broker]:
        raise RuntimeError('container image differs from release pin')
    return obj

def up(broker):
    daemon()
    if not (STATE / f'{broker}.env').exists():
        raise RuntimeError('install selected broker first')
    names = docker('ps', '-a', '--format', '{{.Names}}', capture_output=True).stdout.splitlines()
    name = f'streams-lab-{broker}'
    if name in names:
        if inspect(broker)['State']['Running']:
            print(f'{broker}: already running')
            return
    for port in PORTS[broker]:
        with socket.socket() as s:
            # Do not mistake the owned broker's TIME_WAIT sockets for a live
            # listener after data-preserving down. No SO_REUSEPORT is used.
            s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            s.bind(('127.0.0.1', port))
    if name not in names:
        data = STATE / f'{broker}-data'
        if data.exists():
            raise RuntimeError('refusing existing data without its owned container; no automatic recovery/deletion')
        data.mkdir(mode=0o700)
        args = ['create', '--name', name, '--label', f'streams.lab.owner={STATE}',
                '--network', 'host', '--user', '0:0', '--env-file', str(STATE / f'{broker}.env'),
                '-v', f'{STATE}:/lab:ro', '-v', f'{data}:/var/lib/{broker}/data']
        if broker == 'redpanda':
            args += ['--entrypoint', '/usr/bin/redpanda', PINS[broker], '--redpanda-cfg',
                     '/lab/redpanda.yaml', '--smp', '1', '--memory', '768M', '--reserve-memory', '0M',
                     '--overprovisioned', '--unsafe-bypass-fsync', '1']
        else:
            args += [PINS[broker]]
        docker(*args, stdout=subprocess.DEVNULL)
    command = shlex.join(['sudo', '-n', 'docker', '--host', f'unix://{SOCKET}', 'start', '-a', name])
    service('start', name, command)
    for _ in range(90):
        with socket.socket() as s:
            if s.connect_ex(('127.0.0.1', PORTS[broker][0])) == 0:
                print(f'{broker}: listening on loopback {PORTS[broker][0]}')
                return
        time.sleep(1)
    raise RuntimeError('broker failed readiness; inspect orb service logs')

def seedcheck(broker, mode):
    inspect(broker)
    env = dict(os.environ, GOWORK='off')
    # A private alternate modfile keeps lab compilation independent of concurrent
    # app dependency work and never edits the application's go.mod/go.sum.
    modfile = STATE / 'tools.mod'
    if not modfile.exists():
        private(modfile, 'module streams-lab-tools\n\ngo 1.27.3\n\nrequire (\n'
                '\tgithub.com/twmb/franz-go v1.22.2\n'
                '\tgithub.com/twmb/franz-go/pkg/kadm v1.19.0\n)\n')
    tool = STATE / 'seed'
    run('go', 'build', '-mod=mod', '-modfile', str(modfile), '-o', str(tool),
        './lab/seed', cwd=APP, env=env)
    if mode == 'seed':
        if not (STATE / f'{broker}.seeded').exists():
            run(str(tool), mode, broker, str(STATE), cwd=APP, env=env)
        else:
            print('already seeded; preserving records and offsets')
        command = f'cd {shlex.quote(str(APP))} && ' + shlex.join([str(tool), 'open', broker, str(STATE)])
        service('start', f'streams-lab-{broker}-open', command)
    else:
        if mode == 'reset':
            service('stop', f'streams-lab-{broker}-open')
        run(str(tool), mode, broker, str(STATE), cwd=APP, env=env)

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('command', choices=['install', 'up', 'seed', 'check', 'down', 'reset'])
    p.add_argument('--broker', choices=list(PINS), default='redpanda')
    p.add_argument('--confirm-owned-reset', action='store_true')
    a = p.parse_args()
    owned(a.command == 'install')
    if a.command == 'install':
        install(a.broker)
    elif a.command == 'up':
        up(a.broker)
    elif a.command in ['seed', 'check', 'reset']:
        if a.command == 'reset' and not a.confirm_owned_reset:
            raise RuntimeError('reset requires --confirm-owned-reset; deletes only seeded topics/group')
        seedcheck(a.broker, a.command)
    elif a.command == 'down':
        inspect(a.broker)
        service('stop', f'streams-lab-{a.broker}-open')
        service('stop', f'streams-lab-{a.broker}')
        docker('stop', f'streams-lab-{a.broker}', stdout=subprocess.DEVNULL)
        print('stopped; all data and private daemon preserved')

if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, subprocess.CalledProcessError, OSError) as e:
        print(f'lab: {e}', file=sys.stderr)
        sys.exit(1)
