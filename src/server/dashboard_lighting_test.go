package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"LumenForge/src/cluster"
)

func TestDashboardLightingUsesCanonicalClusterSnapshot(t *testing.T) {
	previous := getRGBClusterLightingStatus
	getRGBClusterLightingStatus = func() (cluster.LightingSnapshot, int) {
		return cluster.LightingSnapshot{
			SelectedEffect:      "gradient",
			Brightness:          37,
			EffectiveBrightness: 0,
			Available:           true,
		}, 3
	}
	t.Cleanup(func() { getRGBClusterLightingStatus = previous })

	recorder := httptest.NewRecorder()
	getDashboardLighting(recorder, nil)
	if recorder.Code != 200 {
		t.Fatalf("dashboard lighting status = %d", recorder.Code)
	}
	var response struct {
		Effect         string `json:"effect"`
		Brightness     int    `json:"brightness"`
		ClusterMembers int    `json:"clusterMembers"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Effect != "gradient" || response.Brightness != 37 || response.ClusterMembers != 3 {
		t.Fatalf("dashboard lighting response = %#v", response)
	}
}
