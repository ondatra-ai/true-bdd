// greet greets a person by name, and the world when no name is given.
// It does not do the second: the missing default is the planted defect,
// in PRODUCTION source, which is what the fix loop has to correct.
function greet(name) {
  return 'Hello, ' + name + '!';
}

module.exports = { greet };
