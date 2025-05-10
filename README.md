# FMesh examples

Examples of [FMesh](https://github.com/hovsep/fmesh) library usage.

FMesh is a FBP-inspired (flow based programming) framework written in Golang.
It allows you to express your program as a computational graph consisting of components connected by pipes ([learn more](https://github.com/hovsep/fmesh/wiki)).

The list of examples: 

- [Async input](https://github.com/hovsep/fmesh-examples/blob/main/async_input/main.go) ([play](https://go.dev/play/p/xEkPgS9a10X))
- [Electric circuit](https://github.com/hovsep/fmesh-examples/blob/main/electric_circuit/main.go)  ([play](https://go.dev/play/p/bibZTWhIbR8))
- [Fibonacci](https://github.com/hovsep/fmesh-examples/blob/main/fibonacci/main.go)  ([play](https://go.dev/play/p/VmLIh6tOsvo))
- [Filter](https://github.com/hovsep/fmesh-examples/blob/main/filter/main.go)  ([play](https://go.dev/play/p/NDBcOZ5f0E1))
- [Load balancer](https://github.com/hovsep/fmesh-examples/blob/main/load_balancer/main.go)  ([play](https://go.dev/play/p/s1ETIrgo7pp))
- [Nesting](https://github.com/hovsep/fmesh-examples/blob/main/nesting/main.go)  ([play](https://go.dev/play/p/GW1HNKZeMzR))
- [Pipeline](https://github.com/hovsep/fmesh-examples/blob/main/pipeline/main.go)  (can not be run in the Go playground as it uses local I/O)
- [String processing](https://github.com/hovsep/fmesh-examples/blob/main/string_processing/main.go)  ([play](https://go.dev/play/p/Yf_29d6vs68))


All examples are using the same [latest version of FMesh](https://github.com/hovsep/fmesh/releases/latest).

## Contributions

We welcome new example programs! 
Each example in this repo lives in its own folder and consists of a single main.go that demonstrates how to use fmesh in any domain you choose—biology, mechanics, game theory, simulation, economics, or beyond.

To contribute:

- Fork this repository.
- Create a new directory under examples/ (e.g. examples/my-domain-demo).
- Add your main.go inside that directory.
  - Follow the pattern of the existing examples.
  - Include a brief comment at the top explaining what scenario you’re modeling and how to run it.
- Optionally add an image with graph visualisation of your mesh.
- Open a pull request against main. We’ll review, suggest improvements if needed, and merge—so everyone can learn from your use case!

Thank you for helping grow the fmesh ecosystem!