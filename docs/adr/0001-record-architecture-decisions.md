---
id: ADR-0001
title: "Record Architecture Decisions"
status: Accepted
date: "2026-09-23"
tags: [adr, architecture, governance]
---

# 1. Record Architecture Decisions

Date: 2026-09-23

## Status
Accepted

## Context
We need a standardized, visual, and human-in-the-loop way to record all architectural decisions made by the autonomous agent swarm, compatible with Obsidian Knowledge Graphs.

## Decision
We will use Architecture Decision Records (ADRs) stored in `docs/adr/` in Markdown format, cross-referenced using `[[wikilinks]]` with `architecture.md` and departmental reports.

## Consequences
- Every significant architectural pivot or design choice will produce an ADR.
- Obsidian Graph View will automatically visualize connections between decisions and codebase components.
