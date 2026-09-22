# jeff

![my name is jeff](assets/my-name-is-jeff.gif)

Ask Jev a question from the terminal and get a number back

# TOC
- [About](#about)
- [Install](#install)
- [Usage](#usage)
- [Rank](#rank)
- [Exit codes](#exit-codes)
- [Using it from Go](#using-it-from-go)
- [What Jev is bad at](#what-jev-is-bad-at)
- [Todo](#todo)

# About

jeff is a cli for TypeSafe's Jev model written in Go with cobra. Jev doesn't write text, you give it some state and a question with a fixed set of answers and it gives back a calibrated probability for each one, so I wanted a way to use that from a shell script, a CI job or an agent without writing a client every time. The name is from the 22 Jump Street scene, and also because jev-cli was already taken on crates.io. It may not be perfect but it's better than nothing.

# Install

You need Go and a key from console.typesafe.ai/settings/keys

```bash
go install github.com/saembit/jeff-cli/cmd/jeff@latest
export TYPESAFE_API_KEY=...
```

or build it yourself

```bash
go build -o jeff ./cmd/jeff
```

# Usage

The state comes from --state, --state-file (- for stdin) or a pipe, and json objects and arrays are sent as structured state on their own, use --state-format text or json to force it. Every command takes -o json for machine output and --model to pin a versioned id such as jev-1.13.0, etc.

```bash
# yes/no, exit 10 when P(yes) is under the gate
git log -1 --pretty=%B | jeff noul "Does this commit describe a user facing change?" --fail-under 0.7

# pick one option, prints a probability per option and the confidence
jeff choice "Which team handles this?" --state-file ticket.txt \
  --option billing="payments, refunds" --option technical="bugs, outages" --option sales

# rate on ordered levels, prints a weighted score
jeff score "How frustrated is the customer?" --state-file ticket.txt \
  --level Calm --level Frustrated --level "Very angry"

# a file full of questions about one state
jeff eval -f examples/triage.yaml --state-file ticket.txt

# score N items on M weighted dimensions in one request
jeff rank examples/rank-ideas.yaml

# what your account can use
jeff models
```

# Rank

This is the reason I made the tool. You write a spec with the items, the dimensions to score them on with a weight and a rubric each, and some shared context, then jeff sends one score question per item per dimension in a single request and adds up weight times score for each item in code, so Jev never sees the weights and changing one is just a rerun of the arithmetic.

```yaml
context:
  builder: solo developer, wants revenue in months
items:
  zapier_node: Zapier / n8n node wrapping Jev
  sheets_addon: Sheets add-on exposing Jev as cell functions
dimensions:
  revenue:
    weight: 0.3
    question: How much could this earn per year within 18 months?
    levels: [none, under $10k, $10k-50k, $50k-200k, over $200k]
  effort:
    weight: 0.2
    question: How much work is a sellable v1?
    levels: [months with a team, one month solo, days solo]
overall: Which single item makes this builder money soonest?
```

The overall line is optional and adds one choice over every item. See examples/rank-ideas.yaml for a full spec.

# Exit codes

0 means it worked and every gate held, 1 is an error such as bad flags, no key, network, api, etc. and 10 means the request worked but --fail-under, --fail-over or --min-confidence didn't hold.

# Using it from Go

pkg/jev is a small client with no dependencies that retries 429, 529 and 5xx with backoff and honors Retry-After.

```go
c, err := jev.New()
resp, err := c.SystemOne(ctx, state, map[string]jev.Question{
    "urgent": jev.Noul("Does this convey urgency?", nil),
    "team":   jev.Choice("Which team?", map[string]any{"billing": nil, "technical": nil}),
    "mood":   jev.Score("How frustrated?", []any{"calm", "frustrated", "furious"}),
})
```

# What Jev is bad at

From TypeSafe's own page at docs.typesafe.ai/model-jaggedness/jev-1.13, it reads literally, can't count or do math, can't compare dates, gets worse with irrelevant state and can be steered by text in the state, so keep the math in code, filter the state first and treat the state as untrusted.

# Todo

- [ ] jeff calibrate, labeled outcomes in, a threshold plus Brier/ECE out
- [ ] jeff batch, ndjson in, one request per row under the rate limit, resumable
- [ ] jeff mcp, an MCP server over stdio so Claude Code and friends get jeff as a tool
- [ ] jeff auth, keyring storage instead of only the env var
