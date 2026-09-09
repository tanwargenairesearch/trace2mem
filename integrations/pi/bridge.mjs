import { spawn } from "node:child_process";
import { createInterface } from "node:readline";

export class Bridge {
  constructor() {
    this.pending = new Map();
    this.nextId = 0;
    this.failure = null;
    this.child = spawn(process.env.TRACE2MEM_PYTHON || "python3",
      ["-m", "trace2mem.bridge"], { stdio: ["pipe", "pipe", "inherit"] });
    const fail = () => {
      this.failure = new Error("Trace2Mem bridge stopped; inspect the retained spool");
      for (const { reject, timer } of this.pending.values()) {
        clearTimeout(timer);
        reject(this.failure);
      }
      this.pending.clear();
    };
    this.child.on("error", fail);
    this.child.on("exit", fail);
    this.child.stdin.on("error", fail);
    this.lines = createInterface({ input: this.child.stdout });
    this.lines.on("line", line => {
      let reply;
      try { reply = JSON.parse(line); } catch { fail(); this.child.kill(); return; }
      const waiter = this.pending.get(reply.id);
      if (!waiter) return;
      this.pending.delete(reply.id);
      clearTimeout(waiter.timer);
      if (reply.error) waiter.reject(new Error(`Trace2Mem: ${reply.error}; inspect capture logs/spool`));
      else waiter.resolve(reply.result);
    });
  }

  request(payload) {
    if (this.failure) return Promise.reject(this.failure);
    if (this.pending.size >= 64) return Promise.reject(new Error("Trace2Mem bridge is overloaded"));
    const id = ++this.nextId;
    const line = JSON.stringify({ ...payload, id }) + "\n";
    if (Buffer.byteLength(line) > 1 << 20) return Promise.reject(new Error("Trace2Mem request too large"));
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error("Trace2Mem bridge timeout; capture acceptance is unknown"));
        this.child.kill();
      }, 20000);
      this.pending.set(id, { resolve, reject, timer });
      this.child.stdin.write(line);
    });
  }

  async close() {
    try { await this.request({ op: "flush" }); }
    finally {
      await new Promise(resolve => {
        if (this.child.exitCode !== null || this.child.signalCode !== null) return resolve();
        const timer = setTimeout(() => { this.child.kill(); }, 15000);
        this.child.once("exit", () => { clearTimeout(timer); resolve(); });
        this.child.stdin.end();
      });
    }
  }
}
