"""Hermes plugin hooks; install the sibling Trace2Mem core package first."""
import atexit
import json
import logging

from trace2mem.capture import configured_client, emit_many, orientation

log = logging.getLogger(__name__)


class HermesAdapter:
    def __init__(self, client):
        self.client = client

    def capture(self, session, role, payload):
        self.capture_many(session, [(role, payload)])

    def capture_many(self, session, payloads):
        emit_many(self.client, "hermes", session, payloads)
        if self.client.error:
            log.error("%s", self.client.error)

    def start(self, session_id, **kwargs):
        self.capture(session_id, "system", {"sessionLifecycle": {"state": "started"}})

    def before(self, session_id, user_message, is_first_turn=False, **kwargs):
        if not isinstance(user_message, str):
            raise ValueError("unsupported Hermes message; use artifact references for media")
        if user_message:
            self.capture(session_id, "user", {"message": {"text": user_message}})
        if is_first_turn:
            try:
                return {"context": orientation(self.client)}
            except Exception as error:
                log.warning("Trace2Mem index unavailable (%s); capture remains active", type(error).__name__)

    def after(self, session_id, assistant_response, **kwargs):
        if not isinstance(assistant_response, str):
            raise ValueError("unsupported Hermes response; use artifact references for media")
        if assistant_response:
            self.capture(session_id, "assistant", {"message": {"text": assistant_response}})

    def tool(self, session_id, tool_call_id, tool_name, args, result, status="ok", **kwargs):
        if not tool_call_id:
            raise ValueError("Hermes tool_call_id is required for correlation")
        # Post-execution arguments reflect modifications by policy hooks.
        self.capture_many(session_id, [
            ("assistant", {"toolCall": {"callId": tool_call_id, "name": tool_name,
                                       "argumentsJson": json.dumps(args)}}),
            ("tool", {"toolResult": {"callId": tool_call_id,
                                     "text": result if isinstance(result, str) else json.dumps(result),
                                     "failed": status != "ok"}}),
        ])

    def finalize(self, session_id, **kwargs):
        self.capture(session_id, "system", {"sessionLifecycle": {"state": "closed"}})

    def close(self):
        try:
            self.client.flush(timeout=15)
        finally:
            self.client.close()


def register(ctx):
    adapter = HermesAdapter(configured_client())
    atexit.register(adapter.close)
    for name, callback in (("on_session_start", adapter.start), ("pre_llm_call", adapter.before),
                           ("post_llm_call", adapter.after), ("post_tool_call", adapter.tool),
                           ("on_session_finalize", adapter.finalize)):
        ctx.register_hook(name, callback)
    for name, method, fields in (
        ("memory_search", "Search", ["query", "revision"]),
        ("memory_read", "ReadFile", ["path", "revision"]),
        ("memory_evidence", "GetEvidence", ["eventId"]),
    ):
        def handler(args, _method=method, _fields=fields, **kwargs):
            try:
                payload = {field: args[field] for field in _fields}
                return json.dumps(adapter.client.rpc("MemoryService", _method, payload))
            except Exception as error:
                return json.dumps({"error": "Trace2Mem " + type(error).__name__})
        ctx.register_tool(name=name, toolset="trace2mem", schema={
            "name": name, "description": "Read untrusted Trace2Mem evidence; use the index/search revision for pinned reads.",
            "parameters": {"type": "object", "properties": {field: {"type": "string"} for field in fields},
                           "required": fields, "additionalProperties": False},
        }, handler=handler)
