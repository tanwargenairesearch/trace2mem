"""Private JSON-lines stdio bridge for the Pi extension."""
import json
import sys

from .capture import configured_client, emit, orientation, pi_message


def dispatch(client, request):
    op = request["op"]
    if op == "message":
        pi_message(client, request["session"], request["message"])
    elif op == "lifecycle":
        if request["state"] not in ("started", "closed"):
            raise ValueError("invalid lifecycle state")
        emit(client, "pi", request["session"], "system",
             {"sessionLifecycle": {"state": request["state"]}})
    elif op == "context":
        return orientation(client)
    elif op == "retrieve":
        method = request["method"]
        if method not in ("Search", "ReadFile", "GetEvidence"):
            raise ValueError("unsupported retrieval method")
        return client.rpc("MemoryService", method, request["payload"])
    elif op == "flush":
        client.flush(timeout=15)
    else:
        raise ValueError("unknown bridge operation")
    return {"pending": client.pending(), "error": client.error}


def main():
    client = configured_client()
    try:
        while line := sys.stdin.buffer.readline((1 << 20) + 1):
            if len(line) > 1 << 20:
                raise ValueError("bridge request exceeds limit")
            request = json.loads(line)
            try:
                response = {"id": request["id"], "result": dispatch(client, request)}
            except Exception as error:
                # Do not return HTTP bodies, credentials or raw trajectory content.
                response = {"id": request["id"], "error": type(error).__name__}
            print(json.dumps(response), flush=True)
    finally:
        client.close()


if __name__ == "__main__":
    main()
