package raft

import (
	"github.com/cmu14736/s23-lab1-go-distributed/remote"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 14-736 Lab 2 Raft implementation in go

const (
	FOLLOWER = iota + 1
	CANDIDATE
	LEADER
)

// StatusReport struct sent from Raft node to Controller in response to command and status requests.
// this is needed by the Controller, so do not change it. make sure you give it to the Controller
// when requested
type StatusReport struct {
	Index     int
	Term      int
	Leader    bool
	CallCount int
}

// RaftInterface -- this is the "service interface" that is implemented by each Raft peer using the
// remote library from Lab 1.  it supports five remote methods that you must define and implement.
// these methods are described as follows:
//
//  1. RequestVote -- this is one of the remote calls defined in the Raft paper, and it should be
//     supported as such.  you will need to include whatever argument types are needed per the Raft
//     algorithm, and you can package the return values however you like, as long as the last return
//     type is `remote.RemoteObjectError`, since that is required for the remote library use.
//
//  2. AppendEntries -- this is one of the remote calls defined in the Raft paper, and it should be
//     supported as such and defined in a similar manner to RequestVote above.
//
//  3. GetCommittedCmd -- this is a remote call that is used by the Controller in the test code. it
//     allows the Controller to check the value of a commmitted log entry at a given index. the
//     type of the function is given below, and it must be implemented as given, otherwise the test
//     code will not function correctly.  more detail about this method is available later in this
//     starter code file.
//
//  4. GetStatus -- this is a remote call that is used by the Controller to collect status information
//     about the Raft peer.  the struct type that it returns is defined above, and it must be implemented
//     as given, or the Controller and test code will not function correctly.  more detail below.
//
//  5. NewCommand -- this is a remote call that is used by the Controller to emulate submission of
//     a new command value by a Raft client.  upon receipt, it will initiate processing of the command
//     and reply back to the Controller with a StatusReport struct as defined above. it must be
//     implemented as given, or the test code will not function correctly.  more detail below
type RaftInterface struct {
	RequestVote     func(int, int, int, int) (bool, remote.RemoteObjectError)             // TODO: define function type
	AppendEntries   func(int, int, int, int, []Log, int) (bool, remote.RemoteObjectError) // TODO: define function type
	GetCommittedCmd func(int) (int, remote.RemoteObjectError)
	GetStatus       func() (StatusReport, remote.RemoteObjectError)
	NewCommand      func(int) (StatusReport, remote.RemoteObjectError)
}

// Log -- log entry; entry contains command
// for state machine, and term when entry
// was received by leader
type Log struct {
	Term int
	Cmd  int
}

// RaftPeerState
// you will need to define a struct that contains the parameters/variables that define and
// explain the current status of each Raft peer.  it doesn't matter what you call this struct,
// and the test code doesn't really care what state it contains, so this part is up to you.
// TODO: define a struct to maintain the local state of a single Raft peer
type RaftPeerState struct {
	// currentTerm: latest term server has seen
	currentTerm int
	// votedFor: candidateId that received vote in current
	// term (or null if none)
	votedFor int
	// log entries; each entry contains command
	// for state machine, and term when entry
	// was received by leader (first index is 1)
	logs []Log
	// port: this is the service port number where this Raft peer will listen for incoming messages
	port int
	// id: this is the ID (or index) of this Raft peer in the peer group, ranging from 0 to num-1
	id int
	// num: this is the number of Raft peers in the peer group (num > id)
	num int
	// peers: hold port numbers used by any other peers.
	peers map[int]string
	// serv: remote service for RPC to connect
	serv *remote.Service
	// addr: raft peer network address
	addr string
	// leader: indicates if raft peer is leader
	leader bool
	// state: current state of raft peer. FOLLOWER, CANDIDATE, LEADER, QUIT
	state int
	// active: indicates if raft peer is
	active bool
	// ctnum: candidate timeout random number
	ctnum time.Duration
	// etnum: election timeout random number
	etnum time.Duration
	// mu: for lock/unlock
	mu sync.Mutex
	// heartbeat: chan to indicate heartbeat
	heartbeat chan int
	// initialbeat: chan to indicate initial heartbeat
	initialbeat chan int
	// commitIndex index of highest log entry known to be committed
	// initialized to 0, increases monotonically
	commitIndex int
	// lastApplied:index of highest log entry applied to state machine
	// initialized to 0, increases monotonically
	lastApplied int
	// deactivate: deactivate raft peer
	deactivate chan int
	// lid: leader id
	lid int
	// logentry: chan to indicate when log entry has been added
	logentry chan int
	// cp chan to indicate when a command has been processed
	cp chan int
	// for each server, index of the next log entry to send to that server
	// initialized to leader last log index + 1
	nextIndex []int
	// for each server, index of highest log entry known to be replicated on server
	// initialized to 0, increases monotonically
	matchIndex []int
	// candidate heartbeat
	//chb chan int
}

//var rps *RaftPeerState

// NewRaftPeer -- this method should create an instance of the above struct and return a pointer
// to it back to the Controller, which calls this method.  this allows the Controller to create,
// interact with, and control the configuration as needed.  this method takes three parameters:
// -- port: this is the service port number where this Raft peer will listen for incoming messages
// -- id: this is the ID (or index) of this Raft peer in the peer group, ranging from 0 to num-1
// -- num: this is the number of Raft peers in the peer group (num > id)
func NewRaftPeer(port int, id int, num int) *RaftPeerState { // TODO: <---- change the return type
	// TODO: create a new raft peer and return a pointer to it

	// when a new raft peer is created, its initial state should be populated into the corresponding
	// struct entries, and its `remote.Service` and `remote.StubFactory` components should be created,
	// but the Service should not be started (the Controller will do that when ready).
	//
	// the `remote.Service` should be bound to port number `port`, as given in the input argument.
	// each `remote.StubFactory` will be used to interact with a different Raft peer, and different
	// port numbers are used for each Raft peer.  the Controller assigns these port numbers sequentially
	// starting from peer with `id = 0` and ending with `id = num-1`, so any peer who knows its own
	// `id`, `port`, and `num` can determine the port number used by any other peer.

	// Start pprof server

	rps := &RaftPeerState{}
	rps.currentTerm = 0
	rps.votedFor = -1
	rps.port = port
	rps.id = id
	rps.num = num
	rps.peers = make(map[int]string)
	rps.state = FOLLOWER
	//rps.heartbeat = make(chan int)
	//rps.heartbeat = make(chan int, 1)
	//rps.initialbeat = make(chan int, 2)
	rps.commitIndex = 0
	rps.active = false
	basePort := port - id
	for i := 0; i < num; i++ {
		if i == id {
			continue
		}

		p := basePort + i

		addr := "127.0.0.1:" + strconv.Itoa(p)

		rps.peers[i] = addr
	}

	rps.nextIndex = make([]int, num)
	rps.matchIndex = make([]int, num)
	rps.logs = make([]Log, 0)
	rps.logs = append(rps.logs, Log{Term: 0, Cmd: 0})
	/**
	rps.addr = "127.0.0.1:" + strconv.Itoa(port)
	var err error
	err = remote.StubFactory(&RaftInterface{}, rps.addr, false, false)
	if err != nil {
		return nil
	}

	*/
	var err error
	rps.serv, err = remote.NewService(&RaftInterface{}, rps, port, false, false)
	if err != nil {
		return nil
	}

	return rps
}

// `Activate` -- this method operates on your Raft peer struct and initiates functionality
// to allow the Raft peer to interact with others.  before the peer is activated, it can
// have internal algorithm state, but it cannot make remote calls using its stubs or receive
// remote calls using its underlying remote.Service interface.  in essence, when not activated,
// the Raft peer is "sleeping" from the perspective of any other Raft peer.
//
// this method is used exclusively by the Controller whenever it needs to "wake up" the Raft
// peer and allow it to start interacting with other Raft peers.  this is used to emulate
// connecting a new peer to the network or recovery of a previously failed peer.
//
// when this method is called, the Raft peer should do whatever is necessary to enable its
// remote.Service interface to support remote calls from other Raft peers as soon as the method
// returns (i.e., if it takes time for the remote.Service to start, this method should not
// return until that happens).  the method should not otherwise block the Controller, so it may
// be useful to spawn go routines from this method to handle the on-going operation of the Raft
// peer until the remote.Service stops.
//
// given an instance `rf` of your Raft peer struct, the Controller will call this method
// as `rf.Activate()`, so you should define this method accordingly. NOTE: this is _not_
// a remote call using the `remote.Service` interface of the Raft peer.  it uses direct
// method calls from the Controller, and is used purely for the purposes of the test code.
// you should not be using this method for any messaging between Raft peers.
//
// TODO: implement the `Activate` method

func (rps *RaftPeerState) Activate() {
	// start rpc service
	//rps.mu.Lock()
	rps.active = true
	//rps.mu.Unlock()
	err := rps.serv.Start()
	rps.heartbeat = make(chan int, 1)
	//rps.chb = make(chan int, 1)
	rps.deactivate = make(chan int)
	rps.logentry = make(chan int, 100)
	rps.cp = make(chan int)
	if err != nil {
		log.Fatal("raft service start fail")
	}

	go rps.raftState()
}

// Deactivate -- this method performs the "inverse" operation to `Activate`, namely to emulate
// disconnection / failure of the Raft peer.  when called, the Raft peer should effectively "go
// to sleep", meaning it should stop its underlying remote.Service interface, including shutting
// down the listening socket, causing any further remote calls to this Raft peer to fail due to
// connection error.  when deactivated, a Raft peer should not make or receive any remote calls,
// and any execution of the Raft protocol should effectively pause.  however, local state should
// be maintained, meaning if a Raft node was the LEADER when it was deactivated, it should still
// believe it is the leader when it reactivates.
//
// given an instance `rf` of your Raft peer struct, the Controller will call this method
// as `rf.Deactivate()`, so you should define this method accordingly. Similar notes / details
// apply here as with `Activate`
//
// TODO: implement the `Deactivate` method
func (rps *RaftPeerState) Deactivate() {
	rps.mu.Lock()
	rps.serv.Stop()
	rps.active = false
	rps.mu.Unlock()
	<-rps.deactivate
}

// TODO: implement remote method calls from other Raft peers:

// RequestVote -- as described in the Raft paper, called by other Raft peers
func (rps *RaftPeerState) RequestVote(term int, candidateId int, lastLogIndex int, lastLogTerm int) (bool, remote.RemoteObjectError) {

	// lock so only one raft peer has access to variable
	rps.mu.Lock()
	defer rps.mu.Unlock()

	// If this peer is the leader, reject the vote request
	if rps.leader {
		//Debug(dError, "S%d rejected vote request from S%d: T%d (Currently leader)", rps.id, candidateId, rps.currentTerm)
		return false, remote.RemoteObjectError{Err: "Currently leader"}
	}

	// reply false if term < currentTerm
	if term < rps.currentTerm {
		//Debug(dError, "S%d rejected vote request from S%d: T%d (Current term is higher)", rps.id, candidateId, rps.currentTerm)
		return false, remote.RemoteObjectError{Err: "Term is less than current term"}
	}

	// Update the current term and reset the vote if the candidate's term is greater
	if term > rps.currentTerm {
		rps.currentTerm = term
		rps.votedFor = -1
	}

	// Grant vote if no vote has been cast or the vote was already given to this candidate
	if rps.votedFor == -1 || rps.votedFor == candidateId {

		// Check if the candidate's log is at least as up-to-date as this peer's log
		fIndex := len(rps.logs) - 1

		flogs := rps.logs[fIndex]

		// If candidate's log term is older, reject the vote request
		if lastLogTerm < flogs.Term {
			//Debug(dWarn, "S%d rejected vote request from S%d: T%d (Candidate's log term is older)", rps.id, candidateId, rps.currentTerm)
			return false, remote.RemoteObjectError{Err: "logs not up to date"}
		}

		// If the logs are from the same term, but the candidate's log is shorter, reject the vote
		if lastLogTerm == flogs.Term && lastLogIndex < fIndex {
			//Debug(dWarn, "S%d rejected vote request from S%d: T%d (Candidate's log index is smaller)", rps.id, candidateId, rps.currentTerm)
			return false, remote.RemoteObjectError{Err: "logs not up to date"}
		}

		select {
		case rps.heartbeat <- 1:
			// Successfully sent the heartbeat
		default:

		}

		// If all conditions are met, grant the vote
		rps.votedFor = candidateId

		return true, remote.RemoteObjectError{}
	}

	// If the candidate doesn't meet the conditions for receiving a vote, reject the vote
	//Debug(dWarn, "S%d rejected vote request from S%d: T%d (Already voted for another candidate)", rps.id, candidateId, rps.currentTerm)
	return false, remote.RemoteObjectError{}
}

// AppendEntries -- as described in the Raft paper, called by other Raft peers
func (rps *RaftPeerState) AppendEntries(term int, leaderId int, prevLogIndex int,
	prevLogTerm int, entries []Log, leaderCommit int) (bool, remote.RemoteObjectError) {

	rps.mu.Lock()
	defer rps.mu.Unlock()

	// reject request if term < currentTerm
	if term < rps.currentTerm {
		//Debug(dWarn, "S%d: Term %d is less than current term %d. Rejecting AppendEntries", leaderId, term, rps.currentTerm)
		return false, remote.RemoteObjectError{Err: "Term is less than current term"}
	}

	rps.currentTerm = term
	//Debug(dInfo, "S%d: Term updated to %d", rps.id, rps.currentTerm)

	// Non-blocking send to heartbeat channel
	select {
	case rps.heartbeat <- 1:
		// Successfully sent the heartbeat
		//Debug(dInfo, "S%d SENDS HB TO S%d: T%d", leaderId, rps.id, rps.currentTerm)
	default:
		// If unable to send heartbeat (channel is full), log a debug message
		//Debug(dInfo, "S%d CANNOT SEND HB TO S%d: T%d", leaderId, rps.id, rps.currentTerm)
	}

	// If there are no entries to append, return
	if len(entries) == 0 {
		// Update the follower's commitIndex if the leader's commitIndex is higher

		if leaderCommit > rps.commitIndex {
			// The leader has committed new entries, so update the commitIndex
			rps.commitIndex = minCommit(leaderCommit, len(rps.logs))
			//Debug(dCommit, "S%d: commitIndex updated to %d", rps.id, rps.commitIndex)
		}

		//Debug(dInfo, "S%d: No log entries to append. Returning success.", rps.id)
		return true, remote.RemoteObjectError{}
	}

	// Index of the last log entry
	flogIndex := len(rps.logs)
	// If no logs, append entries directly
	if flogIndex == 1 {
		rps.logs = append(rps.logs, entries...)
		//Debug(dInfo, "S%d: No existing logs. Appending new entries.", rps.id)
		return true, remote.RemoteObjectError{}
	}

	// Check if prevLogIndex is out of range
	if prevLogIndex >= flogIndex {
		//Debug(dInfo, "S%d: Entries Rejected: prevLogIndex %d is too high (flogIndex = %d)", rps.id, prevLogIndex, flogIndex)
		return false, remote.RemoteObjectError{Err: "Entries Rejected: prevLogIndex is too high"}
	}

	// Check if the term of the prevLogIndex matches the expected term
	flogTerm := rps.logs[prevLogIndex].Term
	if prevLogTerm != flogTerm {
		//Debug(dInfo, "S%d: Entries Rejected: prevLogTerm %d does not match follower logTerm %d", rps.id, prevLogTerm, flogTerm)
		return false, remote.RemoteObjectError{Err: "Entries Rejected: prevLogTerm mismatch"}
	}

	// Truncate conflicting logs and append new entries
	rps.logs = rps.logs[:prevLogIndex+1]
	rps.logs = append(rps.logs, entries...)
	//Debug(dInfo, "S%d: new entries to append: %v", rps.id, entries)
	//Debug(dInfo, "S%d: Logs truncated and new entries appended: %v", rps.id, rps.logs)

	// Update the follower's commitIndex if the leader's commitIndex is higher
	if leaderCommit > rps.commitIndex {
		// The leader has committed new entries, so update the commitIndex
		rps.commitIndex = minCommit(leaderCommit, len(rps.logs))
		//Debug(dCommit, "S%d: commitIndex updated to %d", rps.id, rps.commitIndex)
	}

	return true, remote.RemoteObjectError{}
}

func minCommit(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetCommittedCmd -- called (only) by the Controller.  this method provides an input argument
// `index`.  if the Raft peer has a log entry at the given `index`, and that log entry has been
// committed (per the Raft algorithm), then the command stored in the log entry should be returned
// to the Controller.  otherwise, the Raft peer should return the value 0, which is not a valid
// command number and indicates that no committed log entry exists at that index
func (rps *RaftPeerState) GetCommittedCmd(index int) (int, remote.RemoteObjectError) {

	rps.mu.Lock()
	defer rps.mu.Unlock()
	if rps.commitIndex >= index {
		cmd := rps.logs[index].Cmd
		//Debug(dInfo, "S%d: COMMITED ID IS %d COMMAND: INDEX: %d, CMD %d, COMMITED INDEX %d, %v", rps.id, rps.id, index, cmd, rps.commitIndex, rps.logs)
		return cmd, remote.RemoteObjectError{}
	}

	//Debug(dInfo, "S%d: COMMITED NOT FOUND ID IS %d COMMAND: INDEX: %d, COMMITED INDEX %d, %v", rps.id, rps.id, index, rps.commitIndex, rps.logs)

	return 0, remote.RemoteObjectError{Err: "Committed Command Not Found"}
}

//
// GetStatus -- called (only) by the Controller.  this method takes no arguments and is essentially
// a "getter" for the state of the Raft peer, including the Raft peer's current term, current last
// log index, role in the Raft algorithm, and total number of remote calls handled since starting.
// the method returns a `StatusReport` struct as defined at the top of this file.

func (rps *RaftPeerState) GetStatus() (StatusReport, remote.RemoteObjectError) {
	rps.mu.Lock()
	term := rps.currentTerm
	leader := rps.leader
	index := len(rps.logs) - 1
	rps.mu.Unlock()
	count := rps.serv.GetCount()
	sr := StatusReport{
		Index:     index,
		Term:      term,
		Leader:    leader,
		CallCount: count,
	}

	roe := remote.RemoteObjectError{Err: ""}
	return sr, roe
}

// NewCommand -- called (only) by the Controller.  this method emulates submission of a new command
// by a Raft client to this Raft peer, which should be handled and processed according to the rules
// of the Raft algorithm.  once handled, the Raft peer should return a `StatusReport` struct with
// the updated status after the new command was handled.
func (rps *RaftPeerState) NewCommand(cmd int) (StatusReport, remote.RemoteObjectError) {

	rps.mu.Lock()
	isleader := rps.leader
	rps.mu.Unlock()
	if !isleader {
		return StatusReport{}, remote.RemoteObjectError{Err: "Peer is not a leader"}
	}
	rps.logentry <- cmd
	<-rps.cp
	sr, roe := rps.GetStatus()
	return sr, roe
}

// general notes:
//
// - you are welcome to use additional helper to handle various aspects of the Raft algorithm logic
//   within the scope of a single Raft peer.  you should not need to create any additional remote
//   calls between Raft peers or the Controller.  if there is a desire to create additional remote
//   calls, please talk with the course staff before doing so.
//
// - please make sure to read the Raft paper (https://raft.github.io/raft.pdf) before attempting
//   any coding for this lab.  you will most likely need to refer to it many times during your
//   implementation and testing tasks, so please consult the paper for algorithm details.
//
// - each Raft peer will accept a lot of remote calls from other Raft peers and the Controller,
//   so use of locks / mutexes is essential.  you are expected to use locks correctly in order to
//   prevent race conditions in your implementation.  the Makefile supports testing both without
//   and with go's race detector, and the final auto-grader will enable the race detector, which will
//   cause tests to fail if any race conditions are encountered.

// 'raftState' -- goroutine method to determine whether the raft peer is either in
// FOLLOWER, CANDIDATE, or LEADER state
// and perform the following actions based on the state
// Followers - Respond to RPCs from candidates and leaders
func (rps *RaftPeerState) raftState() {

	var rcvdHbt bool

	for {
		rps.mu.Lock()
		if !rps.active {
			rps.mu.Unlock()
			rps.deactivate <- 1
			return
		}

		//id := rps.id
		//prt := rps.port
		term := rps.currentTerm
		rps.mu.Unlock()
		switch rps.state {
		case FOLLOWER:
			// start new timer if timer expires without
			// receiving heartbeat convert to candidate
			et := newElectionTimer()
			if !rcvdHbt {
				//Debug(dFollower, "S%d -> Follower State: T%d:", id, term)
			}

			// wait for heartbeat
			select {
			case <-rps.heartbeat:
				et.Stop()
				//Debug(dLog, "S%d RCVD heartbeat in follower state: T%d", id, term)

				rcvdHbt = true
			// If a follower receives no communication over a period of time
			// convert to candidate to start election
			case <-et.C:
				rcvdHbt = false
				rps.mu.Lock()
				rps.state = CANDIDATE
				rps.lid = -1
				rps.mu.Unlock()
				//Debug(dTimer, "S%d timeout expired: for: T%d: convert to candidate", id, term)
			}

		case CANDIDATE:
			// On conversion to candidate, start election:
			// Increment currentTerm
			// Vote for self
			rps.mu.Lock()
			rps.currentTerm++
			rps.votedFor = rps.id
			// debugging purposes
			//vtd := rps.votedFor
			term = rps.currentTerm
			// log index and term to send for request vote
			logIndex := len(rps.logs) - 1
			logTerm := rps.logs[logIndex].Term
			rps.mu.Unlock()
			//Debug(dClient, "S%d -> Begin Election. V%d, T%d", id, vtd, term)
			// Create a new candidate timer or Reset election timer
			ct := newCandidateTimer()
			// total vote count
			tv := 0
			// total num of servers
			s := len(rps.peers)
			stub := &RaftInterface{}
			// Send RequestVote RPCs to all other servers
			for n, v := range rps.peers {
				err := remote.StubFactory(stub, v, false, false)
				if err != nil {
					log.Fatal("Could not create raft StubFactory", n)
				}

				isTrue, remoteErr := stub.RequestVote(term, rps.id, logIndex, logTerm)
				if !isTrue {
					if strings.Contains(remoteErr.Error(), "nil socket") {
						s--
						//Debug(dError, "S%d: Offline: %s: result %t", n, remoteErr.Err, isTrue)
					}
					//Debug(dWarn, "S%d: rejected vote request: %s: result %t", n, remoteErr.Err, isTrue)
				}

				if isTrue {
					tv++
					//Debug(dVote, "S%d: RCVD Votes from S%d: Term %d", rps.id, n, term)
				}

			}

			// Calculate the percentage of votes received by the candidate
			// `tv` is the number of votes received, and `s` is the total number of servers (including the candidate itself).
			perc := float64(tv) / float64(s)

			// If more than 50% of servers have voted for this candidate, the candidate becomes the leader
			// A majority is defined as more than half the total servers (perc > 0.50).
			if perc > 0.50 {
				// Stop the candidate's election timer since it is now a leader
				ct.Stop()

				rps.mu.Lock()

				// Update the state to LEADER
				rps.state = LEADER
				rps.leader = true

				// Get the current length of the log
				l := len(rps.logs)

				// Initialize `nextIndex` and `matchIndex` for each server in the cluster.
				// The `nextIndex` is the index of the next log entry the leader will send to a follower,
				// and `matchIndex` tracks the highest log entry known to be replicated on the follower.
				for i := 0; i < rps.num; i++ {
					rps.nextIndex[i] = l
					rps.matchIndex[i] = 0
				}

				rps.mu.Unlock()

				//Debug(dLeader, "S%d (Term %d): Received majority of votes (%d/%d). Now transitioning to LEADER state.", id, term, tv, s)
				break
			}

			<-ct.C
			// If AppendEntries RPC received from new leader: convert to follower
			// If election timeout elapses: start new election
			select {
			case <-rps.heartbeat:
				ct.Stop()
				rps.mu.Lock()
				rps.state = FOLLOWER
				rps.leader = false
				rps.mu.Unlock()
				//Debug(dFollower, "S%d (Term %d): Received heartbeat, transition to FOLLOWER state.", id, term)
			default:
				//Debug(dInfo, "S%d (Term %d): Election timeout expired. Reset candidate, preparing new election.", id, term)
			}

		case LEADER:
			prevLogIndex := 0
			prevLogTerm := 0
			var newCmd bool
			select {
			case cmd := <-rps.logentry:
				// Append the new log entry to the leader's log
				rps.logs = append(rps.logs, Log{Term: term, Cmd: cmd})
				newCmd = true
				//Debug(dLog, "S%d: Added new log entry: %v", rps.id, cmd)
			default:
				newCmd = false
				// No new log entry to append
			}

			// Count of successful heartbeats (entries replicated)
			ep := 0
			// Total number of servers in the cluster
			nservs := rps.num - 1
			// Send heartbeats (AppendEntries RPCs) to all followers
			for n, v := range rps.peers {
				// Prepare remote stub for communication with other peers
				stub := &RaftInterface{}
				err := remote.StubFactory(stub, v, false, false)
				if err != nil {
					log.Fatal("Could not create raft StubFactory")
				}

				rps.mu.Lock()
				// Current leader's ID and commit index
				leaderId := rps.id
				leaderCommit := rps.commitIndex

				// Prepare log entries to be sent to the follower
				var entries []Log
				nIdx := rps.nextIndex[n]
				//Debug(dInfo, "S%d: IDX IS %d FOR SERVER S%d", rps.id, nIdx, n)
				if nIdx < len(rps.logs) {
					entries = rps.logs[nIdx:]
					prevLogIndex = nIdx - 1
					prevLogTerm = rps.logs[prevLogIndex].Term
				} else {
					entries = make([]Log, 0)
				}

				rps.mu.Unlock()
				// Debug: Log the details of the AppendEntries RPC request
				//Debug(dLeader, "S%d: Sending AppendEntries RPC to S%d. Term: %d, PrevLogIndex: %d, PrevLogTerm: %d, Entries: %v", rps.id, n, rps.currentTerm, prevLogIndex, prevLogTerm, entries)

				// Make the AppendEntries RPC call to the follower
				isTrue, remoteErr := stub.AppendEntries(term, leaderId, prevLogIndex, prevLogTerm, entries, leaderCommit)
				if !isTrue {
					if strings.Contains(remoteErr.Error(), "nil socket") {
						// Decrement server count if the peer is not reachable
						nservs--
						//Debug(dError, "S%d: Connection to S%d lost, skipping AppendEntries RPC", rps.id, n)
						continue
					}

					// If the follower's term is higher, the leader should step down to a follower
					if strings.Contains(remoteErr.Error(), "Term is less than current term") {
						rps.mu.Lock()
						rps.state = FOLLOWER
						rps.leader = false
						rps.mu.Unlock()
						//Debug(dFollower, "S%d: Stepping down as leader, term conflict with S%d", rps.id, n)
						break
					}

					// If the follower rejects some log entries, backtrack and try again
					if strings.Contains(remoteErr.Error(), "Entries Rejected") {
						rps.mu.Lock()
						idx := rps.nextIndex[n]
						idx--
						rps.nextIndex[n] = idx
						rps.mu.Unlock()
						//Debug(dLeader, "S%d: Entries rejected by S%d, decrementing nextIndex to %d", rps.id, n, rps.nextIndex[n])
					}
				}

				// If the AppendEntries RPC was successful, update the follower's state
				if isTrue {
					if len(entries) > 0 {
						rps.mu.Lock()
						rps.nextIndex[n] = len(rps.logs)
						rps.matchIndex[n] = len(rps.logs) - 1
						ep++
						rps.mu.Unlock()
						//Debug(dLeader, "S%d: Successfully replicated log to S%d. nextIndex: %d, matchIndex: %d", rps.id, n, rps.nextIndex[n], rps.matchIndex[n])
					}

				}
			}

			s := rps.num - 1
			if nservs < (s / 2) {
				if newCmd {
					rps.cp <- 1
				}
				break // Skip this round of commit if not enough servers are reachable
			}

			// Calculate the percentage of servers that have replicated the entry
			//perc := float64(ep) / float64(nservs)

			// Debug: Log the percentage of servers that have replicated the log entry
			//Debug(dLog2, "S%d: %f%% of servers have replicated the log entry", rps.id, perc*100)

			// If more than 50% of the servers have replicated the entry, set majority to true
			// If more than 50% of the servers have replicated the entry, it is considered a majority
			majority := ep > nservs/2

			//Debug(dLog2, "S%d: %f%% MAJORITY, %t", rps.id, perc*100, majority)

			if majority {
				// Find the highest matchIndex replicated by a majority of servers
				maxMatchIndex := 0
				for _, v := range rps.matchIndex {
					if v > maxMatchIndex {
						maxMatchIndex = v
					}
				}

				// Now check if we can commit any log entries up to maxMatchIndex
				for N := rps.commitIndex + 1; N <= maxMatchIndex; N++ {

					// Check if the log entry at N is from the current term and if a majority has replicated it
					majorityCount := 0
					for _, matchIdx := range rps.matchIndex {
						if matchIdx >= N {
							majorityCount++
						}
					}

					if majorityCount > nservs/2 && rps.logs[N].Term == rps.currentTerm {
						// If majority has replicated the entry and it's from the current term, commit it
						rps.mu.Lock()
						rps.commitIndex = N // Update commitIndex
						rps.mu.Unlock()
						//Debug(dCommit, "S%d: commitIndex updated to %d", rps.id, rps.commitIndex)
					}
				}
			}

			if newCmd {
				rps.cp <- 1
			}

			// Reset the leader's heartbeat timer after sending heartbeats
			leaderTimer := newLeaderHeartbeatTimer()
			<-leaderTimer.C

		}

	}

}

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

// Helper function to create a new timer for election timeouts
func newElectionTimer() *time.Timer {
	timeout := time.Duration(rand.Intn(200)+1200) * time.Millisecond
	return time.NewTimer(timeout)
}

// Helper function to create a new timer for candidate timeouts
func newCandidateTimer() *time.Timer {
	timeout := time.Duration(rand.Intn(400)+400) * time.Millisecond
	return time.NewTimer(timeout)
}

// Helper function to create a new leader heartbeat timer
func newLeaderHeartbeatTimer() *time.Timer {
	// Set the leader heartbeat timeout to a fixed duration, for example 100 milliseconds
	timeout := time.Duration(rand.Intn(150)+200) * time.Millisecond // Heartbeat every 100ms (adjust as needed)
	return time.NewTimer(timeout)
}
