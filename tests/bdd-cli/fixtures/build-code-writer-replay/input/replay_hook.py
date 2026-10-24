"""Record the real crush shell/job inputs without changing their verdict."""
import json
from pathlib import Path
import sys
import time

payload = json.load(sys.stdin)
tool = payload["tool_name"]
if tool in ("bash", "job_output", "job_kill"):
    start = Path("tmp/replay-audit-start")
    if not start.exists():
        start.write_text(str(time.monotonic()))
    record = {
        "tool": tool,
        "input": payload["tool_input"],
        "elapsed_seconds": int(time.monotonic() - float(start.read_text())),
    }
    with Path("services/calc/guard-calls.jsonl").open("a") as evidence:
        evidence.write(
            json.dumps(record, sort_keys=True).replace(str(Path.cwd()), "<project root>") + "\n"
        )
