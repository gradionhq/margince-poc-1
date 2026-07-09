package records

import (
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/records/adapters"
)

func TestOwnerXorTeamValid(t *testing.T) {
	owner := "00000000-0000-0000-0000-000000000001"
	team := "00000000-0000-0000-0000-000000000002"
	tests := []struct {
		name    string
		ownerID *string
		teamID  *string
		want    bool
	}{
		{name: "both nil", ownerID: nil, teamID: nil, want: false},
		{name: "both set", ownerID: &owner, teamID: &team, want: false},
		{name: "owner only", ownerID: &owner, teamID: nil, want: true},
		{name: "team only", ownerID: nil, teamID: &team, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adapters.OwnerXorTeamValid(tt.ownerID, tt.teamID); got != tt.want {
				t.Errorf("OwnerXorTeamValid(%v, %v) = %v, want %v", tt.ownerID, tt.teamID, got, tt.want)
			}
		})
	}
}
