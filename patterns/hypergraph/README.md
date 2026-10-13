# Hypergraph

A [hypergraph](https://en.wikipedia.org/wiki/Hypergraph) is a graph whose edges join **many nodes to many nodes**. Here it is a chemical plant: 37 substances are the nodes and 24 reactions are the hyperedges, each joining all of its reactants to all of its products at once. Put raw materials on the shelf and the mesh works out everything the plant can make, and how.

```
Everything on the shelf:
  step 1: 4 fired → N2, O2, CO, H2, CaO, CO2, NaOH, Cl2
          waiting: Solvay tower (needs NH3, CO2); ...
  step 2: 6 fired → NH3, SO2, HCl, NaClO, CH3OH
          waiting: Solvay tower (needs NH3); ...
  step 3: 5 fired → NO, urea, SO3, HCHO, NaHCO3, NH4Cl
  ...
  step 6: 1 fired → NH4NO3
  → 34 of 37 substances in 6 steps. Cannot make: Ca3(PO4)2, Ca(H2PO4)2, CaSO4.

The natural gas supply is cut:
  ...
  How the plant gets NH3 (ammonia):
    NH3 ← Haber process
      N2 ← air separation
        air from the shelf
      H2 ← chlor-alkali
        NaCl from the shelf
        H2O from the shelf
```

## Why fmesh fits

A plain edge joins one node to one node, so it cannot say what a reaction says: *all of these, together, make all of those*. In fmesh a port already joins many pipes, so a hyperedge needs exactly two ports and no extra plumbing:

- **The mesh is the incidence graph.** One component per substance, one per reaction, one pipe per membership: 61 components, 91 pipes, all generated from [`reactions.txt`](./reactions.txt).
- **Many in: one input port.** Every reactant of a reaction pipes into its single `reactants` port. The Solvay tower (`NaCl + NH3 + CO2 + H2O → NaHCO3 + NH4Cl`) is fed by four pipes.
- **Many out: one output port.** A reaction fires one signal on `products`, and fmesh copies it down every pipe to every product.
- **"All together" is the activation function.** Reactants are made in different steps, so a reaction notes in its `State()` which have arrived and returns `component.ErrWaitDroppingInputs` until the last one is there. The run log shows the Solvay tower waiting for CO2 and ammonia, then firing.
- **Parallel by construction.** Every reaction whose reactants are ready fires in the same cycle; a synthesis step is two cycles (substances announce, reactions fire), with no goroutines or locks in the code.
- **Cycles in the data are fine.** The nitric acid tower makes NO, which feeds the reaction before it. A substance announces itself only the first time it is made, so news stops travelling and the run ends on its own.
- **What-if is a new shelf.** Cutting the natural gas loses methanol and formaldehyde, but ammonia is still made, with hydrogen from the chlor-alkali cells instead of steam reforming. Phosphate rock is never on the shelf, so the superphosphate plant never fires.

Coefficients are left out of the reaction file on purpose: the mesh answers whether something can be made, not how much.

![Mesh graph](./chemical%20plant-graph.svg)

## Run

```bash
go run .

# Load another reaction network
go run . -reactions my-reactions.txt

# Regenerate the graph files (chemical plant-graph.dot / .svg)
FMESH_GRAPH=1 go run .
```
