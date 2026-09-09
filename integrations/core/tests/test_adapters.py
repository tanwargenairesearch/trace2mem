import importlib.util
import json
import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from trace2mem import MemoryClient
from trace2mem.capture import pi_message, orientation
from trace2mem.bridge import dispatch

spec = importlib.util.spec_from_file_location("hermes_plugin", Path(__file__).parents[2] / "hermes" / "__init__.py")
hermes = importlib.util.module_from_spec(spec)
spec.loader.exec_module(hermes)


class RecordingClient:
    error = None
    context = MemoryClient.context

    def __init__(self):
        self.events = []

    def enqueue_many(self, events):
        self.events.extend(json.loads(json.dumps(events)))

    def rpc(self, *args):
        raise OSError("unconfigured test transport")


class AdaptersTest(unittest.TestCase):
    def setUp(self):
        self.client = RecordingClient()

    def events(self):
        return self.client.events

    def test_pi_messages_tool_pair_and_timestamp(self):
        pi_message(self.client, "s1", {"role": "user", "content": "Hello", "timestamp": 1700000000000})
        pi_message(self.client, "s1", {"role": "assistant", "timestamp": 1700000000001, "content": [
            {"type": "thinking", "thinking": "private reasoning"},
            {"type": "text", "text": "Checking"},
            {"type": "toolCall", "id": "c1", "name": "read", "arguments": {"path": "a"}},
        ]})
        pi_message(self.client, "s1", {"role": "toolResult", "timestamp": 1700000000002,
                   "toolCallId": "c1", "isError": True, "content": [{"type": "text", "text": "missing"}]})
        events = self.events()
        self.assertEqual(4, len(events))
        self.assertEqual("2023-11-14T22:13:20+00:00", events[0]["occurredAt"])
        self.assertEqual("pi-" + hashlib.sha256(b"s1").hexdigest(), events[0]["sessionId"])
        self.assertEqual(events[2]["toolCall"]["callId"], events[3]["toolResult"]["callId"])
        self.assertTrue(events[3]["toolResult"]["failed"])
        self.assertNotIn("private reasoning", json.dumps(events))

    def test_pi_rejects_media_before_capture_and_ignores_context(self):
        with self.assertRaises(ValueError):
            pi_message(self.client, "s", {"role": "user", "timestamp": 1,
                       "content": [{"type": "text", "text": "a"}, {"type": "image", "data": "b"}]})
        pi_message(self.client, "s", {"role": "custom", "content": "retrieved memory"})
        self.assertEqual([], self.events())

    def test_hermes_turns_and_finalization(self):
        adapter = hermes.HermesAdapter(self.client)
        adapter.start(session_id="s")
        adapter.before(session_id="s", user_message="hello")
        adapter.tool(session_id="s", tool_call_id="c", tool_name="read", args={"path": "x"}, result="blocked", status="blocked")
        adapter.after(session_id="s", assistant_response="done")
        self.assertFalse(any(e.get("sessionLifecycle", {}).get("state") == "closed" for e in self.events()))
        adapter.finalize(session_id="s")
        events = self.events()
        self.assertEqual("closed", events[-1]["sessionLifecycle"]["state"])
        self.assertEqual("hermes-" + hashlib.sha256(b"s").hexdigest(), events[0]["sessionId"])
        self.assertEqual(events[2]["toolCall"]["callId"], events[3]["toolResult"]["callId"])
        self.assertTrue(events[3]["toolResult"]["failed"])
        with self.assertRaises(ValueError):
            adapter.tool(session_id="s", tool_call_id="", tool_name="read", args={}, result="bad")

    def test_framework_session_id_is_preserved_with_valid_wire_id(self):
        session = "native/session:" + "長" * 150
        hermes.HermesAdapter(self.client).start(session_id=session)
        event = self.events()[0]
        self.assertRegex(event["sessionId"], r"^[A-Za-z0-9_-]{1,128}$")
        self.assertEqual(session, event["metadata"]["framework_session_id"])

    def test_hermes_ok_status_is_success(self):
        hermes.HermesAdapter(self.client).tool(session_id="s", tool_call_id="c", tool_name="read",
                                             args={}, result="found", status="ok")
        self.assertFalse(self.events()[-1]["toolResult"]["failed"])

    def test_context_bounds_and_revision_retrieval(self):
        with patch.object(self.client, "rpc", return_value={"content": "x" * 5000, "revision": "r7"}) as rpc:
            text = orientation(self.client)
            self.assertIn('"truncated": true', text)
            self.assertIn('"revision": "r7"', text)
            self.assertLess(len(text), 4000)
            dispatch(self.client, {"op": "retrieve", "method": "Search", "payload": {"query": "x", "revision": "r7"}})
            rpc.assert_called_with("MemoryService", "Search", {"query": "x", "revision": "r7"})
        with self.assertRaises(ValueError):
            dispatch(self.client, {"op": "retrieve", "method": "Delete", "payload": {}})

    def test_hermes_registration_and_initial_context_failure(self):
        class Context:
            hooks, tools = {}, {}
            def register_hook(self, name, callback):
                self.hooks[name] = callback
            def register_tool(self, **tool):
                self.tools[tool["name"]] = tool
        ctx = Context()
        with patch.object(hermes, "configured_client", return_value=self.client), patch.object(hermes.atexit, "register"):
            hermes.register(ctx)
        self.assertNotIn("on_session_end", ctx.hooks)
        self.assertIn("on_session_finalize", ctx.hooks)
        with patch.object(self.client, "rpc", side_effect=OSError("offline")):
            with self.assertLogs(hermes.log, level="WARNING"):
                ctx.hooks["pre_llm_call"](session_id="s", user_message="kept", is_first_turn=True)
        self.assertEqual("kept", self.events()[0]["message"]["text"])
        with patch.object(self.client, "rpc", return_value={"revision": "r2"}) as rpc:
            response = ctx.tools["memory_read"]["handler"]({"path": "a.md", "revision": "r2"})
            self.assertEqual({"revision": "r2"}, json.loads(response))
            rpc.assert_called_with("MemoryService", "ReadFile", {"path": "a.md", "revision": "r2"})


class DurabilityTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.path = Path(self.directory.name) / "events.sqlite"
        self.client = MemoryClient("http://localhost:8787", "test-token", self.path)

    def tearDown(self):
        self.client.close()
        self.directory.cleanup()

    def test_restart_retries_exact_envelopes_after_lost_ack(self):
        pi_message(self.client, "s", {"role": "user", "content": "durable", "timestamp": 1700000000000})
        attempts = []
        def lose_ack(service, method, payload):
            attempts.append(payload)
            raise RuntimeError("lost acknowledgment")
        with patch.object(self.client, "rpc", side_effect=lose_ack):
            self.client.start()
            with self.assertRaises(RuntimeError):
                self.client.flush(timeout=2)
        self.client.close()
        with self.assertRaises(ValueError):
            MemoryClient("http://localhost:8787", "other-token", self.path)
        self.client = MemoryClient("http://localhost:8787", "test-token", self.path)
        with patch.object(self.client, "rpc", return_value={}) as rpc:
            self.client.start()
            self.client.flush(timeout=2)
            self.assertEqual(attempts[0], rpc.call_args.args[2])
        self.assertEqual(0, self.client.pending())

    def test_oversized_result_does_not_orphan_tool_call(self):
        with self.assertRaises(ValueError):
            hermes.HermesAdapter(self.client).tool(session_id="s", tool_call_id="c", tool_name="read",
                                                 args={}, result="x" * (65 << 10))
        self.assertEqual(0, self.client.pending())

    def test_capacity_rolls_back_whole_callback(self):
        self.client.max_receipts = 1
        with self.assertRaises(BufferError):
            pi_message(self.client, "s", {"role": "assistant", "timestamp": 1700000000000, "content": [
                {"type": "text", "text": "calling"},
                {"type": "toolCall", "id": "c", "name": "read", "arguments": {}},
            ]})
        self.assertEqual(0, self.client.pending())
        self.client.enqueue({"eventId": "valid", "message": {"text": "still usable"}})
        self.assertEqual(1, self.client.pending())

    def test_conflict_rolls_back_batch_and_preserves_prior_events(self):
        self.client.enqueue({"eventId": "old", "message": {"text": "kept"}})
        with self.assertRaises(ValueError):
            self.client.enqueue_many([
                {"eventId": "new", "message": {"text": "rolled back"}},
                {"eventId": "old", "message": {"text": "conflict"}},
            ])
        self.assertEqual(1, self.client.pending())
        with patch.object(self.client, "rpc", return_value={}) as rpc:
            self.client.start()
            self.client.flush(timeout=2)
            self.assertEqual([{"eventId": "old", "message": {"text": "kept"}}], rpc.call_args.args[2]["events"])


if __name__ == "__main__":
    unittest.main()
