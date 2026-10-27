module.exports = {
  testEnvironment: 'node',
  transform: {},
  watchman: false,
  testMatch: ['<rootDir>/unit/**/*.test.js'],
  setupFiles: ['<rootDir>/stack-frames.js'],
};
