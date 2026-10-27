// The Corner Shop's web front: one page, served on the port PORT names.
// The page shows the store name as its heading and links to opening hours.
const http = require('node:http');

const page = `<!doctype html>
<html>
  <head><title>Corner Shop</title></head>
  <body>
    <h1>Corner Shop</h1>
    <a href="/hours">Opening hours</a>
  </body>
</html>
`;

http
  .createServer((req, res) => {
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end(page);
  })
  .listen(Number(process.env.PORT), '127.0.0.1');
