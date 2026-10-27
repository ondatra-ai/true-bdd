const fs = require('node:fs');
const path = require('node:path');

// Records discovery and the next walk's suite run. The isolated rerun
// must select only its failing test and leave this canary unexecuted.
describe('canary', () => {
  test('records each suite run', () => {
    fs.appendFileSync(path.join(__dirname, 'suite-runs.log'), 'suite ran\n');
  });
});
