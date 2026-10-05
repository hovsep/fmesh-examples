# Rule Engine

Most software ends up with a rule engine somewhere: mail filters, tax classification, validation, alerting. This one keeps its rules in a config file and **compiles them into a mesh**: one component per condition, one per rule, one per action. Change `rules.json` and the next run builds a different mesh, with no code changes. The graph below was not drawn; it is the rule set.

The scenario is a mail client's filters. Conditions test one field of an email (`subject contains invoice`), rules combine conditions with `all` or `any`, and each rule names the actions to take.

```
inbox ─┬─▶ if:<condition> ─┬─▶ rule:<name> (all / any) ─┬─▶ do:<action> ─▶ outbox
       └─▶ ...             └─▶ ...                      └─▶ ...
```

```
#1  ceo@acme.example           "urgent: board deck"      → flag, notify-phone
#2  billing@supplies.example   "invoice #2231"           → forward-to-accountant, label-vendor, move-to-finance
#3  news@weekly.example        "This week in Go"         → move-to-later
#4  ceo@acme.example           "lunch on friday?"        · no rule matched
...
```

## How it works

- **The config is data; the mesh is built from it.** `LoadConfig` reads and validates `rules.json` (unknown ops, typos in condition names, duplicate names are all reported together), then `getMesh` turns every entry into a component and every reference into a pipe.
- **An email is a signal, its fields are metadata.** The payload is the email id; `from`, `subject`, `attachment` and `size_kb` ride as signal metadata (`WithMetaMany`, `WithMeta`), which is what conditions read.
- **A condition is a filter.** `if:<name>` forwards only the emails that satisfy it, with `port.ForwardWithFilter`.
- **Conditions are shared.** `invoice` is used by two rules, but it is one component, evaluated once per email and piped to both. This is the core trick of the Rete algorithm that production rule engines use; in fmesh it is just fan-out.
- **A rule has one input port per condition**, named after it. `all` keeps the emails that arrived on every port, `any` those that arrived on at least one. Every condition is one hop from the inbox, so a rule's inputs all arrive in the same cycle and an empty port simply means "no email passed this condition".
- **Actions fan in.** `notify-phone` is asked for by two rules; email #1 matches both, and the action is still taken once.
- The whole inbox goes through in one run: all conditions evaluate concurrently in one cycle, all rules in the next.

![Mesh graph](./rule%20engine-graph.svg)

## Run

```bash
go run .

# Your own rules or emails
go run . -rules my-rules.json -inbox my-inbox.json

# Regenerate the graph files (rule engine-graph.dot / .svg)
FMESH_GRAPH=1 go run .
```

Operators: `equals`, `contains`, `starts_with`, `ends_with` (strings, case-insensitive), `greater_than`, `less_than` (numbers).
