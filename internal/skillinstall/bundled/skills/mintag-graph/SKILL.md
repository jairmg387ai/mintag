---
name: mintag-graph
description: "Trigger: knowledge graph, graph search, graph impact, blast radius, architecture dependencies, what depends on X, what does portal Y expose, repos implementing use case Z, graph_search, graph_node, graph_neighbors, graph_impact, graph_stats. Use Mintag graph tools to answer architecture questions."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

## Activation Contract

Use this skill when the user asks architecture questions that can be answered by the Mintag knowledge graph: dependency chains, blast-radius analysis, what a portal exposes, which repos implement a use case, or any "what depends on / what does X use" question.

## Hard Rules

- Always call graph_stats first when you don't know what the graph contains.
- Always call graph_search before graph_node or graph_impact to get exact node keys.
- Never invent node keys — resolve them from search results.
- For impact analysis, use graph_impact; do not manually traverse neighbors.
- The edge direction convention is SOURCE DEPENDS ON TARGET. "out" edges are dependencies of the node; "in" edges are its dependents.

## Decision Gates

| Situation | Action |
|---|---|
| User asks what depends on X | graph_impact on X |
| User asks what X depends on or uses | graph_node on X, look at "out" relations |
| User asks what a portal exposes | graph_neighbors with relation=exposes on the portal node |
| User asks which repos implement a use case | graph_neighbors with relation=implemented_by on the use_case node |
| Node key is approximate or unknown | graph_search first, then use the exact key returned |
| Graph contents are unknown | graph_stats first |

## Execution Steps

1. Call graph_stats to understand what the graph contains (if not already known).
2. Call graph_search with the entity name to find the exact node key and kind.
3. Call graph_node or graph_neighbors to get the requested context.
4. Call graph_impact when blast-radius or transitive dependents are needed.
5. Use kind_filter and limit in graph_impact to page through large result sets.
6. Report findings in terms of the user's question (not raw JSON).

## Output Contract

Answer the user's architecture question directly. Include relevant node keys, kinds, and relation types so the user can drill down with follow-up queries. When the result is large, summarize by kind counts first, then list specifics.
