// api/internal/repo/errors.go
package repo

import "errors"

// ErrNotFound is returned when a query expects a single row but finds none.
var ErrNotFound = errors.New("repo: not found")
