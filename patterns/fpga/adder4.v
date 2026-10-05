// A 4-bit ripple-carry adder: s = a + b, cout is the carry out.
// Pure combinational logic: no clock, the outputs follow the inputs.
module adder4(input a0, a1, a2, a3, b0, b1, b2, b3, output s0, s1, s2, s3, cout);
  assign s0 = a0 ^ b0;
  assign c0 = a0 & b0;
  assign s1 = a1 ^ b1 ^ c0;
  assign c1 = (a1 & b1) | (c0 & (a1 ^ b1));
  assign s2 = a2 ^ b2 ^ c1;
  assign c2 = (a2 & b2) | (c1 & (a2 ^ b2));
  assign s3 = a3 ^ b3 ^ c2;
  assign cout = (a3 & b3) | (c2 & (a3 ^ b3));
endmodule
