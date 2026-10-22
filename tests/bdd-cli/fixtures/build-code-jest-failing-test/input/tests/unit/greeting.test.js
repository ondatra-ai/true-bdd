const { greet } = require('../../services/greeter/greeting');

// Two tests, one failing on the planted defect in greet(): the run counts
// one failure, not one per test.
describe('greet', () => {
  test('greets a person by name', () => {
    expect(greet('Ada')).toBe('Hello, Ada!');
  });

  test('greets the world when no name is given', () => {
    expect(greet()).toBe('Hello, world!');
  });
});
