package main

import "testing"

func TestRotateAutoPoolCandidates(t *testing.T) {
	input := []schedulerCandidate{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	rotated := rotateAutoPoolCandidates(input, 2)
	want := []string{"c", "a", "b"}
	for index, candidate := range rotated {
		if candidate.ID != want[index] {
			t.Fatalf("rotated[%d].ID = %q, want %q", index, candidate.ID, want[index])
		}
	}
}
