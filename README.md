# go-ui-fs

Provides convenient handler to serve UI from embedded filesystem.

## Example

```go
//go:embed ui/dist
var ui embed.FS

// Responds with content from embedded `ui/dist`.
// Falls back to `index.html` by default, custom path can be provided with `uifs.WithFallbackPath`.
// Path prefix can be provided with `uifs.WithPrefix`.
http.Handle("/", uifs.Handler(ui))

// It's recommended to provide actual binary build time to improve caching.
// If not provided, `Handler` will use call time as build time, which will cause cache invalidation on binary restart.
http.Handle("/", uifs.Handler(ui, uifs.WithBuildTime(actualBuildTime)))
```
