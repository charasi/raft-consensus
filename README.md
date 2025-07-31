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
