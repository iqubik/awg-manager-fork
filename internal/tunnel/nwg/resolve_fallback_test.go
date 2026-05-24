package nwg

import "testing"

func TestTrackEndpointIP_GetReturnsTracked(t *testing.T) {
	o := &OperatorNativeWG{}
	if got := o.GetTrackedEndpointIP("awg10"); got != "" {
		t.Fatalf("empty operator: GetTrackedEndpointIP = %q, want \"\"", got)
	}
	o.trackEndpointIP("awg10", "1.2.3.4")
	if got := o.GetTrackedEndpointIP("awg10"); got != "1.2.3.4" {
		t.Fatalf("GetTrackedEndpointIP = %q, want 1.2.3.4", got)
	}
	if got := o.GetTrackedEndpointIP("other"); got != "" {
		t.Fatalf("unknown id: GetTrackedEndpointIP = %q, want \"\"", got)
	}
}
