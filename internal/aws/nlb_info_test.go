package aws

import (
	"strings"
	"testing"
)

func TestListNLBs_InvalidRegion(t *testing.T) {
	nlbs, err := ListNLBs("invalid-region-123!@#")
	if err == nil {
		t.Errorf("Expected an error for an invalid region, but got nil")
	}
	if nlbs != nil {
		t.Errorf("Expected nlbs to be nil for an invalid region, but got %v", nlbs)
	}
	// Further check if the error message indicates something about the region, if possible and stable.
	// For example, AWS SDK might return an error containing "invalid region"
	// This is a basic check, more specific error type/message checking could be added.
}

// TestListNLBs_ValidRegion_NoPanic_HandlesNoNLBsOrAuthError tests that calling ListNLBs with a valid region
// does not panic. It also covers scenarios where no NLBs are found or an authentication/configuration
// error occurs (which is acceptable in a test environment without live credentials).
func TestListNLBs_ValidRegion_NoPanic_HandlesNoNLBsOrAuthError(t *testing.T) {
	// Using a common region. The test doesn't assert on the number of NLBs found,
	// only that the call completes without panic and errors are handled if they occur.
	region := "us-east-1" 
	nlbs, err := ListNLBs(region)

	if err != nil {
		// It's acceptable to get an error here, for instance, if AWS credentials are not configured
		// in the test environment. The key is that the function should handle this gracefully.
		// We can log the error for information.
		t.Logf("ListNLBs returned an error for region '%s': %v. This is acceptable if due to configuration/auth.", region, err)
		
		// Example of checking for a specific type of error if needed, though this can be complex
		// without deeper knowledge of AWS SDK error types. For now, just ensuring it's an error.
		if strings.Contains(err.Error(), "panic") { // A crude check for panics if they were recovered and returned as errors
			t.Errorf("ListNLBs panicked and recovered, then returned error: %v", err)
		}
	}

	// If there was no error, nlbs could be an empty slice (if no NLBs) or contain data.
	// The main thing is that the call was successful and didn't panic.
	if err == nil {
		t.Logf("ListNLBs successfully queried region '%s', found %d NLBs.", region, len(nlbs))
	}

	// The most important assertion is that we reached this point without a panic.
	// The explicit checks above for err==nil or err!=nil handle the outcomes.
	// No specific assertion on len(nlbs) is made due to variability.
}
