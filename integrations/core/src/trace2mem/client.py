"""Durable, bounded capture queue; one credential and writer per spool."""

import fcntl
import hashlib
import json
import os
from pathlib import Path
import random
import sqlite3
import threading
import time
from urllib.error import HTTPError, URLError
from urllib.request import HTTPRedirectHandler, Request, build_opener


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError("Trace2Mem redirects are not permitted")


class MemoryClient:
    def __init__(self, url, token, spool, *, max_bytes=10 << 20, max_receipts=20000, max_failures=8):
        if not token or not url.startswith(("http://", "https://")):
            raise ValueError("Trace2Mem URL and per-user token are required")
        self.url, self._token = url.rstrip("/"), token
        self._opener = build_opener(NoRedirect())
        self._lock = threading.RLock()
        self._stop = threading.Event()
        self._wake = threading.Event()
        self.error = None
        self._thread = None
        self.max_bytes, self.max_receipts = max_bytes, max_receipts
        if min(max_bytes, max_receipts, max_failures) < 1:
            raise ValueError("capture limits must be positive")
        self.max_failures = max_failures
        path = Path(spool)
        path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        self._file = open(str(path) + ".lock", "a+b")
        os.chmod(str(path) + ".lock", 0o600)
        try:
            fcntl.flock(self._file, fcntl.LOCK_EX | fcntl.LOCK_NB)
            self._db = sqlite3.connect(path, check_same_thread=False)
            os.chmod(path, 0o600)
            self._db.executescript("""
                PRAGMA synchronous=FULL;
                CREATE TABLE IF NOT EXISTS owner (fingerprint TEXT PRIMARY KEY);
                CREATE TABLE IF NOT EXISTS events (id TEXT PRIMARY KEY, body TEXT NOT NULL, size INTEGER NOT NULL);
                CREATE TABLE IF NOT EXISTS receipts (id TEXT PRIMARY KEY, hash TEXT NOT NULL);
            """)
            fingerprint = hashlib.sha256((self.url + "\0" + token).encode()).hexdigest()
            existing = self._db.execute("SELECT fingerprint FROM owner").fetchone()
            if existing and existing[0] != fingerprint:
                raise ValueError("spool belongs to another credential or server; use a separate spool")
            self._db.execute("INSERT OR IGNORE INTO owner VALUES (?)", (fingerprint,))
            self._db.commit()
        except BaseException:
            if hasattr(self, "_db"):
                self._db.close()
            self._file.close()
            raise

    def rpc(self, service, method, payload):
        request = Request(
            f"{self.url}/trace2mem.v1.{service}/{method}",
            data=json.dumps(payload).encode(),
            headers={"Authorization": "Bearer " + self._token, "Content-Type": "application/json"},
        )
        with self._opener.open(request, timeout=10) as response:
            data = response.read((16 << 20) + 1)
        if len(data) > 16 << 20:
            raise RuntimeError("Trace2Mem response exceeds limit")
        return json.loads(data)

    def enqueue(self, event):
        self.enqueue_many([event])

    def enqueue_many(self, events):
        """Commit one callback's envelopes atomically, or retain none of them."""
        prepared = []
        for event in events:
            body = json.dumps(event, separators=(",", ":"), ensure_ascii=False, sort_keys=True)
            size = len(body.encode())
            if size > 60 << 10:
                raise ValueError("event too large; upload an artifact and capture its reference")
            prepared.append((event["eventId"], body, size))
        with self._lock:
            if self._stop.is_set():
                raise RuntimeError("capture client is closed")
            with self._db:
                used = self._db.execute("SELECT COALESCE(sum(size),0) FROM events").fetchone()[0]
                receipts = self._db.execute("SELECT count(*) FROM receipts").fetchone()[0]
                queued = self._db.execute("SELECT count(*) FROM events").fetchone()[0]
                for event_id, body, size in prepared:
                    previous = self._db.execute("SELECT body FROM events WHERE id=?", (event_id,)).fetchone()
                    if previous:
                        if previous[0] != body:
                            raise ValueError("conflicting local event ID")
                        continue
                    receipt = self._db.execute("SELECT hash FROM receipts WHERE id=?", (event_id,)).fetchone()
                    if receipt:
                        if receipt[0] != hashlib.sha256(body.encode()).hexdigest():
                            raise ValueError("conflicting delivered event ID")
                        continue
                    if used + size > self.max_bytes or receipts + queued >= self.max_receipts:
                        raise BufferError("capture spool limit reached; drain and archive it before starting a new spool")
                    self._db.execute("INSERT INTO events VALUES (?,?,?)", (event_id, body, size))
                    used += size
                    queued += 1
        self._wake.set()

    def pending(self):
        with self._lock:
            if self._db is None:
                raise RuntimeError("capture client is closed")
            return self._db.execute("SELECT count(*) FROM events").fetchone()[0]

    def _deliver(self):
        with self._lock:
            rows = self._db.execute("SELECT id,body FROM events ORDER BY rowid LIMIT 64").fetchall()
        if not rows:
            return False
        self.rpc("IngestionService", "AppendEvents", {"events": [json.loads(body) for _, body in rows]})
        with self._lock, self._db:
            self._db.executemany("INSERT OR IGNORE INTO receipts VALUES (?,?)", [(id_, hashlib.sha256(body.encode()).hexdigest()) for id_, body in rows])
            self._db.executemany("DELETE FROM events WHERE id=?", [(id_,) for id_, _ in rows])
        return True

    def _run(self):
        delay = 1
        failures = 0
        while not self._stop.is_set():
            try:
                if self._deliver():
                    delay, self.error = 1, None
                    failures = 0
                    continue
                self._wake.wait(1)
                self._wake.clear()
            except HTTPError as error:
                self.error = f"Trace2Mem delivery failed: HTTP {error.code}; events remain in the spool"
                error.close()
                if error.code != 429 and error.code < 500:
                    return
                failures += 1
                if failures >= self.max_failures:
                    self.error += "; retry budget exhausted; reopen the spool after resolving the failure"
                    return
                self._stop.wait(delay + random.random())
                delay = min(delay * 2, 30)
            except (URLError, TimeoutError, OSError):
                self.error = "Trace2Mem is unavailable; events remain in the spool"
                failures += 1
                if failures >= self.max_failures:
                    self.error += "; retry budget exhausted; reopen the spool after resolving the failure"
                    return
                self._stop.wait(delay + random.random())
                delay = min(delay * 2, 30)
            except Exception as error:
                self.error = f"Capture delivery stopped ({type(error).__name__}); inspect the server and retained spool"
                return

    def start(self):
        with self._lock:
            if self._thread is not None or self._stop.is_set():
                raise RuntimeError("capture client cannot be started twice")
            self._thread = threading.Thread(target=self._run, name="trace2mem-delivery", daemon=True)
            self._thread.start()
        return self

    def flush(self, timeout=30):
        deadline = time.monotonic() + timeout
        while self.pending():
            if self._thread is None or not self._thread.is_alive():
                raise RuntimeError(self.error or "start the delivery worker before flushing")
            if time.monotonic() >= deadline:
                raise TimeoutError(self.error or "capture flush timed out; queued events are retained")
            self._wake.set()
            time.sleep(0.05)

    def context(self, max_chars=8000):
        if not 1 <= max_chars <= 32000:
            raise ValueError("context limit must be between 1 and 32000 characters")
        result = self.rpc("MemoryService", "ReadFile", {"path": "knowledge/index.md"})
        content = result["content"]
        result["truncated"] = len(content) > max_chars
        result["content"] = content[:max_chars]
        return result

    def search(self, query):
        return self.rpc("MemoryService", "Search", {"query": query})

    def import_jsonl(self, path):
        with open(path, "rb") as source:
            while line := source.readline((60 << 10) + 1):
                if len(line) > 60 << 10:
                    raise ValueError("JSONL event exceeds adapter event limit")
                if line.strip():
                    self.enqueue(json.loads(line))

    def close(self):
        with self._lock:
            self._stop.set()
        self._wake.set()
        if self._thread:
            self._thread.join(12)
            if self._thread.is_alive():
                raise TimeoutError("delivery still stopping; spool remains open")
        with self._lock:
            if self._db is not None:
                self._db.close()
                self._db = None
            self._file.close()
