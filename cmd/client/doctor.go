package main

import (
	"github.com/kamikazebr/roamie-desktop/internal/client/diagnostics"
)

// Re-export types for backward compatibility
type CheckStatus = diagnostics.CheckStatus
type CheckResult = diagnostics.CheckResult
type DoctorReport = diagnostics.DoctorReport
type DoctorSummary = diagnostics.DoctorSummary
type CheckCategory = diagnostics.CheckCategory

// Re-export constants
const (
	CheckPassed  = diagnostics.CheckPassed
	CheckWarning = diagnostics.CheckWarning
	CheckError   = diagnostics.CheckError
	CheckInfo    = diagnostics.CheckInfo
)

// GetDoctorChecks returns all diagnostic checks organized by category
func GetDoctorChecks() []CheckCategory {
	return diagnostics.GetDoctorChecks()
}

// RunDoctorProgrammatic runs all diagnostic checks and returns structured results
func RunDoctorProgrammatic() DoctorReport {
	return diagnostics.RunDoctorProgrammatic()
}
