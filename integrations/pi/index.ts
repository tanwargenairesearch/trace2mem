import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";
import { Bridge } from "./bridge.mjs";

export default function (pi: ExtensionAPI) {
  let bridge: Bridge | undefined;
  let context = "";
  const connection = () => bridge ??= new Bridge();
  pi.on("session_start", async (_event, ctx) => {
    await connection().request({ op: "lifecycle", session: ctx.sessionManager.getSessionId(), state: "started" });
    try { context = await connection().request({ op: "context" }); }
    catch { context = ""; ctx.ui.notify("Trace2Mem index unavailable; capture remains active", "warning"); }
  });
  pi.on("context", async event => {
    const messages = event.messages.filter(message =>
      !(message.role === "custom" && message.customType === "trace2mem-context"));
    if (context) messages.unshift({
      role: "custom", customType: "trace2mem-context", content: context, display: false, timestamp: 0,
    });
    return { messages };
  });
  pi.on("message_end", async (event, ctx) => {
    const status = await connection().request({ op: "message", session: ctx.sessionManager.getSessionId(), message: event.message });
    if (status.error) ctx.ui.notify(status.error, "warning");
  });
  pi.on("session_shutdown", async (event, ctx) => {
    if (!bridge) return;
    try {
      if (event.reason !== "reload") await bridge.request({
        op: "lifecycle", session: ctx.sessionManager.getSessionId(), state: "closed",
      });
    } finally {
      try { await bridge.close(); } finally { bridge = undefined; context = ""; }
    }
  });
  pi.registerTool({
    name: "memory_search", label: "Search memory", description: "Search published Trace2Mem evidence. Pass the revision from the initial index.",
    parameters: Type.Object({ query: Type.String(), revision: Type.String() }),
    async execute(_id, payload) {
      const result = await connection().request({ op: "retrieve", method: "Search", payload });
      return { content: [{ type: "text", text: JSON.stringify(result) }], details: result };
    },
  });
  pi.registerTool({
    name: "memory_read", label: "Read memory", description: "Read a memory path at the revision returned by the index or search.",
    parameters: Type.Object({ path: Type.String(), revision: Type.String() }),
    async execute(_id, payload) {
      const result = await connection().request({ op: "retrieve", method: "ReadFile", payload });
      return { content: [{ type: "text", text: JSON.stringify(result) }], details: result };
    },
  });
  pi.registerTool({
    name: "memory_evidence", label: "Read evidence", description: "Resolve an event citation from memory results.",
    parameters: Type.Object({ eventId: Type.String() }),
    async execute(_id, payload) {
      const result = await connection().request({ op: "retrieve", method: "GetEvidence", payload });
      return { content: [{ type: "text", text: JSON.stringify(result) }], details: result };
    },
  });
}
