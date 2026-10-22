const fs = require('node:fs');
const path = require('node:path');

// Always passes, so the fix loop never targets it, and appends one line
// every time the WHOLE suite runs. After a fix, `build code` must re-run
// only the test it fixed, selected by name: a rerun that dropped the filter
// would run this test again and leave a second line behind.
describe('canary', () => {
  test('records each suite run', () => {
    fs.appendFileSync(path.join(__dirname, 'suite-runs.log'), 'suite ran\n');
  });
});
