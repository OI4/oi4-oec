package oec

/*
#include "oec.h"
*/
import "C"

import "fmt"

// Level is the Messaging compliance level of an application (MG 3).
type Level int32

// The levels.
const (
	Level1 Level = C.OEC_LEVEL_1 // MQTT + TLS + auth, Birth/Will, Pub MAM + Health
	Level2 Level = C.OEC_LEVEL_2 // Level 1 + Get, Set/Del where offered, Status events
	Level3 Level = C.OEC_LEVEL_3 // all resources, full MQTT client behavior
)

func (l Level) String() string { return fmt.Sprintf("level%d", int32(l)) }

// ServiceType of an application (REQ-TOP-005). The zero value AnyServiceType
// is the wildcard of subscriptions and requests to every ServiceType; an
// application needs a concrete one.
type ServiceType int32

// The ServiceTypes. The Go values are those of the C ABI plus one, so the
// zero value is AnyServiceType.
const (
	AnyServiceType ServiceType = iota
	Registry
	OTConnector
	Utility
	Persistence
	Aggregation
	OOCConnector
	ITConnector
)

var serviceTypeNames = [...]string{"+", "Registry", "OTConnector", "Utility", "Persistence",
	"Aggregation", "OOCConnector", "ITConnector"}

func (s ServiceType) String() string {
	if s >= 0 && int(s) < len(serviceTypeNames) {
		return serviceTypeNames[s]
	}
	return fmt.Sprintf("ServiceType(%d)", int32(s))
}

func (s ServiceType) c() C.oec_service_type {
	if s == AnyServiceType {
		return C.OEC_ST_ANY
	}
	return C.oec_service_type(s - 1)
}

// Health state of an asset (MG 8.3.2).
type Health int32

// The Health states.
const (
	Normal              Health = C.OEC_HEALTH_NORMAL
	Failure             Health = C.OEC_HEALTH_FAILURE
	CheckFunction       Health = C.OEC_HEALTH_CHECK_FUNCTION
	OffSpec             Health = C.OEC_HEALTH_OFF_SPEC
	MaintenanceRequired Health = C.OEC_HEALTH_MAINTENANCE_REQUIRED
)

var healthNames = [...]string{"NORMAL_0", "FAILURE_1", "CHECK_FUNCTION_2", "OFF_SPEC_3",
	"MAINTENANCE_REQUIRED_4"}

func (h Health) String() string {
	if h >= 0 && int(h) < len(healthNames) {
		return healthNames[h]
	}
	return fmt.Sprintf("Health(%d)", int32(h))
}

// NoScore leaves out the HealthScore in SetHealth.
const NoScore = -1

// SyslogLevel is the severity of a Syslog event (RFC 5424, REQ-TOP-023).
// The Go values are the RFC 5424 severities plus one, so the zero value
// means "not set" (the SDK default SyslogNotice in Options).
type SyslogLevel int32

// The Syslog levels, from the most to the least severe.
const (
	SyslogEmergency SyslogLevel = iota + 1
	SyslogAlert
	SyslogCritical
	SyslogError
	SyslogWarning
	SyslogNotice
	SyslogInformational
	SyslogDebug
)

var syslogNames = [...]string{"Emergency", "Alert", "Critical", "Error", "Warning", "Notice",
	"Informational", "Debug"}

// Severity is the RFC 5424 severity 0 … 7.
func (l SyslogLevel) Severity() int { return int(l) - 1 }

func (l SyslogLevel) String() string {
	if l >= SyslogEmergency && l <= SyslogDebug {
		return syslogNames[l-1]
	}
	return fmt.Sprintf("SyslogLevel(%d)", int32(l))
}

// ConnectionState of an application.
type ConnectionState int32

// The connection states.
const (
	// Connecting for the first time; retried with backoff until the broker
	// accepts.
	Connecting ConnectionState = C.OEC_STATE_CONNECTING
	// Online: connected, the Birth is sent (REQ-LIFE-001).
	Online ConnectionState = C.OEC_STATE_ONLINE
	// Reconnecting after a lost connection; the Birth is sent again.
	Reconnecting ConnectionState = C.OEC_STATE_RECONNECTING
	// Closed: before the run, after a start error and after the Close
	// (REQ-LIFE-005).
	Closed ConnectionState = C.OEC_STATE_CLOSED
)

var stateNames = [...]string{"Connecting", "Online", "Reconnecting", "Closed"}

func (s ConnectionState) String() string {
	if s >= 0 && int(s) < len(stateNames) {
		return stateNames[s]
	}
	return fmt.Sprintf("ConnectionState(%d)", int32(s))
}

// StatusCode is an OPC UA StatusCode (MG 8.3.6): Good, or Bad with its
// reason.
type StatusCode uint32

// The StatusCodes the SDK names.
const (
	Good                StatusCode = C.OEC_GOOD
	Bad                 StatusCode = C.OEC_BAD
	BadNotFound         StatusCode = C.OEC_BAD_NOT_FOUND
	BadNotWritable      StatusCode = C.OEC_BAD_NOT_WRITABLE
	BadTypeMismatch     StatusCode = C.OEC_BAD_TYPE_MISMATCH
	BadOutOfRange       StatusCode = C.OEC_BAD_OUT_OF_RANGE
	BadInvalidArgument  StatusCode = C.OEC_BAD_INVALID_ARGUMENT
	BadUserAccessDenied StatusCode = C.OEC_BAD_USER_ACCESS_DENIED
	BadInternalError    StatusCode = C.OEC_BAD_INTERNAL_ERROR
)

var statusCodeNames = map[StatusCode]string{
	Good:                "Good",
	Bad:                 "Bad",
	BadNotFound:         "BadNotFound",
	BadNotWritable:      "BadNotWritable",
	BadTypeMismatch:     "BadTypeMismatch",
	BadOutOfRange:       "BadOutOfRange",
	BadInvalidArgument:  "BadInvalidArgument",
	BadUserAccessDenied: "BadUserAccessDenied",
	BadInternalError:    "BadInternalError",
}

// IsBad reports the severity Bad (the two top bits 10).
func (c StatusCode) IsBad() bool { return c>>30 == 2 }

// Error makes a StatusCode usable as the result of an OnSet handler:
// return BadOutOfRange to reject a Set with that code.
func (c StatusCode) Error() string { return "oec: StatusCode " + c.String() }

func (c StatusCode) String() string {
	if n, ok := statusCodeNames[c]; ok {
		return n
	}
	return fmt.Sprintf("0x%08X", uint32(c))
}
