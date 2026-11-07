import json
import os
from pathlib import Path
import re
import subprocess
import sys

fixture = Path(__file__).resolve().parent.parent
controls = json.loads((fixture / 'controls.json').read_text())
program, args = sys.argv[1], sys.argv[2:]
input_bytes = sys.stdin.buffer.read()
user = input_bytes.decode()
if program == 'claude' and '-p' in args and args.index('-p') + 1 < len(args):
    user = args[args.index('-p') + 1]
whole = '\n'.join(args) + '\n' + user


def envelope(answer):
    print(json.dumps({'type': 'result', 'subtype': 'success', 'is_error': False,
                      'structured_output': answer, 'result': '', 'total_cost_usd': 0,
                      'usage': {'input_tokens': 0, 'output_tokens': 0}}))


def selected(concept, block):
    matches = [control for control in controls
               if control['concept'] == concept and control['block'] == block]
    if len(matches) != 1:
        raise RuntimeError('no unique planted control for ' + block + ':' + concept)
    return matches[0]


def forward():
    excluded = {str(fixture / 'cli')}
    for key in ['TRUE_BDD_AIPROXY_KNOWN_SHIMS', 'TRUE_BDD_AIPROXY_DIR']:
        excluded.update(str(Path(path).resolve()) for path in os.environ.get(key, '').split(os.pathsep) if path)
    for entry in os.environ.get('PATH', '').split(os.pathsep):
        if not entry or str(Path(entry).resolve()) in excluded:
            continue
        candidate = Path(entry).resolve() / program
        if candidate.is_file() and os.access(candidate, os.X_OK):
            completed = subprocess.run([str(candidate), *args], input=input_bytes, check=False)
            raise SystemExit(completed.returncode)
    raise RuntimeError('no downstream ' + program + ' outside fixture and proxy directories')


# The verification Q is never answered here, even when it mentions proposal text.
if program == 'claude' and 'Verification evidence for one Concept Item.' in user:
    forward()
elif program == 'claude' and 'Propose the step definition patterns of one concept for one suite,' in whole:
    payload = json.loads(user if user.strip() else args[-1])
    control = selected(payload['concept']['concept'], payload['block'])
    envelope({'patterns': [control['pattern']], 'cannot_fix': None})
elif program == 'claude' and user.startswith('# Propose a step definition body'):
    concept = re.search(r'^Concept: (.+)$', user, re.M)[1]
    block = re.search(r'^Block: (.+)$', user, re.M)[1]
    control = selected(concept, block)
    envelope({'fix_prompt': 'Write the preserved definition ' + control['definition']['function'] + '.',
              'questions': [], 'cannot_fix': None})
elif program == 'codex':
    match = re.search(r'^Concept: (.+) \((given|when|then)\)$', user, re.M)
    if not match:
        raise RuntimeError('writer received no selected Concept')
    control = selected(match[1], match[2])
    target = Path(control['definition']['file'])
    if not target.is_file():
        raise RuntimeError('the engine did not write the preserved pattern skeleton: ' + str(target))
    target.write_text(control['source'])
    result = re.search(r'^=== FILE_START: (.+) ===$', user, re.M)[1]
    answer = args[args.index('-o') + 1]
    Path(answer).write_text('=== FILE_START: ' + result + ' ===\napplied: true\ntarget: "' + str(target) +
                            '"\nsummary: "Planted the preserved definition."\n=== FILE_END: ' + result + ' ===\n')
    print('{"type":"turn.completed"}')
else:
    forward()
