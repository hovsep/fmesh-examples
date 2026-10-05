// A 4-bit binary counter: counts up on every clock edge while en is 1.
// Sequential logic: q0..q3 are registers, they change only on the clock.
module counter4(input en, output q0, q1, q2, q3);
  always @(posedge clk) q0 <= q0 ^ en;
  always @(posedge clk) q1 <= q1 ^ (en & q0);
  always @(posedge clk) q2 <= q2 ^ (en & q0 & q1);
  // A LUT has at most 4 inputs, so the carry into bit 3 gets its own cell.
  assign carry2 = en & q0 & q1 & q2;
  always @(posedge clk) q3 <= q3 ^ carry2;
endmodule
