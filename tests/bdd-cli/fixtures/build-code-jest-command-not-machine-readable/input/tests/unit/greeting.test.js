const { greet } = require('../../services/greeter/greeting');

// Fails against the planted defect in greet(), so a run that reported no
// failures could only mean the runner never read the results.
describe('greet', () => {
  test('greets the world when no name is given', () => {
    expect(greet()).toBe('Hello, world!');
  });
});
