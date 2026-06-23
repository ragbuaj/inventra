// Package apidocs embeds the OpenAPI 3.1 spec and serves interactive API docs.
//
// The Scalar viewer bundle is self-hosted (vendored in scalar.standalone.js) and
// served same-origin, avoiding any third-party CDN / supply-chain exposure.
package apidocs

import _ "embed"

// SpecYAML is the canonical OpenAPI 3.1 document (served at /openapi.yaml).
//
//go:embed openapi.yaml
var SpecYAML []byte

// ScalarJS is the vendored Scalar API-reference bundle (served at /docs/scalar.js).
//
//go:embed scalar.standalone.js
var ScalarJS []byte

// ScalarHTML renders the Scalar API reference, loading the spec and the
// self-hosted viewer bundle from the same origin.
const ScalarHTML = `<!doctype html>
<html>
  <head>
    <title>Inventra API</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script id="api-reference" data-url="/openapi.yaml"></script>
    <script src="/docs/scalar.js"></script>
  </body>
</html>`
