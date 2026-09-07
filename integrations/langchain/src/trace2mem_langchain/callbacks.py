"""LangChain callbacks capture inputs, generated messages and tool executions."""

from datetime import datetime, timezone
import json
from uuid import uuid4

from langchain_core.callbacks import BaseCallbackHandler


class MemoryCallback(BaseCallbackHandler):
    raise_error = True

    def __init__(self, client, session_id, agent_id):
        self.client, self.session_id, self.agent_id = client, session_id, agent_id

    def _event(self, role, payload, *, run_id=None, parent_run_id=None):
        event = {
            "eventId": str(uuid4()),
            "sessionId": self.session_id,
            "occurredAt": datetime.now(timezone.utc).isoformat(),
            "actor": {"role": role, "agentId": self.agent_id},
            "source": {"id": "langchain", "format": "langchain-callbacks", "version": "1"},
            "metadata": {"run_id": str(run_id or ""), "parent_run_id": str(parent_run_id or "")},
            **payload,
        }
        self.client.enqueue(event)

    def on_chat_model_start(self, serialized, messages, *, run_id, parent_run_id=None, **kwargs):
        # Each callback records the actual model input, including replayed context.
        # IDs identify these occurrences; retries reuse the durable queued envelope.
        roles = {"human": "user", "ai": "assistant", "system": "system", "tool": "tool"}
        for batch in messages:
            for message in batch:
                if message.type not in roles or not isinstance(message.content, str):
                    raise ValueError("unsupported message content; export multimodal content as an artifact")
                if message.content:
                    self._event(roles[message.type], {"message": {"text": message.content}}, run_id=run_id, parent_run_id=parent_run_id)

    def on_llm_end(self, response, *, run_id, parent_run_id=None, **kwargs):
        for batch in response.generations:
            for generation in batch:
                message = getattr(generation, "message", None)
                text = message.content if message is not None else generation.text
                if not isinstance(text, str):
                    raise ValueError("unsupported generated content; use artifact references")
                if text:
                    self._event("assistant", {"message": {"text": text}}, run_id=run_id, parent_run_id=parent_run_id)

    def on_tool_start(self, serialized, input_str, *, run_id, parent_run_id=None, **kwargs):
        try:
            arguments = json.loads(input_str)
        except (ValueError, TypeError):
            arguments = {"input": str(input_str)}
        self._event("assistant", {"toolCall": {"callId": str(run_id), "name": serialized.get("name", "tool"), "argumentsJson": json.dumps(arguments)}}, run_id=run_id, parent_run_id=parent_run_id)

    def on_tool_end(self, output, *, run_id, parent_run_id=None, **kwargs):
        text = getattr(output, "content", output)
        if not isinstance(text, str):
            text = json.dumps(text, default=str)
        self._event("tool", {"toolResult": {"callId": str(run_id), "text": text}}, run_id=run_id, parent_run_id=parent_run_id)

    def on_tool_error(self, error, *, run_id, parent_run_id=None, **kwargs):
        self._event("tool", {"toolResult": {"callId": str(run_id), "text": type(error).__name__, "failed": True}}, run_id=run_id, parent_run_id=parent_run_id)

    def on_llm_error(self, error, *, run_id, parent_run_id=None, **kwargs):
        self._event("system", {"message": {"text": "Generation failed: " + type(error).__name__}}, run_id=run_id, parent_run_id=parent_run_id)

    def finish(self):
        self._event("system", {"sessionLifecycle": {"state": "closed"}})
