# Backend Development Guidelines

> Best practices for backend development in this project.

---

## Overview

This directory contains the active development guidelines for the Go CLI backend in this repository.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | Filled |
| [Database Guidelines](./database-guidelines.md) | SQLite patterns, queries, migrations | Filled |
| [Error Handling](./error-handling.md) | Error propagation and failure reporting | Filled |
| [Quality Guidelines](./quality-guidelines.md) | Code standards and review checks | Filled |
| [Logging Guidelines](./logging-guidelines.md) | CLI output and logging expectations | Filled |

---

## Pre-Development Checklist

Before editing backend code, read:

1. [Directory Structure](./directory-structure.md)
2. [Error Handling](./error-handling.md)
3. [Quality Guidelines](./quality-guidelines.md)
4. [Database Guidelines](./database-guidelines.md) if persistence is involved
5. [Logging Guidelines](./logging-guidelines.md) if command output or debug visibility changes

---

## Working Rule

Document and preserve what the repository actually does today: thin CLI entrypoint, package-owned responsibilities, digest-first validation, and no-CGO SQLite.

---

**Language**: All documentation should be written in **English**.
