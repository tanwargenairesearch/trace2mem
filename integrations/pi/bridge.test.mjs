import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import http from "node:http";
import { Bridge } from "./bridge.mjs";
import extension from "./index.ts";

test("real Python bridge captures, retrieves, flushes, and releases spool on restart", async () => {
  const directory = await mkdtemp(join(tmpdir(), "trace2mem-pi-"));
  const events = [];
  const server = http.createServer(async (request, response) => {
    let body = "";
    for await (const chunk of request) body += chunk;
    const payload = JSON.parse(body);
    let result;
    if (request.url.endsWith("AppendEvents")) {
      events.push(...payload.events);
      result = { accepted: payload.events.length };
    } else result = { content: "# Index", revision: payload.revision || "r1" };
    response.end(JSON.stringify(result));
  });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  const old = { ...process.env };
  Object.assign(process.env, {
    TRACE2MEM_URL: `http://127.0.0.1:${server.address().port}`, TRACE2MEM_TOKEN: "test",
    TRACE2MEM_SPOOL: join(directory, "spool.sqlite"),
    PYTHONPATH: resolve(import.meta.dirname, "../core/src"),
  });
  let bridge;
  try {
    bridge = new Bridge();
    await bridge.request({ op: "message", session: "s", message: { role: "user", content: "hello", timestamp: 1700000000000 } });
    assert.match(await bridge.request({ op: "context" }), /r1/);
    await assert.rejects(bridge.request({ op: "retrieve", method: "Delete", payload: {} }), /ValueError/);
    await bridge.close();
    bridge = new Bridge();
    await bridge.request({ op: "lifecycle", session: "s", state: "closed" });
    await bridge.close();
    bridge = undefined;
    assert.equal(events.length, 2);
    assert.equal(events[0].message.text, "hello");
    assert.equal(events[1].sessionLifecycle.state, "closed");
    const hooks = {};
    extension({ on: (name, callback) => { hooks[name] = callback; }, registerTool: () => {} });
    const ctx = { sessionManager: { getSessionId: () => "second-session" }, ui: { notify: () => {} } };
    await hooks.session_start({}, ctx);
    const history = [{ role: "user", content: "hello", timestamp: 1 }];
    const first = await hooks.context({ messages: history });
    const second = await hooks.context({ messages: first.messages });
    assert.equal(history.length, 1);
    assert.equal(second.messages.length, 2);
    assert.deepEqual(first, second);
    await hooks.session_shutdown({ reason: "reload" }, ctx);
    assert.equal(events.filter(e => e.sessionLifecycle?.state === "closed").length, 1);
    await hooks.session_start({}, ctx);
    const resumed = await hooks.context({ messages: second.messages });
    assert.equal(resumed.messages.length, 2);
    await hooks.session_shutdown({ reason: "quit" }, ctx);
    assert.equal(events.filter(e => e.sessionLifecycle?.state === "closed").length, 2);
  } finally {
    if (bridge) await bridge.close();
    for (const key of Object.keys(process.env)) if (!(key in old)) delete process.env[key];
    Object.assign(process.env, old);
    await new Promise(resolve => server.close(resolve));
    await rm(directory, { recursive: true, force: true });
  }
});
