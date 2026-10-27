// greet greets a person by name, and the world when no name is given.
function greet(name = 'world') {
  return 'Hello, ' + name + '!';
}

module.exports = { greet };
