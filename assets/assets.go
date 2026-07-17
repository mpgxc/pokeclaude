// Package assets bundles the embedded sprite files into the binary. It lives
// next to the assets directory because go:embed cannot reference parent
// directories, so packages that need the sprites import this one.
package assets

import "embed"

// Sprites holds every ASCII sprite definition under sprites/.
//
//go:embed sprites/*.txt
var Sprites embed.FS
