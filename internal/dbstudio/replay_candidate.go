//go:build desk_recovery_candidate

package dbstudio

// Candidate builds exercise the real approval and rollback handlers before
// both engines meet the release gate.
const postgresReplayCandidate = true
