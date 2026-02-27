module github.com/hovsep/fmesh-examples/life

go 1.26

require (
	github.com/guptarohit/asciigraph v0.7.3
	github.com/hovsep/fmesh v1.8.1-Tarsus
	github.com/hovsep/fmesh-examples v0.0.7
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/dot v1.9.2 // indirect
	github.com/hovsep/fmesh-graphviz v1.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/hovsep/fmesh-examples v0.0.7 => ./../
