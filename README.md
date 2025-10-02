# Local Agent

The **Local Agent** is responsible for handling the execution of test cases and test plans in local machine.

## Responsibilities

1. **Test Case Execution**: Execute individual test cases.
2. **Test Plan Execution**: Manages and executes comprehensive test plans across various environments.
3. **CRUD Operations for Results**: Handles Create, Read, Update, and Delete operations for storing and managing test execution results.

## Prerequisites

## Tech Stack

- **Go**: The primary language used to develop the service logic.
- **Playwright (JavaScript)**: Used for browser automation and end-to-end testing.

## Setup Guide

### Step 1: Clone the Repository

```bash
git clone git@op.gtl.apxor.com:external/autotest/agent.git
cd agent
```

### Step 2: Run service

```bash
go run cmd/agent/main.go
```
