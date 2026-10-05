# FPGA

An FPGA turns the usual model around. Instead of fixed hardware running your program, you get a board of generic, identical logic cells, describe a circuit in a hardware description language, and **the board becomes that circuit**. Nothing is executed instruction by instruction: the logic is the wiring.

Here the board is a mesh. Every logic cell, every pin and the clock is a component, and every wire is a pipe. A small Verilog-like file is synthesised at runtime into look-up tables, and the mesh is assembled from them. The same 16-cell board is flashed twice, first as a 4-bit adder, then as a 4-bit counter, and no Go code describes either circuit: they live in [`adder4.v`](./adder4.v) and [`counter4.v`](./counter4.v).

```
Flashing adder4: 8 of 16 cells used
  cell-00  s0     =  a0 ^ b0                        LUT 0110
  cell-01  c0     =  a0 & b0                        LUT 1000
  cell-02  s1     =  a1 ^ b1 ^ c0                   LUT 10010110
  ...
   5 +  7 = 12   (0101 + 0111 = 01100)
   9 +  9 = 18   (1001 + 1001 = 10010)

Flashing counter4: 5 of 16 cells used
  cell-00  q0     <= q0 ^ en                        LUT + flip-flop 0110
  ...
  clock │ 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 0 │ en=0 │ 1 1 1
```

## How it works

- **The HDL** ([`hdl.go`](./hdl.go)) is a strict subset of Verilog: `module ... endmodule`, `assign x = expr;` for combinational logic and `always @(posedge clk) q <= expr;` for registers, with `~ & ^ |` and parentheses.
- **Synthesis** turns each statement into one cell configuration: the signals it reads and its **truth table**, found by evaluating the expression for every input combination. That table is what a real FPGA cell (a LUT) is programmed with. A cell has four inputs; a wider expression is rejected with a hint to split it, exactly the constraint real synthesis works around.
- **Flashing** ([`board.go`](./board.go)) builds the mesh: the clock, a component per input and output pin, and the board's 16 cells. Used cells get their table (and a flip-flop if they are registers); the rest stay on the board, blank and unconnected, as you can see in the graph. Routing is one pipe per signal a cell reads.
- **One clock edge is one `Run`.** The clock drives the input pins and every register at once. Registers present the value they latched, combinational cells wait for all their inputs (`component.RequireInputs`) and pass on their table's answer, and when the mesh settles the outputs are stable. A register meanwhile computes, but keeps, what it will take on the next edge. That is the difference between combinational and sequential logic, and the counter shows it: it changes only on the clock, and turning `en` off takes effect one edge later.
- **Hardware rules come for free.** A combinational loop (`assign x = a & y; assign y = ~x;`) has no clock to break it, so its cells wait for each other forever, and fmesh's livelock detection stops the run and names them. A register reading its own output is fine: the clock breaks that loop.

![Mesh graph](./fpga%20counter4-graph.svg)

## Run

```bash
go run .

# Regenerate the graph of the board flashed with the counter (fpga counter4-graph.dot / .svg)
FMESH_GRAPH=1 go run .
```
