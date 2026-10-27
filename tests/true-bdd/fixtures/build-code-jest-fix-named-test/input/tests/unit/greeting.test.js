const { greet } = require('../../services/greeter/greeting');

// The target of the fix loop: it fails on the planted defect in greet()
// and passes once the production code greets the world by default.
describe('greet', () => {
  test('greets the world when no name is given', () => {
    expect(greet()).toBe('Hello, world!');
  });
});
