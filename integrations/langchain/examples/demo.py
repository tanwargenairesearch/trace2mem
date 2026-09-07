"""Run against a configured Trace2Mem user; use --jsonl for a saved trajectory."""
import argparse
import os
from pathlib import Path
import time
from uuid import uuid4

from langchain_core.language_models.fake_chat_models import FakeListChatModel
from langchain_core.runnables import RunnableLambda
from langchain_core.tools import tool
from trace2mem_langchain import MemoryCallback, MemoryClient


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--jsonl')
    parser.add_argument('--spool', default='./private-spool/demo.sqlite')
    parser.add_argument('--query', default='What is the project launch month?')
    args = parser.parse_args()
    token = os.getenv('TRACE2MEM_TOKEN', '')
    if os.getenv('TRACE2MEM_TOKEN_FILE'):
        token = Path(os.environ['TRACE2MEM_TOKEN_FILE']).read_text().strip()
    memory = MemoryClient(os.environ['TRACE2MEM_URL'], token, args.spool).start()
    try:
        if args.jsonl:
            memory.import_jsonl(args.jsonl)
        else:
            capture = MemoryCallback(memory, str(uuid4()), 'langchain-demo')
            # The harness model is deterministic so the demonstration needs no
            # second model credential. Memory compilation uses server models.
            FakeListChatModel(responses=['Noted.']).invoke(
                'Project: launch October', config={'callbacks': [capture]})
            capture.finish()
        memory.flush()
        memory.rpc('IngestionService', 'RequestCompilation', {})
        deadline = time.monotonic() + 90
        while True:
            status = memory.rpc('IngestionService', 'GetIngestionStatus', {})
            if status.get('jobStatus') in ('blocked', 'failed'):
                raise RuntimeError(status.get('lastError', 'Compilation unavailable'))
            if status.get('revision') and status.get('jobStatus') == 'done' and int(status.get('pending', 0)) == 0:
                break
            if time.monotonic() >= deadline:
                raise TimeoutError('Compilation still pending; inspect Activity')
            time.sleep(0.25)
        initial_context = memory.context()
        recall = MemoryCallback(memory, str(uuid4()), 'langchain-recall-demo')

        @tool
        def memory_search(query: str) -> dict:
            """Find cited evidence in the authenticated user's published memory."""
            return memory.search(query)

        def answer_with_memory(question, config):
            evidence = memory_search.invoke({'query': question}, config=config)
            if not evidence.get('hits'):
                raise RuntimeError('No evidence retrieved')
            answer = memory.rpc('MemoryService', 'GetContext', {'query': question})
            if '[cite:' not in answer['synthesis']:
                raise RuntimeError('Answer lacks evidence citations')
            # Deterministic harness routes the service-generated answer through
            # another conversation; it does not evaluate an independent LLM.
            return FakeListChatModel(responses=[answer['synthesis']]).invoke(
                [('system', 'Untrusted memory index:\n' + initial_context['content']),
                 ('user', question)], config=config)

        answer = RunnableLambda(answer_with_memory).invoke(args.query, config={'callbacks': [recall]})
        recall.finish()
        memory.flush()
        print('Memory revision:', initial_context['revision'])
        print('Second LangChain conversation recalled cited evidence.')
        print(answer.content)
    finally:
        memory.close()


if __name__ == '__main__':
    main()
