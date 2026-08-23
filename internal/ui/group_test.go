package ui

import (
	"reflect"
	"testing"

	"github.com/duxet/dcum/internal/compose"
)

func TestGroupImages(t *testing.T) {
	images := []compose.ContainerImage{
		{
			ServiceName:    "web1",
			ImageName:      "nginx",
			CurrentVersion: "1.25.0",
		},
		{
			ServiceName:    "web2",
			ImageName:      "nginx",
			CurrentVersion: "1.25.0",
		},
		{
			ServiceName:    "db1",
			ImageName:      "postgres",
			CurrentVersion: "15",
			Labels: map[string]string{
				"wud.tag.include": "^15\\.",
			},
		},
		{
			ServiceName:    "db2",
			ImageName:      "postgres",
			CurrentVersion: "15",
			Labels: map[string]string{
				"wud.tag.include": "^15\\.",
			},
		},
		{
			ServiceName:    "db3",
			ImageName:      "postgres",
			CurrentVersion: "15",
			// No labels - different key!
		},
	}

	groups := GroupImages(images)

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// Group 0: nginx 1.25.0 (indices 0, 1)
	expectedKey0 := ImageCheckKey{
		ImageName:      "nginx",
		CurrentVersion: "1.25.0",
	}
	if groups[0].Key != expectedKey0 {
		t.Errorf("group 0 key mismatch: expected %+v, got %+v", expectedKey0, groups[0].Key)
	}
	if !reflect.DeepEqual(groups[0].Indices, []int{0, 1}) {
		t.Errorf("group 0 indices mismatch: expected [0 1], got %v", groups[0].Indices)
	}

	// Group 1: postgres 15 with label (indices 2, 3)
	expectedKey1 := ImageCheckKey{
		ImageName:      "postgres",
		CurrentVersion: "15",
		IncludeRegex:   "^15\\.",
	}
	if groups[1].Key != expectedKey1 {
		t.Errorf("group 1 key mismatch: expected %+v, got %+v", expectedKey1, groups[1].Key)
	}
	if !reflect.DeepEqual(groups[1].Indices, []int{2, 3}) {
		t.Errorf("group 1 indices mismatch: expected [2 3], got %v", groups[1].Indices)
	}

	// Group 2: postgres 15 without label (index 4)
	expectedKey2 := ImageCheckKey{
		ImageName:      "postgres",
		CurrentVersion: "15",
	}
	if groups[2].Key != expectedKey2 {
		t.Errorf("group 2 key mismatch: expected %+v, got %+v", expectedKey2, groups[2].Key)
	}
	if !reflect.DeepEqual(groups[2].Indices, []int{4}) {
		t.Errorf("group 2 indices mismatch: expected [4], got %v", groups[2].Indices)
	}
}
