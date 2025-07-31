# ⚙️ Raft Consensus Implementation in Go

This project is a ground-up implementation of the **Raft consensus algorithm** using Go. It explores the mechanics of distributed agreement across unreliable networks by building a fully functional Raft peer capable of leader election, log replication, fault recovery, and cluster synchronization.

---

## 🚀 Core Objectives

- Build distributed peers capable of reaching consensus despite crashes or network partitions
- Demonstrate key principles of Raft: **safety**, **leader election**, and **log consistency**
- Simulate real-world network behavior using unreliable communication and fault injection

---

## 🧠 Features Implemented

### 📡 Remote Communication
- Custom RPC library written in Go for peer communication

### 🗳️ Leader Election
- Randomized election timeouts
- Term-based candidate voting
- Heartbeat mechanism using empty `AppendEntries` RPCs

### 📝 Log Replication
- Leader appends log entries via `AppendEntries` RPCs
- Consistency checks based on term and index
- State transitions between **Follower → Candidate → Leader**

### 🔁 Crash & Recovery Simulation
- Controlled deactivation/reactivation of nodes for fault modeling

### ⚔️ Concurrency Management
- Coordinated goroutines
- Channel-based signaling for timeouts, RPCs, and state transitions

---

## ✅ Safety Guarantees

- Only one leader elected per term
- Consistent logs across non-faulty nodes
- Committed entries are never lost or reordered
- All peers eventually apply the same log entries in order

---

## 🧪 Testing & Validation

- Automated tests simulate:
    - Unreliable network conditions
    - Message delays and drops
    - Randomized node crashes

✔️ Successfully passed all correctness evaluations under failure conditions

---

## 🔭 Upcoming Enhancements

| Feature               | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| **Raft Visualization**| Interactive tool for consensus flow, aiding debugging and education        |
| **Cluster Changes**   | Support for dynamic node addition/removal without disrupting consensus      |
| **Log Compaction**    | Snapshotting and log reduction for better performance and scalability       |

---

## 👨‍💻 Language & Tools

- **Go**: Goroutines and native concurrency support for network systems
- **Custom RPC**: Lightweight remote messaging layer tailored to Raft

# 📦 Remote Objects Library (Go)

A lightweight remote method invocation framework inspired by Java RMI, designed for developers who need seamless interaction with remote objects over TCP. This Go-based library enables structured remote procedure calls, custom exception handling, and resilience testing — all in a compact and extensible package.

---

## 🎯 Overview

Remote Objects allows clients to invoke methods on server-hosted objects transparently. It uses:
- A **multithreaded Service** to expose remote objects
- A **StubFactory** to generate client-side proxies via reflection
- A **shared service interface** ensuring type-safe communication between client and server

---

## ⚙️ Core Components

| Component          | Role                                                                 |
|--------------------|----------------------------------------------------------------------|
| **Service**        | Manages client connections, method dispatching, and response encoding |
| **StubFactory**    | Generates proxy objects, handles serialization and error propagation |
| **LeakySocket**    | Simulates lossy/delayed network conditions for resilience testing     |

---

## 🧠 Key Technologies & Concepts

- **Language**: Go
- **Protocol**: TCP with synchronous request-reply messaging
- **Stub Generation**: Reflection-based proxy creation
- **Server Architecture**: Multi-threaded TCP server
- **Serialization**: Go’s `gob` encoding
- **Error Handling**: `RemoteObjectException` / `RemoteObjectError`
- **Resilience Testing**: Network fault injection via `LeakySocket`
- **Design Patterns**: Proxy, Service Interface, Multithreaded Server

---

## 🔁 Robustness Features

- Graceful service restart on failures
- Retransmission support for partial communication errors
- Custom exceptions distinguish protocol vs. application issues
- Dynamic method signature support via reflection

---

## 🧪 Testing Toolkit

- Simulated packet loss and delay
- Concurrent client handling with fault modeling
- Behavior validation under degraded network conditions

---

## 📌 Usage Example (Coming Soon)

```go
// Define your shared interface
type Calculator interface {
    Add(a int, b int) (int, error)
}

// Server-side: Register implementation
service.Register(&CalculatorImpl{})

// Client-side: Generate stub
calc := stubFactory.Create("Calculator").(Calculator)
result, err := calc.Add(3, 5)
