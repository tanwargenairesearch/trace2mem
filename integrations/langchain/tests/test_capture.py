import json
from pathlib import Path
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from langchain_core.language_models.fake_chat_models import FakeListChatModel
from langchain_core.tools import tool
from trace2mem_langchain import MemoryCallback, MemoryClient


class CaptureTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.events = {}
        self.attempts = 0
        case = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                if self.path.endswith('AppendEvents'):
                    case.attempts += 1
                    for event in body['events']:
                        previous = case.events.setdefault(event['eventId'], event)
                        if previous != event:
                            self.send_error(409)
                            return
                    # The first call stores the events but loses the acknowledgment.
                    if case.attempts == 1:
                        self.send_error(503)
                        return
                    result = {'accepted': len(body['events'])}
                else:
                    result = {'content': '# Knowledge index', 'revision': 'r1'}
                self.send_response(200)
                self.end_headers()
                self.wfile.write(json.dumps(result).encode())

        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever)
        self.thread.start()
        self.url = f'http://127.0.0.1:{self.server.server_port}'
        self.path = Path(self.directory.name) / 'spool.sqlite'

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()
        self.directory.cleanup()

    def test_callbacks_retry_and_tool_pairing(self):
        client = MemoryClient(self.url, 'user-a', self.path).start()
        try:
            callback = MemoryCallback(client, 'conversation1', 'agent1')
            model = FakeListChatModel(responses=['Noted.'])
            model.invoke('Launch: October', config={'callbacks': [callback]})

            @tool
            def echo(text: str) -> str:
                '''Return the given text.'''
                return text

            echo.invoke({'text': 'observed'}, config={'callbacks': [callback]})
            callback.finish()
            client.flush(10)
            self.assertEqual(0, client.pending())
            events = list(self.events.values())
            calls = [e['toolCall']['callId'] for e in events if 'toolCall' in e]
            results = [e['toolResult']['callId'] for e in events if 'toolResult' in e]
            self.assertEqual(calls, results)
            self.assertEqual(1, len(calls))
            self.assertTrue(any(e.get('message', {}).get('text') == 'Launch: October' for e in events))
            self.assertEqual('r1', client.context()['revision'])
            self.assertGreaterEqual(self.attempts, 2)
        finally:
            client.close()

    def test_restart_and_credential_isolation(self):
        client = MemoryClient(self.url, 'user-a', self.path)
        event = {'eventId': 'persistent', 'message': {'text': 'retained'}}
        client.enqueue(event)
        client.close()
        with self.assertRaises(ValueError):
            MemoryClient(self.url, 'user-b', self.path)
        client = MemoryClient(self.url, 'user-a', self.path).start()
        try:
            client.flush(10)
            self.assertEqual(event, self.events['persistent'])
        finally:
            client.close()

    def test_capacity_fails_explicitly(self):
        client = MemoryClient(self.url, 'user-a', self.path, max_bytes=32)
        try:
            with self.assertRaises(BufferError):
                client.enqueue({'eventId': 'large', 'message': {'text': 'x' * 40}})
            self.assertEqual(0, client.pending())
        finally:
            client.close()

    def test_retry_budget_retains_events(self):
        client = MemoryClient(self.url, 'user-a', self.path, max_failures=1).start()
        client.enqueue({'eventId': 'retry', 'message': {'text': 'retained'}})
        try:
            with self.assertRaisesRegex(RuntimeError, 'retry budget exhausted'):
                client.flush(5)
            self.assertEqual(1, client.pending())
        finally:
            client.close()
        client = MemoryClient(self.url, 'user-a', self.path).start()
        try:
            client.flush(5)
            self.assertEqual(0, client.pending())
        finally:
            client.close()

    def test_close_serializes_with_enqueue(self):
        client = MemoryClient(self.url, 'user-a', self.path)
        accepted, errors = [], []
        started = threading.Event()

        def capture():
            started.set()
            for i in range(100):
                try:
                    client.enqueue({'eventId': str(i), 'message': {'text': 'persisted'}})
                    accepted.append(i)
                except RuntimeError:
                    return
                except Exception as error:
                    errors.append(error)
                    return

        producer = threading.Thread(target=capture)
        producer.start()
        started.wait()
        client.close()
        producer.join()
        self.assertEqual([], errors)
        reopened = MemoryClient(self.url, 'user-a', self.path)
        try:
            self.assertEqual(len(accepted), reopened.pending())
        finally:
            reopened.close()


if __name__ == '__main__':
    unittest.main()
