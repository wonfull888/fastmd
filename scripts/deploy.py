#!/usr/bin/env python3
"""Build, rehearse, back up and restart only this fastmd installation."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import sqlite3
import subprocess
import time
import urllib.error
import urllib.request

BASE = Path('/www/wwwroot/fastmd')
if Path.cwd() != BASE or BASE.resolve() != BASE:
    raise SystemExit('Run inside /www/wwwroot/fastmd')


def scoped(name):
    path = BASE / name
    if path.is_symlink() or not path.resolve().is_relative_to(BASE):
        raise RuntimeError('Path leaves the project: ' + str(name))
    return path


args = argparse.ArgumentParser()
args.add_argument('--candidate', help='Already-built binary inside dist; skip pull/build')
options = args.parse_args()
os.umask(0o077)
scoped('data').mkdir(exist_ok=True)
lock = open(scoped('data/deploy.lock'), 'a')
fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
cache = scoped('data/build-cache')
for part in ('go', 'mod', 'build', 'tmp'):
    (cache / part).mkdir(parents=True, exist_ok=True)
environment = dict(os.environ, GOCACHE=str(cache/'build'), GOMODCACHE=str(cache/'mod'),
                   GOPATH=str(cache/'go'), GOTMPDIR=str(cache/'tmp'), TMPDIR=str(cache/'tmp'))
live = scoped('dist/fastmd-server')
database = scoped('data/fastmd.db')
key = scoped('data/fastmd.db.key')
pidfile = scoped('fastmd.pid')
for name in ('.git', '.env', 'fastmd.log', 'data/fastmd.db-wal', 'data/fastmd.db-shm'):
    scoped(name)

if options.candidate:
    candidate = scoped(options.candidate)
    if candidate.parent != scoped('dist') or candidate == live:
        raise SystemExit('Candidate must be a separate binary in dist')
else:
    subprocess.run(['git', 'pull', '--ff-only'], check=True, cwd=BASE)
    sha = subprocess.check_output(['git', 'rev-parse', '--short', 'HEAD'], cwd=BASE, text=True).strip()
    candidate = scoped('dist/fastmd-server.new')
    go = shutil.which('go') or '/usr/local/go/bin/go'
    subprocess.run([go, 'build', '-ldflags', '-X main.Version=v0.6.0-'+sha,
                    '-o', str(candidate), './cmd/server'], check=True, cwd=BASE, env=environment)
candidate.chmod(0o755)

pid = int(pidfile.read_text().strip())
command = subprocess.check_output(['ps', '-p', str(pid), '-o', 'command='], text=True).strip()
if not command.startswith(str(live) + ' '):
    raise SystemExit('PID file does not identify this project server; stopped safely')

backup = scoped('data/deploy-backups/' + time.strftime('%Y%m%d-%H%M%S'))
backup.mkdir(parents=True, exist_ok=False)
shutil.copy2(live, backup/'fastmd-server')


def snapshot(destination):
    with sqlite3.connect('file:'+str(database)+'?mode=ro', uri=True) as source:
        with sqlite3.connect(destination) as target:
            source.backup(target)
            if target.execute('PRAGMA quick_check').fetchone()[0] != 'ok':
                raise RuntimeError('Database integrity check failed')


snapshot(backup/'fastmd.db')
if key.exists():
    shutil.copy2(key, backup/'fastmd.db.key')
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def check(port, path):
    try:
        with opener.open('http://127.0.0.1:%s%s' % (port, path), timeout=3) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as error:
        return error.code, error.read()


def start(binary, port, db, log):
    output = open(log, 'ab', buffering=0)
    try:
        return subprocess.Popen([str(binary), '--port', str(port), '--db', str(db)], cwd=BASE,
                                env=environment, stdin=subprocess.DEVNULL, stdout=output,
                                stderr=subprocess.STDOUT, start_new_session=True)
    finally:
        output.close()


def ready(process, port):
    for _ in range(40):
        if process.poll() is not None:
            raise RuntimeError('Server exited during startup; see project log')
        try:
            status, body = check(port, '/v1/version')
            if status == 200:
                return json.loads(body)['version']
        except (OSError, ValueError):
            pass
        time.sleep(.25)
    raise RuntimeError('Server health check timed out')


def stop_child(process):
    if process.poll() is None:
        process.terminate()
        process.wait(timeout=10)


with socket.socket() as sock:
    sock.bind(('127.0.0.1', 0))
    trial_port = sock.getsockname()[1]
trial_db = backup/'trial.db'
snapshot(trial_db)
if key.exists():
    shutil.copy2(key, Path(str(trial_db)+'.key'))
trial = start(candidate, trial_port, trial_db, backup/'trial.log')
try:
    version = ready(trial, trial_port)
    for path, expected in (('/', 200), ('/login', 200), ('/mcp', 401), ('/v1/docs', 401)):
        if check(trial_port, path)[0] != expected:
            raise RuntimeError('Rehearsal failed: '+path)
    with sqlite3.connect(trial_db) as trial_conn, sqlite3.connect(backup/'fastmd.db') as before:
        if trial_conn.execute('SELECT COUNT(*) FROM documents').fetchone() != before.execute('SELECT COUNT(*) FROM documents').fetchone():
            raise RuntimeError('Migration changed document count')
finally:
    stop_child(trial)

snapshot(backup/'fastmd.db')
os.kill(pid, signal.SIGTERM)
for _ in range(80):
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        break
    time.sleep(.1)
else:
    raise RuntimeError('Existing server did not stop; binary left unchanged')

try:
    os.replace(candidate, live)
    process = start(live, 9000, database, scoped('fastmd.log'))
    pidfile.write_text(str(process.pid)+'\n')
    actual = ready(process, 9000)
    if actual != version:
        raise RuntimeError('Live version differs from rehearsal')
    for path, expected in (('/',200),('/login',200),('/mcp',401),('/v1/docs',401)):
        if check(9000,path)[0]!=expected:
            raise RuntimeError('Live smoke check failed: '+path)
except Exception:
    if 'process' in locals():
        stop_child(process)
    shutil.copy2(backup/'fastmd-server', live)
    restored = start(live, 9000, database, scoped('fastmd.log'))
    pidfile.write_text(str(restored.pid)+'\n')
    ready(restored, 9000)
    raise

print(json.dumps({'deployed': version, 'pid': process.pid, 'backup': str(backup)}))
