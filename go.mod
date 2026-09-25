module github.com/chinmay-sawant/spectrePS

go 1.26.4

// Image scaling uses golang.org/x/image/draw; TIFF output uses golang.org/x/image/tiff.
require golang.org/x/image v0.46.0

// JPEG2000 image streams decode through a pure-Go decoder; the module uses no cgo.
require github.com/mrjoshuak/go-jpeg2000 v1.5.12
