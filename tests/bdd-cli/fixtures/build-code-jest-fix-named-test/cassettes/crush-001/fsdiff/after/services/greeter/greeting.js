// greet greets a person by name, and the world when no name is given.
// An omitted name falls back to 'world' via the default parameter.
function greet(name = 'world') {
  return 'Hello, ' + name + '!';
}

module.exports = { greet };
