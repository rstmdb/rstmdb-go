package rstmdb

import "encoding/json"

// operation constants for the 22 RCP operations.
type operation string

const (
	opHello             operation = "HELLO"
	opAuth              operation = "AUTH"
	opPing              operation = "PING"
	opBye               operation = "BYE"
	opInfo              operation = "INFO"
	opPutMachine        operation = "PUT_MACHINE"
	opGetMachine        operation = "GET_MACHINE"
	opListMachines      operation = "LIST_MACHINES"
	opCreateInstance    operation = "CREATE_INSTANCE"
	opGetInstance       operation = "GET_INSTANCE"
	opListInstances     operation = "LIST_INSTANCES"
	opDeleteInstance    operation = "DELETE_INSTANCE"
	opApplyEvent        operation = "APPLY_EVENT"
	opBatch             operation = "BATCH"
	opWatchInstance     operation = "WATCH_INSTANCE"
	opWatchAll          operation = "WATCH_ALL"
	opUnwatch           operation = "UNWATCH"
	opSnapshotInstance  operation = "SNAPSHOT_INSTANCE"
	opWALRead           operation = "WAL_READ"
	opWALStats          operation = "WAL_STATS"
	opCompact           operation = "COMPACT"
	opFlushAll          operation = "FLUSH_ALL"
	opReplicationStatus operation = "REPLICATION_STATUS"
	opBackupBegin       operation = "BACKUP_BEGIN"
	opBackupChunk       operation = "BACKUP_CHUNK"
	opBackupEnd         operation = "BACKUP_END"
)

// Wire-level message types (unexported).

type request struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Op     string `json:"op"`
	Params any    `json:"params,omitempty"`
}

type rawResponse struct {
	Type   string          `json:"type"`
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
	Meta   *Meta           `json:"meta,omitempty"`
}

// rawMessage is used to peek at the "type" field for dispatch.
type rawMessage struct {
	Type string `json:"type"`
}

// Meta contains response metadata.
type Meta struct {
	ServerTime string `json:"server_time,omitempty"`
	Leader     *bool  `json:"leader,omitempty"`
	WALOffset  uint64 `json:"wal_offset,omitempty"`
	TraceID    string `json:"trace_id,omitempty"`
}

// HelloResult is returned from HELLO (handled internally by Connect).
type HelloResult struct {
	ProtocolVersion int      `json:"protocol_version"`
	WireMode        string   `json:"wire_mode"`
	ServerName      string   `json:"server_name"`
	ServerVersion   string   `json:"server_version"`
	Features        []string `json:"features"`
}

// MachineDefinition defines a state machine.
type MachineDefinition struct {
	States      []string     `json:"states"`
	Initial     string       `json:"initial"`
	Transitions []Transition `json:"transitions"`
	Meta        any          `json:"meta,omitempty"`
}

// Transition defines a state transition.
type Transition struct {
	From  StringOrSlice `json:"from"`
	Event string        `json:"event"`
	To    string        `json:"to"`
	Guard string        `json:"guard,omitempty"`
}

// StringOrSlice handles JSON "from" being either a string or []string.
type StringOrSlice []string

func (s *StringOrSlice) UnmarshalJSON(data []byte) error {
	// Try string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = StringOrSlice{str}
		return nil
	}
	// Try []string
	var slice []string
	if err := json.Unmarshal(data, &slice); err != nil {
		return err
	}
	*s = slice
	return nil
}

func (s StringOrSlice) MarshalJSON() ([]byte, error) {
	if len(s) == 1 {
		return json.Marshal(s[0])
	}
	return json.Marshal([]string(s))
}

// PutMachineRequest is the request for PUT_MACHINE.
type PutMachineRequest struct {
	Machine    string            `json:"machine"`
	Version    int               `json:"version"`
	Definition MachineDefinition `json:"definition"`
	Checksum   string            `json:"checksum,omitempty"`
}

// PutMachineResult is returned from PUT_MACHINE.
type PutMachineResult struct {
	Machine        string `json:"machine"`
	Version        int    `json:"version"`
	StoredChecksum string `json:"stored_checksum"`
	Created        bool   `json:"created"`
}

// MachineInfo is returned from GET_MACHINE.
type MachineInfo struct {
	Definition MachineDefinition `json:"definition"`
	Checksum   string            `json:"checksum"`
}

// MachineSummary is an item in the LIST_MACHINES response.
type MachineSummary struct {
	Machine  string `json:"machine"`
	Versions []int  `json:"versions"`
}

// CreateInstanceRequest is the request for CREATE_INSTANCE.
type CreateInstanceRequest struct {
	InstanceID     string         `json:"instance_id,omitempty"`
	Machine        string         `json:"machine"`
	Version        int            `json:"version"`
	InitialCtx     map[string]any `json:"initial_ctx,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
}

// CreateInstanceResult is returned from CREATE_INSTANCE.
type CreateInstanceResult struct {
	InstanceID string `json:"instance_id"`
	State      string `json:"state"`
	WALOffset  uint64 `json:"wal_offset"`
}

// Instance is returned from GET_INSTANCE.
type Instance struct {
	Machine       string         `json:"machine"`
	Version       int            `json:"version"`
	State         string         `json:"state"`
	Ctx           map[string]any `json:"ctx"`
	LastEventID   string         `json:"last_event_id,omitempty"`
	LastWALOffset uint64         `json:"last_wal_offset"`
}

// InstanceSummary is an item in the LIST_INSTANCES response.
type InstanceSummary struct {
	ID            string `json:"id"`
	Machine       string `json:"machine"`
	Version       int    `json:"version"`
	State         string `json:"state"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
	LastWALOffset uint64 `json:"last_wal_offset"`
}

// InstanceList is the LIST_INSTANCES response.
type InstanceList struct {
	Instances []InstanceSummary `json:"instances"`
	Total     int               `json:"total"`
	HasMore   bool              `json:"has_more"`
}

// DeleteInstanceResult is returned from DELETE_INSTANCE.
type DeleteInstanceResult struct {
	InstanceID string `json:"instance_id"`
	Deleted    bool   `json:"deleted"`
	WALOffset  uint64 `json:"wal_offset"`
}

// ApplyEventRequest is the request for APPLY_EVENT.
type ApplyEventRequest struct {
	InstanceID        string         `json:"instance_id"`
	Event             string         `json:"event"`
	Payload           map[string]any `json:"payload,omitempty"`
	ExpectedState     string         `json:"expected_state,omitempty"`
	ExpectedWALOffset *uint64        `json:"expected_wal_offset,omitempty"`
	EventID           string         `json:"event_id,omitempty"`
	IdempotencyKey    string         `json:"idempotency_key,omitempty"`
}

// ApplyEventResult is returned from APPLY_EVENT.
type ApplyEventResult struct {
	FromState string         `json:"from_state"`
	ToState   string         `json:"to_state"`
	Ctx       map[string]any `json:"ctx"`
	WALOffset uint64         `json:"wal_offset"`
	Applied   bool           `json:"applied"`
	EventID   string         `json:"event_id,omitempty"`
}

// BatchMode controls batch execution behavior.
type BatchMode string

const (
	BatchAtomic     BatchMode = "atomic"
	BatchBestEffort BatchMode = "best_effort"
)

// BatchOperation represents a single operation in a batch.
type BatchOperation struct {
	Op     string `json:"op"`
	Params any    `json:"params"`
}

// BatchResult represents the result of one batch operation.
type BatchResult struct {
	Status string          `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

// WatchInstanceRequest is the request for WATCH_INSTANCE.
type WatchInstanceRequest struct {
	InstanceID string `json:"instance_id"`
	IncludeCtx bool   `json:"include_ctx"`
	FromOffset uint64 `json:"from_offset,omitempty"`
}

// Event represents a streamed state change event (server-pushed).
type Event struct {
	SubscriptionID string         `json:"subscription_id"`
	InstanceID     string         `json:"instance_id"`
	Machine        string         `json:"machine"`
	Version        int            `json:"version"`
	WALOffset      uint64         `json:"wal_offset"`
	FromState      string         `json:"from_state"`
	ToState        string         `json:"to_state"`
	EventName      string         `json:"event"`
	Payload        map[string]any `json:"payload"`
	Ctx            map[string]any `json:"ctx,omitempty"`
}

// ServerInfo is returned from INFO.
type ServerInfo struct {
	ServerName      string   `json:"server_name"`
	ServerVersion   string   `json:"server_version"`
	ProtocolVersion int      `json:"protocol_version"`
	Features        []string `json:"features"`
	MaxFrameBytes   int      `json:"max_frame_bytes"`
	MaxBatchOps     int      `json:"max_batch_ops"`
}

// SnapshotResult is returned from SNAPSHOT_INSTANCE.
type SnapshotResult struct {
	InstanceID string `json:"instance_id"`
	SnapshotID string `json:"snapshot_id"`
	WALOffset  uint64 `json:"wal_offset"`
	SizeBytes  int64  `json:"size_bytes,omitempty"`
	Checksum   string `json:"checksum,omitempty"`
}

// WALReadResult is returned from WAL_READ.
type WALReadResult struct {
	Records    []WALRecord `json:"records"`
	NextOffset uint64      `json:"next_offset"`
}

// WALRecord represents a single WAL entry.
type WALRecord struct {
	Sequence uint64 `json:"sequence"`
	Offset   uint64 `json:"offset"`
	Entry    any    `json:"entry"`
}

// WALStatsResult is returned from WAL_STATS.
type WALStatsResult struct {
	EntryCount   int64   `json:"entry_count"`
	SegmentCount int     `json:"segment_count"`
	TotalSize    int64   `json:"total_size_bytes"`
	LatestOffset uint64  `json:"latest_offset"`
	IOStats      IOStats `json:"io_stats"`
}

// IOStats contains WAL I/O statistics.
type IOStats struct {
	BytesWritten uint64 `json:"bytes_written"`
	BytesRead    uint64 `json:"bytes_read"`
	Writes       uint64 `json:"writes"`
	Reads        uint64 `json:"reads"`
	Fsyncs       uint64 `json:"fsyncs"`
}

// CompactResult is returned from COMPACT.
type CompactResult struct {
	SnapshotsCreated int   `json:"snapshots_created"`
	SegmentsDeleted  int   `json:"segments_deleted"`
	BytesReclaimed   int64 `json:"bytes_reclaimed"`
	TotalSnapshots   int   `json:"total_snapshots"`
	WALSegments      int   `json:"wal_segments"`
}

// FlushAllResult is returned from FLUSH_ALL.
type FlushAllResult struct {
	Flushed          bool `json:"flushed"`
	InstancesRemoved int  `json:"instances_removed"`
	MachinesRemoved  int  `json:"machines_removed"`
}

// ReplicationStatusResult is returned from REPLICATION_STATUS.
type ReplicationStatusResult struct {
	Role                string           `json:"role"`
	Mode                string           `json:"mode,omitempty"`
	PrimarySequence     *uint64          `json:"primary_sequence,omitempty"`
	ConnectedReplicas   *int             `json:"connected_replicas,omitempty"`
	Replicas            []map[string]any `json:"replicas,omitempty"`
	Upstream            string           `json:"upstream,omitempty"`
	LastAppliedSequence *uint64          `json:"last_applied_sequence,omitempty"`
	LagEntries          *uint64          `json:"lag_entries,omitempty"`
	LagSeconds          *float64         `json:"lag_seconds,omitempty"`
}

// BackupBeginOptions configures BACKUP_BEGIN.
type BackupBeginOptions struct {
	Compression string `json:"compression,omitempty"`
}

// BackupBeginResult is returned from BACKUP_BEGIN.
type BackupBeginResult struct {
	BackupID   string         `json:"backup_id"`
	Manifest   map[string]any `json:"manifest"`
	TotalBytes uint64         `json:"total_bytes"`
	ChunkSize  uint64         `json:"chunk_size"`
}

// BackupChunkResult is returned from BACKUP_CHUNK.
type BackupChunkResult struct {
	Bytes string `json:"bytes"`
	EOF   bool   `json:"eof"`
}

// BackupEndResult is returned from BACKUP_END.
type BackupEndResult struct {
	OK bool `json:"ok"`
}
