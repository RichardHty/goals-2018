package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"labrpc"
	"math/rand"
	"sync"
	"time"
)

// import "bytes"
// import "encoding/gob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
//
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentTerm   int
	votedFor      int
	log           []LogEntry
	commitIndex   int
	lastApplied   int
	nextIndex     []int
	matchIndex    []int
	state         int
	chanGrantVote chan bool
	voteCount     int
	chanLeader    chan bool
	chanHeartbeat chan bool
}

type LogEntry struct {
	LogIndex int
	LogTerm  int
}

const (
	STATE_LEADER = iota
	STATE_CANDIDATE
	STATE_FOLLOWER
	HEARTBEATS = 100 * time.Millisecond
)

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	term = rf.currentTerm
	isleader = (rf.state == STATE_LEADER)
	// Your code here (2A).
	return term, isleader
}

//customized  utility func
func (rf *Raft) getLastIndex() int {
	return rf.log[len(rf.log)-1].LogIndex
}
func (rf *Raft) getLastTerm() int {
	return rf.log[len(rf.log)-1].LogTerm
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := gob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := gob.NewDecoder(r)
	// d.Decode(&rf.xxx)
	// d.Decode(&rf.yyy)
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term      int
	Success   bool
	NextIndex int
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()
	reply.VoteGranted = false
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		//fmt.Printf("%v currentTerm:%v vote reject for:%v term:%v", rf.me, rf.currentTerm, args.CandidateId, args.Term)
		return
	}
	if args.Term > rf.currentTerm { //should vote for current requester
		rf.currentTerm = args.Term
		rf.state = STATE_FOLLOWER
		rf.votedFor = -1
	}
	reply.Term = rf.currentTerm
	upToDate := false
	if args.LastLogTerm > rf.getLastTerm() {
		upToDate = true
	}
	if args.LastLogTerm == rf.getLastTerm() && args.LastLogIndex >= rf.getLastIndex() {
		upToDate = true
	}
	if upToDate && (rf.votedFor == -1 || rf.votedFor == args.CandidateId) {
		rf.chanGrantVote <- true
		rf.state = STATE_FOLLOWER
		reply.VoteGranted = true
		rf.votedFor = args.CandidateId
		//fmt.Printf("%v currentTerm:%v vote for:%v term:%v", rf.me, rf.currentTerm, args.CandidateId, args.Term)
	}
	//fmt.Printf("%v currentTerm:%v vote reject for:%v term:%v\n", rf.me, rf.currentTerm, args.CandidateId, args.Term)
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if ok {
		if rf.state != STATE_CANDIDATE {
			return ok
		}
		if args.Term != rf.currentTerm {
			return ok
		}
		if reply.Term > rf.currentTerm {
			rf.currentTerm = reply.Term
			rf.state = STATE_FOLLOWER
			rf.votedFor = -1
			rf.persist()
		}
		if reply.VoteGranted {
			rf.voteCount++
			if rf.state == STATE_CANDIDATE && rf.voteCount > len(rf.peers)/2 {
				rf.state = STATE_FOLLOWER
				rf.chanLeader <- true
			}
		}
	}
	return ok
}

//now handle with AppendEntries stuffs
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()
	reply.Success = false
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.NextIndex = rf.getLastIndex() + 1
		//fmt.Printf("%v currentTerm: %v rejected %v:%v\n", rf.me, rf.currentTerm, args.LeaderId, args.Term)
		return
	}
	rf.chanHeartbeat <- true
	//fmt.Printf("%d respond for %v\n", rf.me, args.LeaderId)
	if args.Term > rf.currentTerm { //successfuly
		rf.currentTerm = args.Term
		rf.state = STATE_FOLLOWER
		rf.votedFor = -1
	}
	reply.Term = args.Term
	if args.PrevLogIndex > rf.getLastIndex() {
		reply.NextIndex = rf.getLastIndex() + 1
		return
	}
	baseIndex := rf.log[0].LogIndex
	if args.PrevLogTerm > baseIndex {
		term := rf.log[args.PrevLogIndex-baseIndex].LogTerm
		if args.PrevLogTerm != term {
			for i := args.PrevLogIndex - 1; i >= baseIndex; i-- {
				if rf.log[i-baseIndex].LogTerm != term {
					reply.NextIndex = i + 1
					break
				}
			}
			return
		}
	}
	if args.PrevLogIndex >= baseIndex {
		rf.log = rf.log[:args.PrevLogIndex+1-baseIndex]
		rf.log = append(rf.log, args.Entries...)
		reply.Success = true
		reply.NextIndex = rf.getLastIndex() + 1
	}
	return
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if ok {
		if rf.state != STATE_LEADER {
			return ok
		}
		if args.Term != rf.currentTerm {
			return ok
		}
		if reply.Term > rf.currentTerm {
			rf.currentTerm = reply.Term
			rf.state = STATE_FOLLOWER
			rf.votedFor = -1
			rf.persist()
			return ok
		}
		if reply.Success {
			if len(args.Entries) > 0 {
				rf.nextIndex[server] = args.Entries[len(args.Entries)-1].LogIndex + 1
				rf.matchIndex[server] = rf.nextIndex[server] - 1
			}
		} else {
			rf.nextIndex[server] = reply.NextIndex
		}
	}
	return ok
}

//broadcast RequestVote and AppendEntries to all
func (rf *Raft) bcRequestVote() {
	var args RequestVoteArgs
	rf.mu.Lock()
	args.Term = rf.currentTerm
	args.CandidateId = rf.me
	args.LastLogTerm = rf.getLastTerm()
	args.LastLogIndex = rf.getLastIndex()
	rf.mu.Unlock()
	for i := range rf.peers {
		if i != rf.me && rf.state == STATE_CANDIDATE {
			go func(i int) {
				var reply RequestVoteReply
				//fmt.Printf("%v RequestVote to %v\n", rf.me, i)
				rf.sendRequestVote(i, &args, &reply)
			}(i)
		}
	}
}

func (rf *Raft) bcAppendEntries() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	baseIndex := rf.log[0].LogIndex
	for i := range rf.peers {
		if i != rf.me && rf.state == STATE_LEADER {
			if rf.nextIndex[i] > baseIndex {
				var args AppendEntriesArgs
				args.Term = rf.currentTerm
				args.LeaderId = rf.me
				args.PrevLogIndex = rf.nextIndex[i] - 1
				//	//fmt.Printf("baseIndex:%d PrevLogIndex:%d\n",baseIndex,args.PrevLogIndex )
				args.PrevLogTerm = rf.log[args.PrevLogIndex-baseIndex].LogTerm
				args.Entries = make([]LogEntry, len(rf.log[args.PrevLogIndex+1-baseIndex:]))
				copy(args.Entries, rf.log[args.PrevLogIndex+1-baseIndex:])
				go func(i int, args AppendEntriesArgs) {
					var reply AppendEntriesReply
					rf.sendAppendEntries(i, &args, &reply)
				}(i, args)
			}
		}
	}
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true
	// Your code here (2B).

	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.state = STATE_FOLLOWER
	rf.votedFor = -1
	rf.log = append(rf.log, LogEntry{LogTerm: 0})
	rf.currentTerm = 0
	rf.chanGrantVote = make(chan bool, 100)
	rf.chanLeader = make(chan bool, 100)
	rf.chanHeartbeat = make(chan bool, 100)

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	go rf.run()

	return rf
}

func (rf *Raft) run() {
	for {
		switch rf.state {
		case STATE_FOLLOWER:
			select {
			case <-rf.chanHeartbeat:
			case <-rf.chanGrantVote:
			case <-time.After(time.Duration(rand.Int63()%333+550) * time.Millisecond):
				rf.state = STATE_CANDIDATE
			}
		case STATE_LEADER:
			//fmt.Printf("Leader:%v %v\n", rf.me, "boatcastAppendEntries	")
			rf.bcAppendEntries()
			time.Sleep(HEARTBEATS)
		case STATE_CANDIDATE:
			rf.mu.Lock()
			rf.currentTerm++
			rf.votedFor = rf.me
			rf.voteCount = 1
			rf.persist()
			rf.mu.Unlock()
			go rf.bcRequestVote()
			//fmt.Printf("%v become CANDIDATE %v\n", rf.me, rf.currentTerm)
			select {
			case <-time.After(time.Duration(rand.Int63()%333+550) * time.Millisecond):
			case <-rf.chanHeartbeat:
				rf.state = STATE_FOLLOWER
				//fmt.Printf("CANDIDATE %v reveive chanHeartbeat\n", rf.me)
			case <-rf.chanLeader:
				rf.mu.Lock()
				rf.state = STATE_LEADER
				//fmt.Printf("%v is Leader\n", rf.me)
				rf.nextIndex = make([]int, len(rf.peers))
				rf.matchIndex = make([]int, len(rf.peers))
				for i := range rf.peers {
					rf.nextIndex[i] = rf.getLastIndex() + 1
					rf.matchIndex[i] = 0
				}
				rf.mu.Unlock()
			}
		}
	}
}
